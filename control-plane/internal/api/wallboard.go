package api

import (
	_ "embed"
	"net/http"
)

//go:embed wallboard.html
var wallboardHTML []byte

// handleWallboard serves the live supervisor wallboard (a static SPA). The page
// itself carries no secrets; the operator enters the API token in-browser and
// it is used only for the same-origin fetch to /api/v1/events (SSE).
func (s *Server) handleWallboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(wallboardHTML)
}
