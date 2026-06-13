package api

import (
	"net/http"
	"strconv"
)

const maxPageLimit = 1000

// page holds parsed ?limit=&offset= values. limit==0 means "no limit"
// (unbounded — preserves the pre-pagination behaviour for existing clients).
type page struct {
	limit  int
	offset int
}

// parsePage reads optional limit/offset. Returns ok=false and writes a 400 on
// malformed input.
func parsePage(w http.ResponseWriter, r *http.Request) (page, bool) {
	var p page
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "limit must be a non-negative integer")
			return p, false
		}
		if n > maxPageLimit {
			n = maxPageLimit
		}
		p.limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "offset must be a non-negative integer")
			return p, false
		}
		p.offset = n
	}
	return p, true
}

// apply slices items per the page window and returns the page plus the total
// count of the full set. limit==0 returns everything from offset on.
func apply[T any](items []T, p page) (out []T, total int) {
	total = len(items)
	if p.offset >= total {
		return []T{}, total
	}
	items = items[p.offset:]
	if p.limit > 0 && p.limit < len(items) {
		items = items[:p.limit]
	}
	return items, total
}

// writeList writes a paginated list response: a bare JSON array (unchanged
// shape for existing consumers) plus an X-Total-Count header.
func writeList[T any](w http.ResponseWriter, items []T, p page) {
	out, total := apply(items, p)
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, out)
}
