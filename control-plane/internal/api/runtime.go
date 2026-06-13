package api

import (
	"encoding/xml"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleReloadXML(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	out, err := s.esl.ReloadXML()
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	s.audit.Log(r.Context(), "terraform", "reloadxml", "freeswitch_runtime", "reloadxml", nil, nil)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"command": "reloadxml",
		"message": out,
	})
}

func (s *Server) handleRuntimeGatewayStatus(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	profile := chi.URLParam(r, "profile")
	name := chi.URLParam(r, "name")
	attrs, err := s.esl.GatewayStatus(name)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	if attrs == nil {
		writeError(w, http.StatusNotFound, "not_found", "gateway not found in runtime")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       name,
		"profile":    profile,
		"status":     attrs["Status"],
		"state":      attrs["State"],
		"attributes": attrs,
	})
}

func (s *Server) handleRuntimeRegistration(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	domain := chi.URLParam(r, "domain")
	user := chi.URLParam(r, "user")
	row, err := s.esl.Registration(user, domain)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	if row == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"user": user, "domain": domain, "registered": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"domain":       domain,
		"registered":   true,
		"contact":      row["contact"],
		"agent":        row["agent"],
		"network_ip":   row["network_ip"],
		"network_port": row["network_port"],
		"expires":      row["expires"],
	})
}

// handleCCReload re-reads the (xml_curl-served) callcenter.conf by reloading
// mod_callcenter — the "apply" step after queue/agent/tier changes.
func (s *Server) handleCCReload(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	out, err := s.esl.API("reload mod_callcenter")
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	s.audit.Log(r.Context(), "terraform", "reload", "freeswitch_callcenter", "mod_callcenter", nil, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "command": "reload mod_callcenter", "message": out})
}

// handleCCAgentStatus sets an agent's live status (Available / Logged Out / ...).
func (s *Server) handleCCAgentStatus(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	name := chi.URLParam(r, "name")
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Status == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "body must be {\"status\": \"...\"}")
		return
	}
	out, err := s.esl.API("callcenter_config agent set status " + name + " '" + body.Status + "'")
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"agent": name, "status": body.Status, "message": out})
}

// handleCCQueueList proxies `callcenter_config queue list agents|members|tiers <q>`.
func (s *Server) handleCCQueueList(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	name := chi.URLParam(r, "name")
	what := chi.URLParam(r, "what")
	switch what {
	case "agents", "members", "tiers":
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "what must be agents|members|tiers")
		return
	}
	out, err := s.esl.API("callcenter_config queue list " + what + " " + name)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"queue": name, "list": what, "raw": out})
}

// ---------- conference runtime ----------

type confMemberXML struct {
	ID           string `xml:"id"`
	CallerIDName string `xml:"caller_id_name"`
	CallerIDNum  string `xml:"caller_id_number"`
	JoinTime     int    `xml:"join_time"`
	Flags        struct {
		CanHear   bool `xml:"can_hear"`
		CanSee    bool `xml:"can_see"`
		CanSpeak  bool `xml:"can_speak"`
		HasVideo  bool `xml:"has_video"`
		IsTalking bool `xml:"talking"`
	} `xml:"flags"`
}

type confListXML struct {
	XMLName     xml.Name `xml:"conferences"`
	Conferences []struct {
		Name        string          `xml:"name,attr"`
		MemberCount int             `xml:"member-count,attr"`
		Rate        int             `xml:"rate,attr"`
		RunTime     int             `xml:"run_time,attr"`
		Members     []confMemberXML `xml:"members>member"`
	} `xml:"conference"`
}

// handleConferenceStatus parses `conference <room> xml_list` into JSON:
// the live participants of one room (404 when the room is not running).
func (s *Server) handleConferenceStatus(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	name := chi.URLParam(r, "name")
	out, err := s.esl.API("conference " + name + " xml_list")
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	var parsed confListXML
	if err := xml.Unmarshal([]byte(out), &parsed); err != nil || len(parsed.Conferences) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "conference is not running")
		return
	}
	c := parsed.Conferences[0]
	members := []map[string]any{}
	for _, m := range c.Members {
		members = append(members, map[string]any{
			"id":               m.ID,
			"caller_id_name":   m.CallerIDName,
			"caller_id_number": m.CallerIDNum,
			"join_time":        m.JoinTime,
			"can_hear":         m.Flags.CanHear,
			"can_see":          m.Flags.CanSee,
			"can_speak":        m.Flags.CanSpeak,
			"has_video":        m.Flags.HasVideo,
			"talking":          m.Flags.IsTalking,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":         c.Name,
		"member_count": c.MemberCount,
		"rate":         c.Rate,
		"run_time":     c.RunTime,
		"members":      members,
	})
}

// handleConferenceCommand proxies kick/mute/unmute on a member ("all" allowed).
func (s *Server) handleConferenceCommand(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	name := chi.URLParam(r, "name")
	action := chi.URLParam(r, "action")
	switch action {
	case "kick", "mute", "unmute":
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "action must be kick|mute|unmute")
		return
	}
	var body struct {
		Member string `json:"member"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Member == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "body must be {\"member\": \"<id>|all|last\"}")
		return
	}
	out, err := s.esl.API("conference " + name + " " + action + " " + body.Member)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	s.audit.Log(r.Context(), "terraform", action, "freeswitch_conference", name+"/"+body.Member, nil, nil)
	writeJSON(w, http.StatusOK, map[string]string{"conference": name, "action": action, "member": body.Member, "message": out})
}

// handleConferenceLayout switches the live video layout of a running room.
func (s *Server) handleConferenceLayout(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "ESL is not configured")
		return
	}
	name := chi.URLParam(r, "name")
	var body struct {
		Layout string `json:"layout"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Layout == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "body must be {\"layout\": \"...\"}")
		return
	}
	out, err := s.esl.API("conference " + name + " vid-layout " + body.Layout)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"conference": name, "layout": body.Layout, "message": out})
}

func (s *Server) handleRuntimeHealth(w http.ResponseWriter, r *http.Request) {
	if !s.esl.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"esl": "disabled"})
		return
	}
	if err := s.esl.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"esl":   "error",
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"esl": "ok"})
}
