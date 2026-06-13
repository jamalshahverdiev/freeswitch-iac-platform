package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCreateGateway(w http.ResponseWriter, r *http.Request) {
	var g models.Gateway
	g.Enabled = true
	g.Register = true
	if err := decodeJSON(r, &g); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if g.Name == "" || g.Profile == "" || g.Proxy == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name, profile and proxy are required")
		return
	}
	if err := s.store.CreateGateway(r.Context(), &g); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_gateway", g.Profile+"/"+g.Name, nil, g)
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleListGateways(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	gateways, err := s.store.ListGateways(r.Context(), r.URL.Query().Get("profile"))
	if writeStoreError(w, err) {
		return
	}
	writeList(w, gateways, pg)
}

func (s *Server) handleGetGateway(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGateway(r.Context(), chi.URLParam(r, "profile"), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleUpdateGateway(w http.ResponseWriter, r *http.Request) {
	profile := chi.URLParam(r, "profile")
	name := chi.URLParam(r, "name")
	var g models.Gateway
	g.Enabled = true
	g.Register = true
	if err := decodeJSON(r, &g); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if g.Proxy == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "proxy is required")
		return
	}
	if err := s.store.UpdateGateway(r.Context(), profile, name, &g); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_gateway", profile+"/"+name, nil, g)
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGateway(w http.ResponseWriter, r *http.Request) {
	profile := chi.URLParam(r, "profile")
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteGateway(r.Context(), profile, name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_gateway", profile+"/"+name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
