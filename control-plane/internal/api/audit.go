package api

import (
	"net/http"
	"strconv"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// handleListAudit exposes the audit log as a read-only changelog.
// GET /api/v1/audit?actor=&action=&resource_type=&resource_id=&limit=&offset=
// Newest first. X-Total-Count carries the unpaged total.
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	p, ok := parsePage(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := models.AuditFilter{
		Actor:        q.Get("actor"),
		Action:       q.Get("action"),
		ResourceType: q.Get("resource_type"),
		ResourceID:   q.Get("resource_id"),
		Limit:        p.limit,
		Offset:       p.offset,
	}
	logs, total, err := s.store.ListAuditLogs(r.Context(), f)
	if writeStoreError(w, err) {
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, logs)
}
