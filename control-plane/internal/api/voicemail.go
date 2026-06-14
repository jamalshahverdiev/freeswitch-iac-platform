package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleGetVoicemail returns a user's mailbox (messages + MWI counters) read
// from freeswitch_core. Read-only; 503 when CORE_DATABASE_URL is not set.
// GET /api/v1/voicemail/{domain}/{number}
func (s *Server) handleGetVoicemail(w http.ResponseWriter, r *http.Request) {
	if s.vmStore == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "voicemail read API is not configured (set CORE_DATABASE_URL)")
		return
	}
	domain := chi.URLParam(r, "domain")
	number := chi.URLParam(r, "number")
	box, err := s.vmStore.Messages(r.Context(), domain, number)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, box)
}
