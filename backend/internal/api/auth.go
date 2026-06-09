package api

import (
	"errors"
	"net"
	"net/http"
	"time"

	"couchverse/internal/auth"
	"couchverse/internal/config"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Auth struct {
	store   *store.Store
	cfg     config.Config
	limiter *rateLimiter
}

func NewAuth(st *store.Store, cfg config.Config) *Auth {
	return &Auth{
		store:   st,
		cfg:     cfg,
		limiter: newRateLimiter(5, time.Minute),
	}
}

// dummyHash keeps login timing constant when the username does not exist.
var dummyHash, _ = auth.HashPassword("dummy-password-for-constant-timing")

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
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
	ok, err := auth.VerifyPassword(req.Password, hash)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !ok || user == nil || user.Disabled {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	token, tokenHash, err := auth.NewToken()
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	expires := time.Now().Add(auth.SessionTTL)
	if err := a.store.CreateSession(r.Context(), tokenHash, user.ID, expires, r.UserAgent()); err != nil {
		httpx.Internal(w, err)
		return
	}

	auth.SetSessionCookie(w, token, a.cfg.CookieSecure)
	httpx.JSON(w, http.StatusOK, user)
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookie); err == nil && cookie.Value != "" {
		if err := a.store.DeleteSession(r.Context(), auth.HashToken(cookie.Value)); err != nil && !errors.Is(err, httpx.ErrNotFound) {
			httpx.Internal(w, err)
			return
		}
	}
	auth.ClearSessionCookie(w, a.cfg.CookieSecure)
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, auth.UserFrom(r.Context()))
}
