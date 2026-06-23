package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func operatorDefaults() models.Operator {
	return models.Operator{Enabled: true}
}

func (s *Server) handleCreateOperator(w http.ResponseWriter, r *http.Request) {
	o := operatorDefaults()
	if err := decodeJSON(r, &o); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if o.Subject == "" || o.Domain == "" || o.Number == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "subject, domain and number are required")
		return
	}
	if err := s.store.CreateOperator(r.Context(), &o); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_operator", o.Subject, nil, o)
	writeJSON(w, http.StatusCreated, o)
}

func (s *Server) handleListOperators(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	out, err := s.store.ListOperators(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeList(w, out, pg)
}

func (s *Server) handleGetOperator(w http.ResponseWriter, r *http.Request) {
	o, err := s.store.GetOperator(r.Context(), chi.URLParam(r, "subject"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleUpdateOperator(w http.ResponseWriter, r *http.Request) {
	subject := chi.URLParam(r, "subject")
	o := operatorDefaults()
	if err := decodeJSON(r, &o); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if o.Domain == "" || o.Number == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "domain and number are required")
		return
	}
	if err := s.store.UpdateOperator(r.Context(), subject, &o); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_operator", subject, nil, o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleDeleteOperator(w http.ResponseWriter, r *http.Request) {
	subject := chi.URLParam(r, "subject")
	if err := s.store.DeleteOperator(r.Context(), subject); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_operator", subject, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
