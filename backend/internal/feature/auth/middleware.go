package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"couchverse/internal/httpx"
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

// SessionFrom returns the session that authenticated the request, nil when anonymous.
func SessionFrom(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionKey).(*Session)
	return s
}

type Middleware struct {
	store  *Store
	secure bool
}

func NewMiddleware(st *Store, cookieSecure bool) *Middleware {
	return &Middleware{store: st, secure: cookieSecure}
}

// Load resolves the bearer token (devices) or session cookie (browsers) into a
// user and session on the request context. It never rejects - the SignedIn and
// Admin guards do that per route group.
func (m *Middleware) Load(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, fromCookie := requestToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, sess, extended, err := m.store.UserBySession(r.Context(), HashToken(token))
		if err != nil {
			if !errors.Is(err, httpx.ErrNotFound) {
				httpx.Internal(w, err)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if extended && fromCookie {
			cookie := sessionCookie(token, m.secure)
			http.SetCookie(w, &cookie)
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, sessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestToken(r *http.Request) (token string, fromCookie bool) {
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return strings.TrimSpace(bearer), false
	}
	if cookie, err := r.Cookie(SessionCookie); err == nil {
		return cookie.Value, true
	}
	return "", false
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
