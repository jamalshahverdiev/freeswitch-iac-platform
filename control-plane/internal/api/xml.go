package api

import (
	"errors"
	"net/http"
	"sort"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/renderer"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
)

func (s *Server) writeXML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) xmlError(w http.ResponseWriter) {
	// On internal error, return the FreeSWITCH "not found" document so it
	// falls back to on-disk config instead of breaking.
	s.writeXML(w, []byte(renderer.NotFoundDocument))
}

func (s *Server) handleXMLDirectory(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	// Gateway/profile lookups are not served from desired state.
	if r.Form.Get("purpose") == "gateways" {
		s.writeXML(w, []byte(renderer.NotFoundDocument))
		return
	}

	user := r.Form.Get("user")
	domain := r.Form.Get("domain")
	if domain == "" {
		domain = r.Form.Get("key_value")
	}

	// Specific user lookup (registration / invite auth). Return only that user
	// if we manage it, otherwise "not found" so FreeSWITCH falls back to its
	// static directory (e.g. the default 1000-1019 users).
	if user != "" && domain != "" {
		u, err := s.store.GetUser(r.Context(), domain, user)
		if errors.Is(err, store.ErrNotFound) {
			s.writeXML(w, []byte(renderer.NotFoundDocument))
			return
		}
		if err != nil {
			s.log.Error("directory user lookup", "err", err)
			s.xmlError(w)
			return
		}
		d, err := s.store.GetDomain(r.Context(), domain)
		if err != nil {
			s.xmlError(w)
			return
		}
		body, err := renderer.RenderDirectory([]models.DomainWithUsers{{Domain: *d, Users: []models.User{*u}}})
		if err != nil {
			s.xmlError(w)
			return
		}
		s.writeXML(w, body)
		return
	}

	// No specific user: return the full managed directory (manual curl / browse).
	data, err := s.store.DirectoryData(r.Context())
	if err != nil {
		s.log.Error("directory render", "err", err)
		s.xmlError(w)
		return
	}
	body, err := renderer.RenderDirectory(data)
	if err != nil {
		s.xmlError(w)
		return
	}
	s.writeXML(w, body)
}

func (s *Server) handleXMLDialplan(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	context := r.Form.Get("Hunt-Context")
	if context == "" {
		context = r.Form.Get("context")
	}
	exts, err := s.store.ListDialplanExtensions(r.Context(), "", context)
	if err != nil {
		s.log.Error("dialplan render", "err", err)
		s.xmlError(w)
		return
	}
	// Conference rooms materialize their own entry extension.
	rooms, err := s.store.ListConferenceRooms(r.Context(), context)
	if err != nil {
		s.log.Error("dialplan rooms", "err", err)
		s.xmlError(w)
		return
	}
	for _, room := range rooms {
		exts = append(exts, renderer.ConferenceRoomExtension(room))
	}
	sort.SliceStable(exts, func(i, j int) bool {
		if exts[i].Context != exts[j].Context {
			return exts[i].Context < exts[j].Context
		}
		if exts[i].Priority != exts[j].Priority {
			return exts[i].Priority < exts[j].Priority
		}
		return exts[i].Name < exts[j].Name
	})
	// If FreeSWITCH asked for a context we don't manage, fall back to its files.
	if context != "" && len(exts) == 0 {
		s.writeXML(w, []byte(renderer.NotFoundDocument))
		return
	}
	body, err := renderer.RenderDialplan(exts, context)
	if err != nil {
		s.xmlError(w)
		return
	}
	s.writeXML(w, body)
}

func (s *Server) handleXMLConfiguration(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	key := r.Form.Get("key_value")
	switch key {
	case "callcenter.conf":
		s.serveCallcenterConf(w, r)
	case "conference.conf":
		s.serveConferenceConf(w, r)
	case "voicemail.conf":
		s.serveVoicemailConf(w, r)
	default:
		// Everything else (including sofia.conf — gateways stay on disk until
		// the gateways milestone) falls back to FreeSWITCH's files.
		s.writeXML(w, []byte(renderer.NotFoundDocument))
	}
}

func (s *Server) serveVoicemailConf(w http.ResponseWriter, r *http.Request) {
	body, err := renderer.RenderVoicemail(s.vmOdbcDSN)
	if err != nil {
		s.log.Error("voicemail render", "err", err)
		s.xmlError(w)
		return
	}
	s.writeXML(w, body)
}

func (s *Server) serveConferenceConf(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.store.ListConferenceProfiles(r.Context())
	if err != nil {
		s.log.Error("conference render", "err", err)
		s.xmlError(w)
		return
	}
	// No managed profiles -> let FreeSWITCH use its on-disk conference.conf.
	if len(profiles) == 0 {
		s.writeXML(w, []byte(renderer.NotFoundDocument))
		return
	}
	body, err := renderer.RenderConference(profiles)
	if err != nil {
		s.xmlError(w)
		return
	}
	s.writeXML(w, body)
}

func (s *Server) serveCallcenterConf(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queues, err := s.store.ListCCQueues(ctx)
	if err != nil {
		s.log.Error("callcenter render", "err", err)
		s.xmlError(w)
		return
	}
	agents, err := s.store.ListCCAgents(ctx)
	if err != nil {
		s.xmlError(w)
		return
	}
	tiers, err := s.store.ListCCTiers(ctx, "")
	if err != nil {
		s.xmlError(w)
		return
	}
	body, err := renderer.RenderCallcenter(queues, agents, tiers, s.ccOdbcDSN)
	if err != nil {
		s.xmlError(w)
		return
	}
	s.writeXML(w, body)
}
