package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

func deviceDefaults() models.Device {
	return models.Device{Vendor: "yealink", Enabled: true}
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	d := deviceDefaults()
	if err := decodeJSON(r, &d); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if d.MAC == "" || d.Number == "" || d.Domain == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "mac, number and domain are required")
		return
	}
	switch d.Vendor {
	case "yealink", "grandstream", "generic":
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "vendor must be yealink|grandstream|generic")
		return
	}
	if err := s.store.CreateDevice(r.Context(), &d); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_device", d.MAC, nil, d)
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	out, err := s.store.ListDevices(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeList(w, out, pg)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDevice(r.Context(), chi.URLParam(r, "mac"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	d := deviceDefaults()
	if err := decodeJSON(r, &d); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if d.Number == "" || d.Domain == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "number and domain are required")
		return
	}
	if err := s.store.UpdateDevice(r.Context(), mac, &d); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_device", mac, nil, d)
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if err := s.store.DeleteDevice(r.Context(), mac); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_device", mac, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
