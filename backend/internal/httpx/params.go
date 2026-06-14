package httpx

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type langCtxKey struct{}

// Lang reads the display-language query param (?lang=), "" when absent.
func Lang(r *http.Request) string { return r.URL.Query().Get("lang") }

// WithLang stores the display language on the context so stores can resolve
// translated catalog text. Empty (admin reads, background jobs) means base text.
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langCtxKey{}, lang)
}

// LangFrom returns the display language set by WithLang, "" when unset.
func LangFrom(ctx context.Context) string {
	lang, _ := ctx.Value(langCtxKey{}).(string)
	return lang
}

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
