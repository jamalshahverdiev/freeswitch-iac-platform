package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mod_voicemail stores message audio under this tree on the FS host; the
// recordings nginx exposes it read-only at /voicemail/ (see recordings.conf).
const vmStoragePrefix = "/var/lib/freeswitch/storage/voicemail/"

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

// handleGetVoicemailAudio streams one message's .wav via the recordings file
// server (nginx /voicemail/ alias). The uuid must belong to the {domain,number}
// mailbox, so a caller can only fetch their own messages.
// GET /api/v1/voicemail/{domain}/{number}/{uuid}/audio
func (s *Server) handleGetVoicemailAudio(w http.ResponseWriter, r *http.Request) {
	if s.vmStore == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "voicemail read API is not configured (set CORE_DATABASE_URL)")
		return
	}
	if s.recURL == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "REC_URL is not configured")
		return
	}
	domain := chi.URLParam(r, "domain")
	number := chi.URLParam(r, "number")
	uuid := chi.URLParam(r, "uuid")

	fp, err := s.vmStore.MessageFilePath(r.Context(), domain, number, uuid)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	// Guard: must be a real .wav under the voicemail storage tree.
	if fp == "" || !strings.HasPrefix(fp, vmStoragePrefix) || !strings.HasSuffix(fp, ".wav") {
		writeError(w, http.StatusNotFound, "not_found", "voicemail message not found")
		return
	}
	rel := strings.TrimPrefix(fp, vmStoragePrefix) // <folder>/<domain>/<number>/msg_*.wav

	resp, err := s.recRequest(r, "/voicemail/"+rel)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "not_found", "voicemail audio not found")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "runtime_error", "file server returned "+resp.Status)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
