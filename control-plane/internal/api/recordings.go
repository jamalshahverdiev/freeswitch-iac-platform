package api

import (
	"encoding/json"
	"fmt"
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
	recNumRe  = regexp.MustCompile(`^[0-9]{1,15}$`)
)

// recParties parses "<caller>_<dest>_<uuid>.wav" into caller/dest. Returns empty
// strings for recordings not produced by the call-recording dialplan (queue /
// conference recordings use a different naming scheme).
func recParties(file string) (caller, dest string) {
	base := strings.TrimSuffix(strings.TrimSuffix(file, ".wav"), ".mp4")
	parts := strings.SplitN(base, "_", 3)
	if len(parts) < 3 {
		return "", ""
	}
	return parts[0], parts[1]
}

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

var recYearRe = regexp.MustCompile(`^\d{4}$`)
var recMonDayRe = regexp.MustCompile(`^\d{2}$`)

type recEntry struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Mtime string `json:"mtime"`
	Size  int64  `json:"size"`
}

// recList fetches one autoindex-json directory listing from the recordings file
// server. A missing directory (404) yields an empty slice, not an error.
func (s *Server) recList(r *http.Request, path string) ([]recEntry, error) {
	resp, err := s.recRequest(r, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("recordings server returned %s", resp.Status)
	}
	var entries []recEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("bad listing from recordings server")
	}
	return entries, nil
}

// handleListRecordings lists recordings from the year/month/day tree on the
// recordings file server, either for a single day or an inclusive date range.
// It walks only directories that exist (years → months → days), so a wide range
// spanning months/years is cheap.
//   GET /api/v1/recordings?date=2026-06-04                 (single day; default today)
//   GET /api/v1/recordings?from=2025-12-01&to=2026-07-01   (inclusive range)
//   &number=4201  → keep only recordings where the extension is a party
func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	if s.recURL == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_error", "REC_URL is not configured")
		return
	}
	q := r.URL.Query()

	number := q.Get("number")
	if number != "" && !recNumRe.MatchString(number) {
		writeError(w, http.StatusBadRequest, "validation_error", "number must be digits")
		return
	}

	from, to := q.Get("from"), q.Get("to")
	if from == "" && to == "" {
		d := q.Get("date")
		if d == "" {
			d = time.Now().Format("2006-01-02")
		}
		from, to = d, d
	}
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	if !recDateRe.MatchString(from) || !recDateRe.MatchString(to) {
		writeError(w, http.StatusBadRequest, "validation_error", "dates must be YYYY-MM-DD")
		return
	}
	start, _ := time.Parse("2006-01-02", from)
	end, _ := time.Parse("2006-01-02", to)
	if end.Before(start) {
		start, end = end, start
	}

	out, err := s.walkRecordings(r, start, end, number)
	if err != nil {
		writeError(w, http.StatusBadGateway, "runtime_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"), "recordings": out,
	})
}

// walkRecordings descends year → month → day within [start, end] (inclusive,
// day granularity), touching only existing directories, and returns matching
// files newest day first.
func (s *Server) walkRecordings(r *http.Request, start, end time.Time, number string) ([]map[string]any, error) {
	inRange := func(t time.Time) bool { return !t.Before(start) && !t.After(end) }
	out := []map[string]any{}

	years, err := s.recList(r, "/")
	if err != nil {
		return nil, err
	}
	yearNames := descendingDirs(years, recYearRe, start.Year(), end.Year())
	for _, y := range yearNames {
		months, err := s.recList(r, "/"+y+"/")
		if err != nil {
			return nil, err
		}
		for _, m := range descendingDirs(months, recMonDayRe, 1, 12) {
			days, err := s.recList(r, "/"+y+"/"+m+"/")
			if err != nil {
				return nil, err
			}
			for _, d := range descendingDirs(days, recMonDayRe, 1, 31) {
				date := y + "-" + m + "-" + d
				dt, perr := time.Parse("2006-01-02", date)
				if perr != nil || !inRange(dt) {
					continue
				}
				files, err := s.recList(r, "/"+y+"/"+m+"/"+d+"/")
				if err != nil {
					return nil, err
				}
				for _, e := range files {
					if e.Type != "file" {
						continue
					}
					caller, dest := recParties(e.Name)
					if number != "" && caller != number && dest != number {
						continue
					}
					out = append(out, map[string]any{
						"file":   e.Name,
						"date":   date,
						"caller": caller,
						"dest":   dest,
						"size":   e.Size,
						"mtime":  e.Mtime,
						"url":    "/api/v1/recordings/" + date + "/" + e.Name,
					})
				}
			}
		}
	}
	return out, nil
}

// descendingDirs returns matching directory names (as ints in [lo, hi]) sorted
// descending, so callers emit newest-first without a full pre-parse.
func descendingDirs(entries []recEntry, re *regexp.Regexp, lo, hi int) []string {
	var names []string
	for _, e := range entries {
		if e.Type != "directory" || !re.MatchString(e.Name) {
			continue
		}
		var n int
		fmt.Sscanf(e.Name, "%d", &n)
		if n < lo || n > hi {
			continue
		}
		names = append(names, e.Name)
	}
	// Names are zero-padded fixed-width, so lexical sort == numeric sort.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] > names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
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
