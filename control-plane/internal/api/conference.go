package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

// ---------- profiles ----------

func confProfileDefaults() models.ConferenceProfile {
	return models.ConferenceProfile{
		Rate:            48000,
		IntervalMs:      20,
		EnergyLevel:     200,
		ComfortNoise:    true,
		MohSound:        "local_stream://moh",
		VideoLayout:     "group:grid",
		VideoCanvasSize: "1280x720",
		VideoFPS:        15,
	}
}

func (s *Server) handleCreateConfProfile(w http.ResponseWriter, r *http.Request) {
	p := confProfileDefaults()
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "profile name is required")
		return
	}
	if err := s.store.CreateConferenceProfile(r.Context(), &p); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_conference_profile", p.Name, nil, p)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleListConfProfiles(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	out, err := s.store.ListConferenceProfiles(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeList(w, out, pg)
}

func (s *Server) handleGetConfProfile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetConferenceProfile(r.Context(), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleUpdateConfProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	p := confProfileDefaults()
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := s.store.UpdateConferenceProfile(r.Context(), name, &p); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_conference_profile", name, nil, p)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteConfProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteConferenceProfile(r.Context(), name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_conference_profile", name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- rooms ----------

func confRoomDefaults() models.ConferenceRoom {
	return models.ConferenceRoom{Priority: 5, Enabled: true}
}

func (s *Server) handleCreateConfRoom(w http.ResponseWriter, r *http.Request) {
	room := confRoomDefaults()
	if err := decodeJSON(r, &room); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if room.Name == "" || room.Number == "" || room.Domain == "" || room.Context == "" || room.Profile == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "room name, number, domain, context and profile are required")
		return
	}
	if err := s.store.CreateConferenceRoom(r.Context(), &room); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_conference_room", room.Name, nil, room)
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) handleListConfRooms(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	out, err := s.store.ListConferenceRooms(r.Context(), r.URL.Query().Get("context"))
	if writeStoreError(w, err) {
		return
	}
	writeList(w, out, pg)
}

func (s *Server) handleGetConfRoom(w http.ResponseWriter, r *http.Request) {
	room, err := s.store.GetConferenceRoom(r.Context(), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (s *Server) handleUpdateConfRoom(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	room := confRoomDefaults()
	if err := decodeJSON(r, &room); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if room.Number == "" || room.Domain == "" || room.Context == "" || room.Profile == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "room number, domain, context and profile are required")
		return
	}
	if err := s.store.UpdateConferenceRoom(r.Context(), name, &room); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_conference_room", name, nil, room)
	writeJSON(w, http.StatusOK, room)
}

func (s *Server) handleDeleteConfRoom(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteConferenceRoom(r.Context(), name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_conference_room", name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
