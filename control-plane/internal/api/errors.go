package api

import (
	"errors"
	"net/http"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/store"
)

// writeStoreError maps a store error to an HTTP response and reports whether
// it handled the error (true) or the caller should continue (false, err==nil).
func writeStoreError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, store.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "resource already exists")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "resource conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
	return true
}
