package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
)

// subscribeReq is the browser PushSubscription plus the operator binding the
// BFF resolved (subject/domain/number).
type subscribeReq struct {
	Subject  string `json:"subject"`
	Domain   string `json:"domain"`
	Number   string `json:"number"`
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	UserAgent string `json:"user_agent"`
}

// handlePushVAPID returns the server VAPID public key for PushManager.subscribe.
func (s *Server) handlePushVAPID(w http.ResponseWriter, r *http.Request) {
	if s.vapidPublic == "" {
		writeError(w, http.StatusServiceUnavailable, "push_disabled", "web push is not configured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": s.vapidPublic})
}

// handlePushSubscribe stores a browser subscription bound to an extension.
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.Domain == "" || req.Number == "" || req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "domain, number, endpoint and keys are required")
		return
	}
	err := s.store.SavePushSub(r.Context(), store.PushSub{
		Subject:   req.Subject,
		Domain:    req.Domain,
		Number:    req.Number,
		Endpoint:  req.Endpoint,
		P256dh:    req.Keys.P256dh,
		Auth:      req.Keys.Auth,
		UserAgent: req.UserAgent,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "could not save subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePushUnsubscribe removes a subscription by endpoint.
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "endpoint is required")
		return
	}
	if err := s.store.DeletePushSub(r.Context(), req.Endpoint); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "could not delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
