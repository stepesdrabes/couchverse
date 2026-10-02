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
	// pairingLimiter bounds the other unauthenticated ways in (pairing starts,
	// connect-code redemptions) per IP
	pairingLimiter *rateLimiter
}

func NewHandlers(st *Store, cfg config.Config) *Handlers {
	return &Handlers{
		store:          st,
		cfg:            cfg,
		limiter:        newRateLimiter(5, time.Minute),
		pairingLimiter: newRateLimiter(10, time.Minute),
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
	user, err := a.authenticate(ctx, in.remoteAddr, in.Body)
	if err != nil {
		return nil, err
	}
	token, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	if _, err := a.store.CreateSession(ctx, NewSession{
		TokenHash: tokenHash, UserID: user.ID, Kind: "browser", UserAgent: in.userAgent,
	}); err != nil {
		return nil, err
	}
	return &loginOutput{SetCookie: sessionCookie(token, a.cfg.CookieSecure), Body: user}, nil
}

// authenticate checks a username and password, throttled per client IP and
// username, in constant time whether or not the user exists.
func (a *Handlers) authenticate(ctx context.Context, ip string, c Credentials) (*User, error) {
	if !a.limiter.allow(ip + "|" + c.Username) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
	}
	user, err := a.store.UserByUsername(ctx, c.Username)
	if err != nil && !errors.Is(err, httpx.ErrNotFound) {
		return nil, err
	}
	hash := dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	ok, err := VerifyPassword(c.Password, hash)
	if err != nil {
		return nil, err
	}
	if !ok || user == nil || user.Disabled {
		return nil, httpx.Fail(http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	}
	return user, nil
}

type logoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

// Logout ends the session that made the request, whether a browser cookie or a
// device token, and clears the cookie either way.
func (a *Handlers) Logout(ctx context.Context, _ *struct{}) (*logoutOutput, error) {
	if sess := SessionFrom(ctx); sess != nil {
		if err := a.store.DeleteSession(ctx, sess.ID); err != nil && !errors.Is(err, httpx.ErrNotFound) {
			return nil, err
		}
	}
	return &logoutOutput{SetCookie: clearedSessionCookie(a.cfg.CookieSecure)}, nil
}

type userOutput struct{ Body *User }

func (a *Handlers) Me(ctx context.Context, _ *struct{}) (*userOutput, error) {
	return &userOutput{Body: UserFrom(ctx)}, nil
}
