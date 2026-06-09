package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ID parses a chi URL parameter as int64, returning 0 when missing/invalid.
func ID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
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
