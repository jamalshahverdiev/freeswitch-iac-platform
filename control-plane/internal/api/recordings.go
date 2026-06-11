package api

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

var (
	recDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	recFileRe = regexp.MustCompile(`^[A-Za-z0-9._-]+\.(wav|mp4)$`)
)

// recDatePath turns "2026-06-04" into "2026/06/04".
func recDatePath(date string) string {
	return strings.ReplaceAll(date, "-", "/")
}

func (s *Server) recRequest(r *http.Request, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.recURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.recUser, s.recPass)
	return (&http.Client{Timeout: 60 * time.Second}).Do(req)
}

// handleListRecordings proxies the per-day directory listing from the
// recordings file server on the FreeSWITCH host.
// GET /api/v1/recordings?date=2026-06-04 (default: today)
func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	if s.recURL == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "REC_URL is not configured")
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !recDateRe.MatchString(date) {
		writeError(w, http.StatusBadRequest, "validation_error", "date must be YYYY-MM-DD")
		return
	}
	resp, err := s.recRequest(r, "/"+recDatePath(date)+"/")
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// No recordings that day -> empty list, not an error.
		writeJSON(w, http.StatusOK, map[string]any{"date": date, "recordings": []any{}})
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "runtime_error", "recordings server returned "+resp.Status)
		return
	}
	var entries []struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Mtime string `json:"mtime"`
		Size  int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", "bad listing from recordings server")
		return
	}
	out := []map[string]any{}
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		out = append(out, map[string]any{
			"file":  e.Name,
			"size":  e.Size,
			"mtime": e.Mtime,
			"url":   "/api/v1/recordings/" + date + "/" + e.Name,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "recordings": out})
}

// handleGetRecording streams one recording through the control-plane.
// GET /api/v1/recordings/{date}/{file}
func (s *Server) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	if s.recURL == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "REC_URL is not configured")
		return
	}
	date := chi.URLParam(r, "date")
	file := chi.URLParam(r, "file")
	if !recDateRe.MatchString(date) || !recFileRe.MatchString(file) {
		writeError(w, http.StatusBadRequest, "validation_error", "bad date or file name")
		return
	}
	resp, err := s.recRequest(r, "/"+recDatePath(date)+"/"+file)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "not_found", "recording not found")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "runtime_error", "recordings server returned "+resp.Status)
		return
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "audio/wav"
	}
	w.Header().Set("Content-Type", ct)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
