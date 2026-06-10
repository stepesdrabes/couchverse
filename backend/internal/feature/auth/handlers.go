package auth

import (
	"errors"
	"net"
	"net/http"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/httpx"
)

type Handlers struct {
	store   *Store
	cfg     config.Config
	limiter *rateLimiter
}

func NewHandlers(st *Store, cfg config.Config) *Handlers {
	return &Handlers{
		store:   st,
		cfg:     cfg,
		limiter: newRateLimiter(5, time.Minute),
	}
}

// dummyHash keeps login timing constant when the username does not exist.
var dummyHash, _ = HashPassword("dummy-password-for-constant-timing")

func (a *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httpx.BadRequest(w, "username and password are required")
		return
	}
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if !a.limiter.allow(ip + "|" + req.Username) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
		return
	}

	user, err := a.store.UserByUsername(r.Context(), req.Username)
	if err != nil && !errors.Is(err, httpx.ErrNotFound) {
		httpx.Internal(w, err)
		return
	}

	hash := dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	ok, err := VerifyPassword(req.Password, hash)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !ok || user == nil || user.Disabled {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	token, tokenHash, err := NewToken()
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	expires := time.Now().Add(SessionTTL)
	if err := a.store.CreateSession(r.Context(), tokenHash, user.ID, expires, r.UserAgent()); err != nil {
		httpx.Internal(w, err)
		return
	}

	SetSessionCookie(w, token, a.cfg.CookieSecure)
	httpx.JSON(w, http.StatusOK, user)
}

func (a *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookie); err == nil && cookie.Value != "" {
		if err := a.store.DeleteSession(r.Context(), HashToken(cookie.Value)); err != nil && !errors.Is(err, httpx.ErrNotFound) {
			httpx.Internal(w, err)
			return
		}
	}
	ClearSessionCookie(w, a.cfg.CookieSecure)
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (a *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, UserFrom(r.Context()))
}
