package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// jsonCDRPayload is the slice of mod_json_cdr we care about. `variables` carries
// uuid/times/cause; caller & destination live in callflow[].caller_profile.
type jsonCDRPayload struct {
	Variables map[string]string `json:"variables"`
	Callflow  []struct {
		CallerProfile struct {
			CallerIDName      string `json:"caller_id_name"`
			CallerIDNumber    string `json:"caller_id_number"`
			DestinationNumber string `json:"destination_number"`
			Context           string `json:"context"`
		} `json:"caller_profile"`
	} `json:"callflow"`
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func atoi64(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }
func atoi(s string) int     { n, _ := strconv.Atoi(s); return n }

// handlePostCDR ingests a CDR posted by FreeSWITCH mod_json_cdr. It is
// FreeSWITCH-facing, so it sits behind the same guard as /xml/* (mTLS + Basic).
// mod_json_cdr posts the JSON either as the raw body or as a form field `cdr`.
func (s *Server) handlePostCDR(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20)) // 4 MiB cap
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "cannot read body")
		return
	}
	raw := body
	// form-encoded variant: cdr=<json>. ParseForm reads r.Body, so restore it
	// from the bytes we already buffered.
	if ct := r.Header.Get("Content-Type"); strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		if vals, e := url.ParseQuery(string(body)); e == nil && vals.Get("cdr") != "" {
			raw = []byte(vals.Get("cdr"))
		}
	}

	var p jsonCDRPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.Variables == nil {
		writeError(w, http.StatusBadRequest, "validation_error", "body is not a json_cdr payload")
		return
	}
	v := p.Variables
	uuid := v["uuid"]
	if uuid == "" {
		uuid = v["call_uuid"]
	}
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "cdr has no uuid")
		return
	}
	rec := firstNonEmpty(v["record_path"], v["cc_record_filename"])
	// caller/destination/context: prefer the caller_profile, fall back to vars.
	var cp struct{ name, num, dest, ctx string }
	if len(p.Callflow) > 0 {
		c := p.Callflow[0].CallerProfile
		cp.name, cp.num, cp.dest, cp.ctx = c.CallerIDName, c.CallerIDNumber, c.DestinationNumber, c.Context
	}
	cdr := models.CDR{
		ID:                uuid,
		Direction:         v["direction"],
		CallerIDNumber:    firstNonEmpty(cp.num, v["caller_id_number"]),
		CallerIDName:      firstNonEmpty(cp.name, v["caller_id_name"]),
		DestinationNumber: firstNonEmpty(cp.dest, v["destination_number"]),
		Context:           firstNonEmpty(cp.ctx, v["context"]),
		HangupCause:       v["hangup_cause"],
		StartEpoch:        atoi64(v["start_epoch"]),
		AnswerEpoch:       atoi64(v["answer_epoch"]),
		EndEpoch:          atoi64(v["end_epoch"]),
		Duration:          atoi(v["duration"]),
		Billsec:           atoi(v["billsec"]),
		RecordingPath:     rec,
		Raw:               raw,
	}
	if err := s.store.InsertCDR(r.Context(), &cdr); err != nil {
		s.log.Error("insert cdr", "err", err)
		// 500 makes mod_json_cdr retry from its failure queue.
		writeError(w, http.StatusInternalServerError, "internal_error", "could not store cdr")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleListCDR — GET /api/v1/cdr?number=&cause=&from=&to=&answered=&limit=&offset=
// (from/to are unix epoch seconds). Newest first, X-Total-Count header.
func (s *Server) handleListCDR(w http.ResponseWriter, r *http.Request) {
	pg, ok := parsePage(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := models.CDRFilter{
		Number:       q.Get("number"),
		HangupCause:  q.Get("cause"),
		FromEpoch:    atoi64(q.Get("from")),
		ToEpoch:      atoi64(q.Get("to")),
		AnsweredOnly: q.Get("answered") == "true",
		Limit:        pg.limit,
		Offset:       pg.offset,
	}
	out, total, err := s.store.ListCDR(r.Context(), f)
	if writeStoreError(w, err) {
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, out)
}

// handleCDRStats — GET /api/v1/cdr/stats?from=&to=  per-day rollups.
func (s *Server) handleCDRStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stats, err := s.store.CDRStats(r.Context(), atoi64(q.Get("from")), atoi64(q.Get("to")))
	if writeStoreError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
