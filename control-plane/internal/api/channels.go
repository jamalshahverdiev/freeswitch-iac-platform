package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

// These values are concatenated into ESL `api` commands, so they are strictly
// validated to prevent command injection.
var (
	uuidRe   = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	destRe   = regexp.MustCompile(`^[0-9A-Za-z*#+._-]{1,64}$`)
	ctxRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	domainRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)
)

func eslReady(s *Server, w http.ResponseWriter) bool {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return false
	}
	return true
}

// handleListChannels lists active call legs. GET /api/v1/runtime/channels
func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	if !eslReady(s, w) {
		return
	}
	chans, err := s.esl.Channels()
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chans)
}

// eslResult turns a raw "+OK"/"-ERR ..." reply into an HTTP response.
func eslResult(w http.ResponseWriter, out string, err error) {
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	if strings.HasPrefix(out, "-ERR") || strings.HasPrefix(out, "-USAGE") {
		writeError(w, http.StatusNotFound, "runtime_error", strings.TrimSpace(out))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": strings.TrimSpace(out)})
}

// handleHangupChannel disconnects a channel. POST /runtime/channels/{uuid}/hangup
func (s *Server) handleHangupChannel(w http.ResponseWriter, r *http.Request) {
	if !eslReady(s, w) {
		return
	}
	uuid := chi.URLParam(r, "uuid")
	if !uuidRe.MatchString(uuid) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid channel uuid")
		return
	}
	out, err := s.esl.Hangup(uuid)
	s.audit.Log(r.Context(), "supervisor", "hangup", "freeswitch_channel", uuid, nil, nil)
	eslResult(w, out, err)
}

// handleParkChannel parks a channel. POST /runtime/channels/{uuid}/park
func (s *Server) handleParkChannel(w http.ResponseWriter, r *http.Request) {
	if !eslReady(s, w) {
		return
	}
	uuid := chi.URLParam(r, "uuid")
	if !uuidRe.MatchString(uuid) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid channel uuid")
		return
	}
	out, err := s.esl.ParkChannel(uuid)
	s.audit.Log(r.Context(), "supervisor", "park", "freeswitch_channel", uuid, nil, nil)
	eslResult(w, out, err)
}

// handleTransferChannel redirects a channel to a destination extension.
// POST /runtime/channels/{uuid}/transfer  {"destination":"4100","context":"company"}
func (s *Server) handleTransferChannel(w http.ResponseWriter, r *http.Request) {
	if !eslReady(s, w) {
		return
	}
	uuid := chi.URLParam(r, "uuid")
	if !uuidRe.MatchString(uuid) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid channel uuid")
		return
	}
	var body struct {
		Destination string `json:"destination"`
		Context     string `json:"context"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !destRe.MatchString(body.Destination) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid destination")
		return
	}
	ctx := body.Context
	if ctx == "" {
		ctx = "company"
	}
	if !ctxRe.MatchString(ctx) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid context")
		return
	}
	out, err := s.esl.TransferChannel(uuid, body.Destination, ctx)
	s.audit.Log(r.Context(), "supervisor", "transfer", "freeswitch_channel", uuid, nil, nil)
	eslResult(w, out, err)
}

// handleEavesdropChannel lets a supervisor covertly listen to a channel by
// dialing their own extension into eavesdrop on the target.
// POST /runtime/channels/{uuid}/eavesdrop  {"extension":"4100","domain":"..."}
func (s *Server) handleEavesdropChannel(w http.ResponseWriter, r *http.Request) {
	if !eslReady(s, w) {
		return
	}
	uuid := chi.URLParam(r, "uuid")
	if !uuidRe.MatchString(uuid) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid channel uuid")
		return
	}
	var body struct {
		Extension string `json:"extension"`
		Domain    string `json:"domain"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !destRe.MatchString(body.Extension) || !domainRe.MatchString(body.Domain) {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid extension or domain")
		return
	}
	out, err := s.esl.Eavesdrop(uuid, body.Extension, body.Domain)
	s.audit.Log(r.Context(), "supervisor", "eavesdrop", "freeswitch_channel", uuid, nil, nil)
	eslResult(w, out, err)
}
