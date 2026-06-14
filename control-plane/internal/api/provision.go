package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
	"github.com/go-chi/chi/v5"
)

// macFromFile extracts the normalized MAC from a provisioning filename. Phones
// fetch vendor-specific names; we accept the common ones:
//   <mac>.cfg        Yealink
//   cfg<mac>.xml     Grandstream
//   <mac>.xml        generic
func macFromFile(file string) string {
	f := strings.ToLower(file)
	f = strings.TrimSuffix(f, ".cfg")
	f = strings.TrimSuffix(f, ".xml")
	f = strings.TrimPrefix(f, "cfg")
	return store.NormalizeMAC(f)
}

// handleProvision renders a device's provisioning config. FreeSWITCH-facing
// (a phone), so it sits behind provisionGuard (Basic auth + CIDR allowlist).
// GET /provision/{file}
func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	mac := macFromFile(chi.URLParam(r, "file"))
	if mac == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	acct, err := s.store.DeviceAccount(r.Context(), mac)
	if err != nil {
		// 404 for any miss (unknown/disabled device, no user/password)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	server := s.provSIPServer
	if server == "" {
		server = acct.Domain
	}
	body, contentType := renderProvision(acct, server, s.provSIPPort)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(body))
}

// renderProvision builds the vendor-specific config. Returns body + content-type.
func renderProvision(a *models.DeviceAccount, server, port string) (string, string) {
	name := a.DisplayName
	if name == "" {
		name = a.Number
	}
	switch a.Vendor {
	case "grandstream":
		// Grandstream P-value XML (classic GXP set, account 1).
		body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<gs_provision version="1">
  <config version="1">
    <P271>1</P271>
    <P47>%s</P47>
    <P40>%s</P40>
    <P35>%s</P35>
    <P36>%s</P36>
    <P34>%s</P34>
    <P3>%s</P3>
  </config>
</gs_provision>
`, xmlEsc(server), xmlEsc(port), xmlEsc(a.Number), xmlEsc(a.Number), xmlEsc(a.Password), xmlEsc(name))
		return body, "text/xml; charset=utf-8"
	default:
		// Yealink-style .cfg (account 1). Works for Yealink; "generic" reuses it.
		body := fmt.Sprintf(`#!version:1.0.0.1
account.1.enable = 1
account.1.label = %s
account.1.display_name = %s
account.1.auth_name = %s
account.1.user_name = %s
account.1.password = %s
account.1.sip_server.1.address = %s
account.1.sip_server.1.port = %s
account.1.sip_server.1.transport_type = 0
`, name, name, a.Number, a.Number, a.Password, server, port)
		return body, "text/plain; charset=utf-8"
	}
}

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
