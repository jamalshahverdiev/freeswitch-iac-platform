package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/audit"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/events"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/runtime"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	store        *store.Store
	audit        *audit.Recorder
	esl          *runtime.Client
	token        string
	xmlUser      string
	xmlPass      string
	xmlAllow     []*net.IPNet
	xmlClientTLS bool
	ccOdbcDSN    string
	vmOdbcDSN    string
	recURL       string
	recUser      string
	recPass      string
	hub          *events.Hub
	vmStore      *store.VoicemailStore
	provUser     string
	provPass     string
	provAllow    []*net.IPNet
	provSIPServer string
	provSIPPort   string
	vapidPublic   string
	log          *slog.Logger
}

type Options struct {
	Token         string
	XMLUser       string
	XMLPassword   string
	XMLAllowCIDRs []string
	// XMLRequireClientCert enforces a verified TLS client cert on /xml/* (mTLS).
	XMLRequireClientCert bool
	// CCOdbcDSN ("dsn:user:pass") is emitted into rendered callcenter.conf.
	CCOdbcDSN string
	// VMOdbcDSN ("dsn:user:pass") is emitted into rendered voicemail.conf.
	VMOdbcDSN string
	// RecURL/RecUser/RecPassword: the recordings file server on the FS host
	// (nginx, autoindex json) that /api/v1/recordings proxies to.
	RecURL      string
	RecUser     string
	RecPassword string
	// Hub streams telephony events to GET /api/v1/events (SSE). May be nil.
	Hub *events.Hub
	// VoicemailStore reads freeswitch_core for the voicemail API. nil → 503.
	VoicemailStore *store.VoicemailStore
	// Phone provisioning (GET /provision/*): Basic auth + CIDR allowlist guard,
	// and the SIP server/port phones register to.
	ProvisionUser       string
	ProvisionPassword   string
	ProvisionAllowCIDRs []string
	ProvisionSIPServer  string
	ProvisionSIPPort    string
	// VAPIDPublicKey is exposed via GET /api/v1/push/vapid so browsers can
	// subscribe to Web Push. Empty → push disabled (endpoint returns 503).
	VAPIDPublicKey string
}

func NewServer(st *store.Store, au *audit.Recorder, esl *runtime.Client, opts Options, log *slog.Logger) *Server {
	s := &Server{
		store:        st,
		audit:        au,
		esl:          esl,
		token:        opts.Token,
		xmlUser:      opts.XMLUser,
		xmlPass:      opts.XMLPassword,
		xmlClientTLS: opts.XMLRequireClientCert,
		ccOdbcDSN:    opts.CCOdbcDSN,
		vmOdbcDSN:    opts.VMOdbcDSN,
		recURL:        opts.RecURL,
		recUser:       opts.RecUser,
		recPass:       opts.RecPassword,
		hub:           opts.Hub,
		vmStore:       opts.VoicemailStore,
		provUser:      opts.ProvisionUser,
		provPass:      opts.ProvisionPassword,
		provSIPServer: opts.ProvisionSIPServer,
		provSIPPort:   opts.ProvisionSIPPort,
		vapidPublic:   opts.VAPIDPublicKey,
		log:           log,
	}
	for _, c := range opts.XMLAllowCIDRs {
		if _, ipnet, err := net.ParseCIDR(c); err == nil {
			s.xmlAllow = append(s.xmlAllow, ipnet)
		} else {
			log.Warn("ignoring invalid XML_ALLOW_CIDRS entry", "cidr", c, "err", err)
		}
	}
	for _, c := range opts.ProvisionAllowCIDRs {
		if _, ipnet, err := net.ParseCIDR(c); err == nil {
			s.provAllow = append(s.provAllow, ipnet)
		} else {
			log.Warn("ignoring invalid PROVISION_ALLOW_CIDRS entry", "cidr", c, "err", err)
		}
	}
	if s.provPass == "" && len(s.provAllow) == 0 {
		log.Warn("/provision/* is UNAUTHENTICATED — set PROVISION_PASSWORD and/or PROVISION_ALLOW_CIDRS (configs contain the SIP password)")
	}
	if s.xmlPass == "" && len(s.xmlAllow) == 0 && !s.xmlClientTLS {
		log.Warn("/xml/* endpoints are UNAUTHENTICATED — set XML_PASSWORD, XML_ALLOW_CIDRS and/or mTLS to protect SIP credentials")
	}
	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestLogger)

	// Public health endpoints.
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	// Supervisor wallboard (static HTML shell; auth happens in-browser for the
	// /api/v1/events fetch). Live data comes from the SSE stream.
	r.Get("/wallboard", s.handleWallboard)

	// Phone provisioning: device-facing config fetch (Basic auth + CIDR).
	r.Group(func(r chi.Router) {
		r.Use(s.provisionGuard)
		r.Get("/provision/{file}", s.handleProvision)
	})

	// FreeSWITCH-facing XML endpoints. Protected by Basic auth and/or an IP
	// allowlist (they expose SIP/trunk secrets), consumed by mod_xml_curl.
	r.Group(func(r chi.Router) {
		r.Use(s.xmlGuard)
		r.Handle("/xml/directory", http.HandlerFunc(s.handleXMLDirectory))
		r.Handle("/xml/dialplan", http.HandlerFunc(s.handleXMLDialplan))
		r.Handle("/xml/configuration", http.HandlerFunc(s.handleXMLConfiguration))
		// CDR ingest is also FreeSWITCH-facing (mod_json_cdr) — same guard.
		r.Post("/cdr", s.handlePostCDR)
	})

	// Management API (token-protected).
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.requireToken)

		r.Post("/domains", s.handleCreateDomain)
		r.Get("/domains", s.handleListDomains)
		r.Get("/domains/{name}", s.handleGetDomain)
		r.Put("/domains/{name}", s.handleUpdateDomain)
		r.Delete("/domains/{name}", s.handleDeleteDomain)

		r.Post("/users", s.handleCreateUser)
		r.Get("/users", s.handleListUsers)
		r.Get("/users/{domain}/{number}", s.handleGetUser)
		r.Put("/users/{domain}/{number}", s.handleUpdateUser)
		r.Delete("/users/{domain}/{number}", s.handleDeleteUser)

		r.Get("/voicemail/{domain}/{number}", s.handleGetVoicemail)
		r.Get("/voicemail/{domain}/{number}/{uuid}/audio", s.handleGetVoicemailAudio)
		r.Post("/voicemail/{domain}/{number}/{uuid}/read", s.handleMarkVoicemailRead)

		r.Post("/operators", s.handleCreateOperator)
		r.Get("/operators", s.handleListOperators)
		r.Get("/operators/{subject}", s.handleGetOperator)
		r.Put("/operators/{subject}", s.handleUpdateOperator)
		r.Delete("/operators/{subject}", s.handleDeleteOperator)

		r.Post("/gateways", s.handleCreateGateway)
		r.Get("/gateways", s.handleListGateways)
		r.Get("/gateways/{profile}/{name}", s.handleGetGateway)
		r.Put("/gateways/{profile}/{name}", s.handleUpdateGateway)
		r.Delete("/gateways/{profile}/{name}", s.handleDeleteGateway)

		r.Post("/dialplan/extensions", s.handleCreateExtension)
		r.Get("/dialplan/extensions", s.handleListExtensions)
		r.Get("/dialplan/extensions/{id}", s.handleGetExtension)
		r.Put("/dialplan/extensions/{id}", s.handleUpdateExtension)
		r.Delete("/dialplan/extensions/{id}", s.handleDeleteExtension)

		r.Post("/callcenter/queues", s.handleCreateCCQueue)
		r.Get("/callcenter/queues", s.handleListCCQueues)
		r.Get("/callcenter/queues/{name}", s.handleGetCCQueue)
		r.Put("/callcenter/queues/{name}", s.handleUpdateCCQueue)
		r.Delete("/callcenter/queues/{name}", s.handleDeleteCCQueue)

		r.Post("/callcenter/agents", s.handleCreateCCAgent)
		r.Get("/callcenter/agents", s.handleListCCAgents)
		r.Get("/callcenter/agents/{name}", s.handleGetCCAgent)
		r.Put("/callcenter/agents/{name}", s.handleUpdateCCAgent)
		r.Delete("/callcenter/agents/{name}", s.handleDeleteCCAgent)

		r.Post("/callcenter/tiers", s.handleCreateCCTier)
		r.Get("/callcenter/tiers", s.handleListCCTiers)
		r.Get("/callcenter/tiers/{queue}/{agent}", s.handleGetCCTier)
		r.Put("/callcenter/tiers/{queue}/{agent}", s.handleUpdateCCTier)
		r.Delete("/callcenter/tiers/{queue}/{agent}", s.handleDeleteCCTier)

		r.Post("/conference/profiles", s.handleCreateConfProfile)
		r.Get("/conference/profiles", s.handleListConfProfiles)
		r.Get("/conference/profiles/{name}", s.handleGetConfProfile)
		r.Put("/conference/profiles/{name}", s.handleUpdateConfProfile)
		r.Delete("/conference/profiles/{name}", s.handleDeleteConfProfile)

		r.Post("/conference/rooms", s.handleCreateConfRoom)
		r.Get("/conference/rooms", s.handleListConfRooms)
		r.Get("/conference/rooms/{name}", s.handleGetConfRoom)
		r.Put("/conference/rooms/{name}", s.handleUpdateConfRoom)
		r.Delete("/conference/rooms/{name}", s.handleDeleteConfRoom)

		r.Get("/audit", s.handleListAudit)

		r.Get("/cdr", s.handleListCDR)
		r.Get("/cdr/stats", s.handleCDRStats)

		r.Get("/events", s.handleEvents)

		r.Get("/push/vapid", s.handlePushVAPID)
		r.Post("/push/subscriptions", s.handlePushSubscribe)
		r.Delete("/push/subscriptions", s.handlePushUnsubscribe)

		r.Post("/devices", s.handleCreateDevice)
		r.Get("/devices", s.handleListDevices)
		r.Get("/devices/{mac}", s.handleGetDevice)
		r.Put("/devices/{mac}", s.handleUpdateDevice)
		r.Delete("/devices/{mac}", s.handleDeleteDevice)

		r.Post("/runtime/reloadxml", s.handleReloadXML)
		r.Get("/runtime/health", s.handleRuntimeHealth)
		r.Get("/runtime/gateways/{profile}/{name}", s.handleRuntimeGatewayStatus)
		r.Get("/runtime/registrations/{domain}/{user}", s.handleRuntimeRegistration)
		r.Post("/runtime/callcenter/reload", s.handleCCReload)
		r.Put("/runtime/callcenter/agents/{name}/status", s.handleCCAgentStatus)
		r.Get("/runtime/callcenter/queues/{name}/{what}", s.handleCCQueueList)
		r.Get("/runtime/channels", s.handleListChannels)
		r.Post("/runtime/channels/{uuid}/hangup", s.handleHangupChannel)
		r.Post("/runtime/channels/{uuid}/park", s.handleParkChannel)
		r.Post("/runtime/channels/{uuid}/transfer", s.handleTransferChannel)
		r.Post("/runtime/channels/{uuid}/eavesdrop", s.handleEavesdropChannel)
		r.Get("/runtime/conference/{name}", s.handleConferenceStatus)
		r.Post("/runtime/conference/{name}/{action}", s.handleConferenceCommand)
		r.Put("/runtime/conference/{name}/layout", s.handleConferenceLayout)

		r.Get("/recordings", s.handleListRecordings)
		r.Get("/recordings/{date}/{file}", s.handleGetRecording)
	})

	return r
}

// --- middleware ---

func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || auth[len(prefix):] != s.token {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing API token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// xmlGuard protects the /xml/* endpoints. If an IP allowlist is configured the
// source must match; if an XML password is configured HTTP Basic auth is
// required. Both are applied when both are set (defense in depth).
func (s *Server) xmlGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.xmlClientTLS && (r.TLS == nil || len(r.TLS.VerifiedChains) == 0) {
			s.log.Warn("xml access denied: missing/invalid client certificate (mTLS)", "remote", r.RemoteAddr, "path", r.URL.Path)
			http.Error(w, "client certificate required", http.StatusUnauthorized)
			return
		}
		if len(s.xmlAllow) > 0 && !s.ipAllowed(r) {
			s.log.Warn("xml access denied by ip allowlist", "remote", r.RemoteAddr, "path", r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if s.xmlPass != "" {
			u, p, ok := r.BasicAuth()
			userOK := subtle.ConstantTimeCompare([]byte(u), []byte(s.xmlUser)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(p), []byte(s.xmlPass)) == 1
			if !ok || !userOK || !passOK {
				w.Header().Set("WWW-Authenticate", `Basic realm="freeswitch-xml"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ipAllowed(r *http.Request) bool { return ipInList(r, s.xmlAllow) }

func ipInList(r *http.Request, list []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range list {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// provisionGuard protects /provision/* (phone-facing, configs hold the SIP
// password): optional CIDR allowlist + optional HTTP Basic auth.
func (s *Server) provisionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.provAllow) > 0 && !ipInList(r, s.provAllow) {
			s.log.Warn("provision denied by ip allowlist", "remote", r.RemoteAddr, "path", r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if s.provPass != "" {
			u, p, ok := r.BasicAuth()
			userOK := subtle.ConstantTimeCompare([]byte(u), []byte(s.provUser)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(p), []byte(s.provPass)) == 1
			if !ok || !userOK || !passOK {
				w.Header().Set("WWW-Authenticate", `Basic realm="provision"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer so SSE (/api/v1/events) keeps working
// through the logging middleware — embedding ResponseWriter does not promote
// the Flusher interface on its own.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// --- response helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	writeJSON(w, status, b)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
