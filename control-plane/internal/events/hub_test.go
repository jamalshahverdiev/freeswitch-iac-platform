package events

import (
	"sync"
	"testing"
	"time"
)

func TestHubFanout(t *testing.T) {
	h := NewHub()
	ch1, c1 := h.Subscribe()
	ch2, c2 := h.Subscribe()
	defer c1()
	defer c2()

	if h.Subscribers() != 2 {
		t.Fatalf("subscribers = %d, want 2", h.Subscribers())
	}

	h.Publish(Event{Type: "call.started", Data: map[string]string{"uuid": "abc"}})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Type != "call.started" || e.Data["uuid"] != "abc" {
				t.Errorf("sub %d got %+v", i, e)
			}
			if e.Time == 0 {
				t.Errorf("sub %d: Time not stamped", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub %d: no event", i)
		}
	}
}

func TestHubCancelUnsubscribes(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()
	cancel()
	if h.Subscribers() != 0 {
		t.Fatalf("subscribers = %d after cancel, want 0", h.Subscribers())
	}
	// channel is closed
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after cancel")
	}
	cancel() // idempotent, must not panic
	// publishing with no subscribers must not panic
	h.Publish(Event{Type: "noop"})
}

func TestHubSlowConsumerDropped(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()
	defer cancel()

	// Never drain ch: fill its buffer, then publish more. Publish must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < subBuffer+100; i++ {
			h.Publish(Event{Type: "flood"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow consumer")
	}
	// buffer holds at most subBuffer; the rest were dropped — no deadlock.
	if got := len(ch); got > subBuffer {
		t.Fatalf("buffered %d, want <= %d", got, subBuffer)
	}
}

func TestHubConcurrent(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	// concurrent subscribers churning + a publisher — race detector catches bugs
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := h.Subscribe()
			go h.Publish(Event{Type: "x"})
			select {
			case <-ch:
			case <-time.After(500 * time.Millisecond):
			}
			cancel()
		}()
	}
	wg.Wait()
	if h.Subscribers() != 0 {
		t.Fatalf("leaked subscribers: %d", h.Subscribers())
	}
}
