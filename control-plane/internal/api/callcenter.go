package api

import (
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/go-chi/chi/v5"
)

// ---------- queues ----------

func ccQueueDefaults() models.CCQueue {
	return models.CCQueue{
		Strategy:                          "longest-idle-agent",
		MohSound:                          "local_stream://moh",
		TimeBaseScore:                     "system",
		MaxWaitTimeWithNoAgentTimeReached: 5,
		TierRuleWaitSecond:                300,
		TierRuleWaitMultiplyLevel:         true,
		DiscardAbandonedAfter:             60,
	}
}

func (s *Server) handleCreateCCQueue(w http.ResponseWriter, r *http.Request) {
	q := ccQueueDefaults()
	if err := decodeJSON(r, &q); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if q.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "queue name is required")
		return
	}
	if err := s.store.CreateCCQueue(r.Context(), &q); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_callcenter_queue", q.Name, nil, q)
	writeJSON(w, http.StatusCreated, q)
}

func (s *Server) handleListCCQueues(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListCCQueues(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetCCQueue(w http.ResponseWriter, r *http.Request) {
	q, err := s.store.GetCCQueue(r.Context(), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) handleUpdateCCQueue(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	q := ccQueueDefaults()
	if err := decodeJSON(r, &q); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := s.store.UpdateCCQueue(r.Context(), name, &q); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_callcenter_queue", name, nil, q)
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) handleDeleteCCQueue(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteCCQueue(r.Context(), name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_callcenter_queue", name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- agents ----------

func ccAgentDefaults() models.CCAgent {
	return models.CCAgent{
		Type:              "callback",
		Status:            "Available",
		MaxNoAnswer:       3,
		WrapUpTime:        10,
		RejectDelayTime:   3,
		BusyDelayTime:     60,
		NoAnswerDelayTime: 60,
	}
}

func (s *Server) handleCreateCCAgent(w http.ResponseWriter, r *http.Request) {
	a := ccAgentDefaults()
	if err := decodeJSON(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if a.Name == "" || a.Contact == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "agent name and contact are required")
		return
	}
	if err := s.store.CreateCCAgent(r.Context(), &a); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_callcenter_agent", a.Name, nil, a)
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleListCCAgents(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListCCAgents(r.Context())
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetCCAgent(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetCCAgent(r.Context(), chi.URLParam(r, "name"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleUpdateCCAgent(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	a := ccAgentDefaults()
	if err := decodeJSON(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if a.Contact == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "agent contact is required")
		return
	}
	if err := s.store.UpdateCCAgent(r.Context(), name, &a); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_callcenter_agent", name, nil, a)
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDeleteCCAgent(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteCCAgent(r.Context(), name); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_callcenter_agent", name, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- tiers ----------

func (s *Server) handleCreateCCTier(w http.ResponseWriter, r *http.Request) {
	t := models.CCTier{Level: 1, Position: 1}
	if err := decodeJSON(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if t.Queue == "" || t.Agent == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tier queue and agent are required")
		return
	}
	if err := s.store.CreateCCTier(r.Context(), &t); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "create", "freeswitch_callcenter_tier", t.Queue+"/"+t.Agent, nil, t)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListCCTiers(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListCCTiers(r.Context(), r.URL.Query().Get("queue"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetCCTier(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetCCTier(r.Context(), chi.URLParam(r, "queue"), chi.URLParam(r, "agent"))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateCCTier(w http.ResponseWriter, r *http.Request) {
	queue := chi.URLParam(r, "queue")
	agent := chi.URLParam(r, "agent")
	t := models.CCTier{Level: 1, Position: 1}
	if err := decodeJSON(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := s.store.UpdateCCTier(r.Context(), queue, agent, &t); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "update", "freeswitch_callcenter_tier", queue+"/"+agent, nil, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteCCTier(w http.ResponseWriter, r *http.Request) {
	queue := chi.URLParam(r, "queue")
	agent := chi.URLParam(r, "agent")
	if err := s.store.DeleteCCTier(r.Context(), queue, agent); writeStoreError(w, err) {
		return
	}
	s.audit.Log(r.Context(), "terraform", "delete", "freeswitch_callcenter_tier", queue+"/"+agent, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}
