package api

import (
	"net/http"
	"regexp"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func validateExtension(e *models.DialplanExtension) (string, bool) {
	if e.Name == "" {
		return "name is required", false
	}
	if e.Context == "" {
		return "context is required", false
	}
	if len(e.Conditions) == 0 {
		return "at least one condition is required", false
	}
	for _, c := range e.Conditions {
		if c.Field == "" || c.Expression == "" {
			return "condition field and expression are required", false
		}
		if _, err := regexp.Compile(c.Expression); err != nil {
			return "condition expression is not a valid regex: " + c.Expression, false
		}
		if len(c.Actions) == 0 {
			return "each condition requires at least one action", false
		}
		for _, a := range c.Actions {
			if a.Application == "" {
				return "action application is required", false
			}
		}
	}
	return "", true
}

func (s *Server) handleCreateExtension(w http.ResponseWriter, r *http.Request) {
	var e models.DialplanExtension
	e.Enabled = true
	if err := decodeJSON(r, &e); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if e.Domain == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "domain is required")
		return
	}
	if msg, ok := validateExtension(&e); !ok {
		writeError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}
	if err := s.store.CreateDialplanExtension(r.Context(), &e); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_dialplan_extension", e.ID, nil, e)
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	exts, err := s.store.ListDialplanExtensions(r.Context(), q.Get("domain"), q.Get("context"))
	if writeStoreError(w, err) {
		return
	}
	writeList(w, exts, pg)
}

func (s *Server) handleGetExtension(w http.ResponseWriter, r *http.Request) {
	e, err := s.store.GetDialplanExtension(r.Context(), chi.URLParam(r, "id"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleUpdateExtension(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var e models.DialplanExtension
	e.Enabled = true
	if err := decodeJSON(r, &e); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if msg, ok := validateExtension(&e); !ok {
		writeError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}
	if err := s.store.UpdateDialplanExtension(r.Context(), id, &e); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_dialplan_extension", id, nil, e)
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleDeleteExtension(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteDialplanExtension(r.Context(), id); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_dialplan_extension", id, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
