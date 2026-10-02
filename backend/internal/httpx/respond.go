package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"couchverse/internal/db"
)

// ErrNotFound aliases db.ErrNotFound so handlers can match store errors
// without importing the db package.
var ErrNotFound = db.ErrNotFound

// APIError is every error the API returns. It encodes as the envelope all
// clients parse, {"error":{"code","message"}}, and satisfies huma.StatusError
// so typed operations can return it directly.
type APIError struct {
	status int
	Body   ErrorBody `json:"error"`
}

// ErrorBody is the payload of an APIError. Code is stable and meant for
// programs; Message is a human-readable English hint.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string  { return e.Body.Message }
func (e *APIError) GetStatus() int { return e.status }

// Fail returns an APIError with an explicit status and code.
func Fail(status int, code, message string) *APIError {
	return &APIError{status: status, Body: ErrorBody{Code: code, Message: message}}
}

func NotFoundError() *APIError {
	return Fail(http.StatusNotFound, "not_found", "resource not found")
}

func BadRequestError(message string) *APIError {
	return Fail(http.StatusBadRequest, "bad_request", message)
}

// InternalError logs err and hides it behind a generic 500.
func InternalError(err error) *APIError {
	slog.Error("internal error", "err", err)
	return Fail(http.StatusInternalServerError, "internal", "internal server error")
}

// StoreError maps a store error to 404 for missing rows, 500 otherwise.
func StoreError(err error) *APIError {
	if errors.Is(err, ErrNotFound) {
		return NotFoundError()
	}
	return InternalError(err)
}

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
	JSON(w, status, Fail(status, code, message))
}

func Internal(w http.ResponseWriter, err error) {
	e := InternalError(err)
	JSON(w, e.status, e)
}

func NotFound(w http.ResponseWriter) {
	e := NotFoundError()
	JSON(w, e.status, e)
}

func BadRequest(w http.ResponseWriter, message string) {
	e := BadRequestError(message)
	JSON(w, e.status, e)
}

// StoreErr writes the response StoreError describes.
func StoreErr(w http.ResponseWriter, err error) {
	e := StoreError(err)
	JSON(w, e.status, e)
}

// Decode reads a JSON request body into v, capping the body size.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// ServeFile serves path like http.ServeFile, but a missing file gets the API's
// JSON 404 (file_missing) rather than a plain-text page.
func ServeFile(w http.ResponseWriter, r *http.Request, path string) {
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		Error(w, http.StatusNotFound, "file_missing", "the file is missing on disk")
		return
	}
	http.ServeFile(w, r, path)
}
