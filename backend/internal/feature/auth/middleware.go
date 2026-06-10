package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"couchverse/internal/httpx"
)

type ctxKey int

const userKey ctxKey = iota

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

type Middleware struct {
	store *Store
}

func NewMiddleware(st *Store) *Middleware {
	return &Middleware{store: st}
}

// Load resolves the session cookie into a user on the request context.
// It never rejects - RequireAuth/RequireAdmin do that per route.
func (m *Middleware) Load(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := m.store.UserBySession(r.Context(), HashToken(cookie.Value))
		if err != nil {
			if !errors.Is(err, httpx.ErrNotFound) {
				httpx.Internal(w, err)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if u.Role != "admin" {
			httpx.Error(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFOrigin rejects state-changing cross-origin requests. Browsers always
// send Origin on those; requests without one (curl, same-origin GETs) pass.
func CSRFOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "null" {
			if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
				httpx.Error(w, http.StatusForbidden, "cross_origin", "cross-origin request rejected")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
