package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

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

type Credentials struct {
	Username string `json:"username" minLength:"1"`
	Password string `json:"password" minLength:"1"`
}

type loginInput struct {
	Body Credentials

	remoteAddr string
	userAgent  string
}

// Resolve captures the caller for throttling and the session row; neither is
// an API parameter.
func (in *loginInput) Resolve(ctx huma.Context) []error {
	// RemoteAddr is the bare client IP (server.clientIP resolves trusted proxies)
	in.remoteAddr = ctx.RemoteAddr()
	in.userAgent = ctx.Header("User-Agent")
	return nil
}

type loginOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      *User
}

func (a *Handlers) Login(ctx context.Context, in *loginInput) (*loginOutput, error) {
	req := in.Body
	if !a.limiter.allow(in.remoteAddr + "|" + req.Username) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
	}

	user, err := a.store.UserByUsername(ctx, req.Username)
	if err != nil && !errors.Is(err, httpx.ErrNotFound) {
		return nil, err
	}

	hash := dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	ok, err := VerifyPassword(req.Password, hash)
	if err != nil {
		return nil, err
	}
	if !ok || user == nil || user.Disabled {
		return nil, httpx.Fail(http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	}

	token, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(SessionTTL)
	if err := a.store.CreateSession(ctx, tokenHash, user.ID, expires, in.userAgent); err != nil {
		return nil, err
	}

	return &loginOutput{SetCookie: sessionCookie(token, a.cfg.CookieSecure), Body: user}, nil
}

type logoutInput struct {
	Session string `cookie:"couchverse_session"` // SessionCookie; tags cannot name a constant
}

type logoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

func (a *Handlers) Logout(ctx context.Context, in *logoutInput) (*logoutOutput, error) {
	if in.Session != "" {
		if err := a.store.DeleteSession(ctx, HashToken(in.Session)); err != nil && !errors.Is(err, httpx.ErrNotFound) {
			return nil, err
		}
	}
	return &logoutOutput{SetCookie: clearedSessionCookie(a.cfg.CookieSecure)}, nil
}

type userOutput struct{ Body *User }

func (a *Handlers) Me(ctx context.Context, _ *struct{}) (*userOutput, error) {
	return &userOutput{Body: UserFrom(ctx)}, nil
}
