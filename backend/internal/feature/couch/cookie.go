package couch

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	// CouchCookie carries an opaque participant token. All authority lives in
	// the in-memory Hub; the cookie is only a lookup handle. Mirrors the auth
	// session cookie's attributes (HttpOnly, SameSite=Lax, Secure when configured).
	CouchCookie = "couchverse_couch"
	// Short-lived; refreshed while the participant is connected. A leaked cookie
	// dies quickly, and a server restart invalidates every token regardless.
	couchCookieTTL = 6 * time.Hour
)

// newToken returns a random participant token for the cookie and its hex SHA-256
// hash for the in-memory index, mirroring auth.NewToken: a heap dump cannot be
// used to forge cookies.
func newToken() (token, hash string) {
	token = randToken(32)
	return token, hashToken(token)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// randToken returns n cryptographically-random bytes as a URL-safe string.
func randToken(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic("couch: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// randDigits returns an n-digit numeric code (the human-shareable join code).
// Unlike the participant token it is short and brute-forceable, so joins are
// rate-limited and a guessed code only grants the same access a leaked link does.
func randDigits(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic("couch: crypto/rand failed: " + err.Error())
	}
	b := make([]byte, n)
	for i, v := range raw {
		b[i] = '0' + (v % 10)
	}
	return string(b)
}

func setCouchCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CouchCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(couchCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCouchCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CouchCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
