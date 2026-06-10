package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"couchverse/internal/db"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Error apiError `json:"error"`
}

// ErrNotFound aliases db.ErrNotFound so handlers can match store errors
// without importing the db package.
var ErrNotFound = db.ErrNotFound

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}

func Internal(w http.ResponseWriter, err error) {
	slog.Error("internal error", "err", err)
	Error(w, http.StatusInternalServerError, "internal", "internal server error")
}

func NotFound(w http.ResponseWriter) {
	Error(w, http.StatusNotFound, "not_found", "resource not found")
}

func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, "bad_request", message)
}

// StoreErr maps a store error to 404 for missing rows, 500 otherwise.
func StoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		NotFound(w)
		return
	}
	Internal(w, err)
}

// Decode reads a JSON request body into v, capping the body size.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
