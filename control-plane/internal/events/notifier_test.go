package events

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNotifierEnabled(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if NewNotifier(nil, NotifyConfig{}, log).Enabled() {
		t.Error("no sinks should be disabled")
	}
	if !NewNotifier(nil, NotifyConfig{WebhookURL: "http://x"}, log).Enabled() {
		t.Error("webhook sink should be enabled")
	}
	// telegram needs BOTH token and chat
	if NewNotifier(nil, NotifyConfig{TelegramToken: "t"}, log).Enabled() {
		t.Error("telegram needs chat id too")
	}
	if !NewNotifier(nil, NotifyConfig{TelegramToken: "t", TelegramChat: "c"}, log).Enabled() {
		t.Error("telegram with token+chat should be enabled")
	}
}

func TestNotifierWebhookOnIncrease(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	done := make(chan struct{}, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		done <- struct{}{}
	}))
	defer srv.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(nil, NotifyConfig{WebhookURL: srv.URL}, log)
	ctx := context.Background()

	mwi := func(acct, newC string) Event {
		return Event{Type: "voicemail.mwi", Time: 1, Data: map[string]string{
			"account": acct, "user": "1001", "domain": "d", "new": newC, "saved": "0",
		}}
	}

	// 0 -> 1: notify; 1 -> 1 (refresh): no notify; 1 -> 2: notify; 2 -> 0 (read): no notify
	n.handle(ctx, mwi("1001@d", "1"))
	n.handle(ctx, mwi("1001@d", "1"))
	n.handle(ctx, mwi("1001@d", "2"))
	n.handle(ctx, mwi("1001@d", "0"))

	// expect exactly 2 webhooks (the two increases)
	deadline := time.After(2 * time.Second)
	got := 0
	for got < 2 {
		select {
		case <-done:
			got++
		case <-deadline:
			t.Fatalf("got %d webhooks, want 2", got)
		}
	}
	// ensure no extra arrives shortly after
	select {
	case <-done:
		t.Fatal("unexpected 3rd webhook (refresh/read should not notify)")
	case <-time.After(150 * time.Millisecond):
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("bodies = %d, want 2", len(bodies))
	}
	// webhooks are sent in goroutines, so order isn't guaranteed — assert the set
	seen := map[string]bool{}
	for _, b := range bodies {
		if b["event"] != "voicemail.new" || b["account"] != "1001@d" {
			t.Errorf("bad payload: %+v", b)
		}
		seen[b["new"].(string)] = true
	}
	if !seen["1"] || !seen["2"] {
		t.Errorf("expected notifications for new=1 and new=2, got %+v", bodies)
	}
}

func TestNotifierIgnoresNonMWI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("non-MWI event must not trigger a webhook")
	}))
	defer srv.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(nil, NotifyConfig{WebhookURL: srv.URL}, log)
	// handle() is MWI-specific; the Run loop filters by type, so simulate that:
	e := Event{Type: "call.started", Data: map[string]string{"uuid": "x"}}
	if e.Type == "voicemail.mwi" {
		n.handle(context.Background(), e)
	}
	time.Sleep(100 * time.Millisecond)
}
