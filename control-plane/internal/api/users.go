package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var u models.User
	u.Enabled = true
	if err := decodeJSON(r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if u.Domain == "" || u.Number == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "domain and number are required")
		return
	}
	if err := s.store.CreateUser(r.Context(), &u); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_user", u.Domain+"/"+u.Number, nil, u)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	users, err := s.store.ListUsers(r.Context(), r.URL.Query().Get("domain"))
	if writeStoreError(w, err) {
		return
	}
	writeList(w, users, pg)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.Context(), chi.URLParam(r, "domain"), chi.URLParam(r, "number"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	number := chi.URLParam(r, "number")
	var u models.User
	u.Enabled = true
	if err := decodeJSON(r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := s.store.UpdateUser(r.Context(), domain, number, &u); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_user", domain+"/"+number, nil, u)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	number := chi.URLParam(r, "number")
	if err := s.store.DeleteUser(r.Context(), domain, number); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_user", domain+"/"+number, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
