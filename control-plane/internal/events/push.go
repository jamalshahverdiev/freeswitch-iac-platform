package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushConfig holds the VAPID keypair and contact subject used to sign Web
// Push messages. Both keys must be set for the pusher to be enabled.
type WebPushConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string // VAPID "sub": a mailto: or https: contact URL
}

type pushStore interface {
	PushSubsForExtension(ctx context.Context, domain, number string) ([]store.PushSub, error)
	DeletePushSub(ctx context.Context, endpoint string) error
}

// WebPusher sends VAPID-signed Web Push messages to a user's registered browser
// subscriptions, pruning endpoints the push service reports as gone.
type WebPusher struct {
	cfg   WebPushConfig
	store pushStore
	log   *slog.Logger
}

func NewWebPusher(cfg WebPushConfig, st pushStore, log *slog.Logger) *WebPusher {
	return &WebPusher{cfg: cfg, store: st, log: log}
}

func (w *WebPusher) Enabled() bool {
	return w.cfg.PublicKey != "" && w.cfg.PrivateKey != ""
}

// SendToExtension delivers {title, body} to every browser subscribed for
// domain/number. Stale subscriptions (404/410) are deleted.
func (w *WebPusher) SendToExtension(ctx context.Context, domain, number, title, body string) {
	if !w.Enabled() || domain == "" || number == "" {
		return
	}
	subs, err := w.store.PushSubsForExtension(ctx, domain, number)
	if err != nil {
		w.log.Warn("push: load subscriptions", "err", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	for _, s := range subs {
		sub := &webpush.Subscription{
			Endpoint: s.Endpoint,
			Keys:     webpush.Keys{P256dh: s.P256dh, Auth: s.Auth},
		}
		resp, err := webpush.SendNotificationWithContext(ctx, payload, sub, &webpush.Options{
			Subscriber:      w.cfg.Subject,
			VAPIDPublicKey:  w.cfg.PublicKey,
			VAPIDPrivateKey: w.cfg.PrivateKey,
			TTL:             60,
		})
		if err != nil {
			w.log.Warn("push: send", "endpoint", s.Endpoint, "err", err)
			continue
		}
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			if err := w.store.DeletePushSub(ctx, s.Endpoint); err != nil {
				w.log.Warn("push: prune", "err", err)
			}
		case resp.StatusCode >= 300:
			w.log.Warn("push: non-2xx", "status", resp.StatusCode)
		}
	}
}
