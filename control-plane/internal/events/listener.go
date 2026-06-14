package events

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// eslEvents is the event subscription sent to FreeSWITCH after auth.
const eslEvents = "CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE " +
	"CUSTOM callcenter::info conference::maintenance"

// Listener is a persistent ESL inbound client that subscribes to telephony
// events and publishes normalized Events to the Hub. It reconnects on failure.
type Listener struct {
	addr     string
	password string
	hub      *Hub
	log      *slog.Logger
}

func NewListener(addr, password string, hub *Hub, log *slog.Logger) *Listener {
	return &Listener{addr: addr, password: password, hub: hub, log: log}
}

// Run connects and streams until ctx is cancelled, reconnecting with backoff.
func (l *Listener) Run(ctx context.Context) {
	if l.addr == "" {
		l.log.Info("events: ESL not configured, listener disabled")
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		if err := l.session(ctx); err != nil && ctx.Err() == nil {
			l.log.Warn("events: ESL session ended, reconnecting", "err", err, "in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

// session runs one connection: auth, subscribe, then read+publish events.
func (l *Listener) session(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", l.addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-ctx.Done(); conn.Close() }() // unblock the reader on shutdown

	r := bufio.NewReader(conn)
	if _, err := readHeaders(r); err != nil { // auth/request
		return fmt.Errorf("auth request: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "auth %s\n\n", l.password); err != nil {
		return err
	}
	reply, err := readHeaders(r)
	if err != nil {
		return fmt.Errorf("auth reply: %w", err)
	}
	if !strings.Contains(reply["Reply-Text"], "+OK") {
		return fmt.Errorf("auth failed: %s", reply["Reply-Text"])
	}
	if _, err := fmt.Fprintf(conn, "event plain %s\n\n", eslEvents); err != nil {
		return err
	}
	if _, err := readHeaders(r); err != nil { // command reply
		return fmt.Errorf("subscribe reply: %w", err)
	}
	l.log.Info("events: ESL listener connected", "addr", l.addr)

	for {
		frame, err := readHeaders(r)
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(frame["Content-Length"])
		if n == 0 {
			continue
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return err
		}
		if !strings.HasPrefix(frame["Content-Type"], "text/event-plain") {
			continue // log/disconnect/command replies — ignore
		}
		if e, ok := normalize(parseEventBlock(body)); ok {
			l.hub.Publish(e)
		}
	}
}

// readHeaders reads `Name: value` lines until a blank line.
func readHeaders(r *bufio.Reader) (map[string]string, error) {
	h := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return h, nil
		}
		if i := strings.Index(line, ":"); i > 0 {
			h[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
}

// parseEventBlock parses an event-plain block (Name: value lines, values
// URL-encoded) into a map. Stops at the first blank line (the event body, if
// any, is not needed).
func parseEventBlock(b []byte) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if dec, err := url.PathUnescape(val); err == nil {
			val = dec
		}
		m[key] = val
	}
	return m
}

// normalize maps a raw ESL event map to a normalized Event. ok=false means we
// don't care about this event and it should be skipped.
func normalize(m map[string]string) (Event, bool) {
	name := m["Event-Name"]
	uuid := firstNonEmpty(m["Unique-ID"], m["Channel-Call-UUID"])
	d := func(kv ...string) map[string]string {
		out := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			if kv[i+1] != "" {
				out[kv[i]] = kv[i+1]
			}
		}
		return out
	}

	switch name {
	case "CHANNEL_CREATE":
		return Event{Type: "call.started", Data: d(
			"uuid", uuid, "direction", m["Call-Direction"],
			"caller", m["Caller-Caller-ID-Number"], "destination", m["Caller-Destination-Number"],
		)}, true
	case "CHANNEL_ANSWER":
		return Event{Type: "call.answered", Data: d(
			"uuid", uuid, "caller", m["Caller-Caller-ID-Number"],
			"destination", m["Caller-Destination-Number"],
		)}, true
	case "CHANNEL_HANGUP_COMPLETE":
		return Event{Type: "call.ended", Data: d(
			"uuid", uuid, "cause", m["Hangup-Cause"],
			"duration", m["variable_duration"], "billsec", m["variable_billsec"],
		)}, true
	case "CUSTOM":
		switch m["Event-Subclass"] {
		case "callcenter::info":
			action := m["CC-Action"]
			if action == "agent-status-change" {
				return Event{Type: "agent.status", Data: d(
					"agent", m["CC-Agent"], "status", m["CC-Agent-Status"],
				)}, true
			}
			return Event{Type: "queue.member", Data: d(
				"queue", m["CC-Queue"], "action", action,
				"caller", m["CC-Caller-CID-Number"], "agent", m["CC-Agent"],
			)}, true
		case "conference::maintenance":
			return Event{Type: "conference", Data: d(
				"name", m["Conference-Name"], "action", m["Action"],
				"member", m["Member-ID"], "caller", m["Caller-Caller-ID-Number"],
			)}, true
		}
	}
	return Event{}, false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
