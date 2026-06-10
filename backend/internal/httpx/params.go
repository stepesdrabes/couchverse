package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ID parses a chi URL parameter as int64, returning 0 when missing/invalid.
func ID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}

// UUID reads a chi URL parameter as a lowercased uuid, returning "" when it
// doesn't look like one (callers treat "" as not found).
func UUID(r *http.Request, name string) string {
	v := strings.ToLower(chi.URLParam(r, name))
	if len(v) != 36 {
		return ""
	}
	for i, c := range v {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return ""
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return ""
			}
		}
	}
	return v
}

func QueryInt(r *http.Request, name string, def int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
