package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleEvents streams live telephony events as Server-Sent Events (SSE).
// GET /api/v1/events  (Bearer token). One `data: <json>\n\n` per event, with a
// periodic heartbeat comment so idle connections and dead peers are detected.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "event stream not enabled")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	// initial comment so the client knows the stream is open
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := w.Write([]byte("event: " + e.Type + "\ndata: ")); err != nil {
				return
			}
			if _, err := w.Write(payload); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
