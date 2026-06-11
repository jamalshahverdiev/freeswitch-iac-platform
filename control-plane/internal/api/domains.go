package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var d models.Domain
	d.Enabled = true
	if err := decodeJSON(r, &d); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if d.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "domain name is required")
		return
	}
	if err := s.store.CreateDomain(r.Context(), &d); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_domain", d.Name, nil, d)
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.store.ListDomains(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDomain(r.Context(), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var d models.Domain
	d.Enabled = true
	if err := decodeJSON(r, &d); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := s.store.UpdateDomain(r.Context(), name, &d); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_domain", name, nil, d)
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteDomain(r.Context(), name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_domain", name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
