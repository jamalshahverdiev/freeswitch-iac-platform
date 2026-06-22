package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NotifyConfig configures the voicemail MWI notifier sinks. A sink is active
// when its fields are set; if none are set the notifier is disabled.
type NotifyConfig struct {
	WebhookURL    string // generic: POST a JSON payload here
	WebhookHeader string // optional extra header on the webhook, "Key: Value"
	TelegramToken string // Telegram bot token
	TelegramChat  string // Telegram chat id
}

// Notifier subscribes to the Hub and pushes a notification when a user's NEW
// voicemail count increases (a fresh message arrived). It dedupes per account so
// MWI refreshes (e.g. phone re-subscribes) don't re-notify.
type Notifier struct {
	hub  *Hub
	cfg  NotifyConfig
	log  *slog.Logger
	http *http.Client

	mu      sync.Mutex
	lastNew map[string]int // account -> last observed new count
}

func NewNotifier(hub *Hub, cfg NotifyConfig, log *slog.Logger) *Notifier {
	return &Notifier{
		hub:     hub,
		cfg:     cfg,
		log:     log,
		http:    &http.Client{Timeout: 10 * time.Second},
		lastNew: map[string]int{},
	}
}

// Enabled reports whether at least one sink is configured.
func (n *Notifier) Enabled() bool {
	return n.cfg.WebhookURL != "" || (n.cfg.TelegramToken != "" && n.cfg.TelegramChat != "")
}

// Run subscribes to the hub and notifies on new voicemail until ctx is done.
func (n *Notifier) Run(ctx context.Context) {
	if !n.Enabled() {
		return
	}
	ch, cancel := n.hub.Subscribe()
	defer cancel()
	n.log.Info("voicemail notifier enabled",
		"webhook", n.cfg.WebhookURL != "", "telegram", n.cfg.TelegramToken != "")
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if e.Type == "voicemail.mwi" {
				n.handle(ctx, e)
			}
		}
	}
}

// handle fires notifications only when the account's new count increased.
func (n *Notifier) handle(ctx context.Context, e Event) {
	acct := e.Data["account"]
	if acct == "" {
		return
	}
	newC, _ := strconv.Atoi(e.Data["new"])

	n.mu.Lock()
	prev := n.lastNew[acct]
	n.lastNew[acct] = newC
	n.mu.Unlock()

	if newC <= prev { // read/refresh/no-change — not a new message
		return
	}

	msg := fmt.Sprintf("New voicemail for %s — %d new message(s)", acct, newC)
	if n.cfg.WebhookURL != "" {
		go n.sendWebhook(ctx, e, msg)
	}
	if n.cfg.TelegramToken != "" && n.cfg.TelegramChat != "" {
		go n.sendTelegram(ctx, msg)
	}
}

func (n *Notifier) sendWebhook(ctx context.Context, e Event, msg string) {
	body, _ := json.Marshal(map[string]any{
		"event":   "voicemail.new",
		"account": e.Data["account"],
		"user":    e.Data["user"],
		"domain":  e.Data["domain"],
		"new":     e.Data["new"],
		"saved":   e.Data["saved"],
		"ts":      e.Time,
		"message": msg,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		n.log.Warn("voicemail webhook build failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h := n.cfg.WebhookHeader; h != "" {
		if k, v, ok := strings.Cut(h, ":"); ok {
			req.Header.Set(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
	resp, err := n.http.Do(req)
	if err != nil {
		n.log.Warn("voicemail webhook failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		n.log.Warn("voicemail webhook non-2xx", "status", resp.StatusCode)
	}
}

func (n *Notifier) sendTelegram(ctx context.Context, msg string) {
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramToken)
	form := url.Values{"chat_id": {n.cfg.TelegramChat}, "text": {msg}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, strings.NewReader(form.Encode()))
	if err != nil {
		n.log.Warn("telegram build failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := n.http.Do(req)
	if err != nil {
		n.log.Warn("telegram notify failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		n.log.Warn("telegram notify non-2xx", "status", resp.StatusCode)
	}
}
