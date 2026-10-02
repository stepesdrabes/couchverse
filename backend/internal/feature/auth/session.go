package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"
)

const (
	SessionCookie = "couchverse_session"
	// SessionTTL is how long a session survives without being used; every use
	// slides the expiry forward again.
	SessionTTL = 30 * 24 * time.Hour
)

// Session is the session that authenticated a request.
type Session struct {
	ID        string
	Kind      string // browser | device
	ExpiresAt time.Time
}

// NewToken returns a random session token for the cookie and its SHA-256 hash
// for storage - a leaked sessions table cannot be used to forge cookies.
func NewToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func sessionCookie(token string, secure bool) http.Cookie {
	return http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func clearedSessionCookie(secure bool) http.Cookie {
	return http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
