// Package events provides an in-process publish/subscribe hub for telephony
// events. A single FreeSWITCH ESL listener (the only publisher) feeds the Hub;
// each SSE connection is a subscriber. No external broker — events are not
// shared across control-plane replicas (single-instance deployment). If the
// service is ever scaled out, swap the Hub for a shared broker (e.g. Redis
// pub/sub) so the one ESL listener reaches subscribers on any replica.
package events

import (
	"sync"
	"time"
)

// Event is a normalized telephony event delivered to subscribers.
type Event struct {
	Type string            `json:"type"`           // e.g. call.started, agent.status
	Time int64             `json:"ts"`             // unix seconds
	Data map[string]string `json:"data,omitempty"` // event-specific fields
}

// subBuffer is how many events a slow subscriber may lag before we drop events
// to it (so one stuck SSE client can never stall the publisher).
const subBuffer = 64

// Hub fans out events to all current subscribers. Safe for concurrent use.
type Hub struct {
	mu   sync.RWMutex
	subs map[int]chan Event
	next int
}

func NewHub() *Hub {
	return &Hub{subs: make(map[int]chan Event)}
}

// Subscribe registers a subscriber and returns its event channel plus a cancel
// func that unsubscribes and closes the channel. Always call cancel (defer).
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subBuffer)
	h.mu.Lock()
	id := h.next
	h.next++
	h.subs[id] = ch
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if c, ok := h.subs[id]; ok {
				delete(h.subs, id)
				close(c)
			}
			h.mu.Unlock()
		})
	}
	return ch, cancel
}

// Publish delivers an event to every subscriber. Non-blocking: if a
// subscriber's buffer is full it is skipped (event dropped for that one),
// keeping the publisher and other subscribers unaffected.
func (h *Hub) Publish(e Event) {
	if e.Time == 0 {
		e.Time = time.Now().Unix()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs {
		select {
		case ch <- e:
		default: // slow subscriber — drop this event for it
		}
	}
}

// Subscribers returns the current subscriber count (for diagnostics/tests).
func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
