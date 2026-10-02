// Package grant signs short-lived capabilities that authorize media requests by
// URL alone. AVPlayer, ExoPlayer, AirPlay receivers and the browser's media
// element cannot reliably attach a cookie or an Authorization header to every
// segment they fetch, so the grant travels in the path (/media/{grant}/...),
// where relative HLS URIs inherit it.
package grant

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/settings"
)

// Scope says what a grant unlocks.
type Scope byte

const (
	// Media is one media file: its stream, HLS, frames, subtitles and instant-play
	// sessions.
	Media Scope = 1
	// Artwork is every artwork image, for images a system fetches without the
	// app's session (tvOS Top Shelf, AirPlay receivers, anonymous couch guests).
	Artwork Scope = 2
)

// Lifetimes: a media grant outlives any film and is renewed by fetching the
// playback payload again; artwork grants back system caches that refresh daily.
const (
	MediaTTL   = 6 * time.Hour
	ArtworkTTL = 7 * 24 * time.Hour
)

// Grant is a verified capability.
type Grant struct {
	Scope Scope
	// Subject is the user the grant was issued to, 0 for an anonymous couch guest.
	Subject int64
	// Resource is the media file of a Media grant, the zero UUID otherwise.
	Resource uuid.UUID
	// Couch binds a couch follower's grant to their participant: it only works
	// while they are on a live couch playing Resource, so leaving, a media switch
	// or the session's end revokes it at once. Zero for a grant that stands on
	// its own.
	Couch   uuid.UUID
	Expires time.Time
}

var (
	ErrInvalid = errors.New("grant: invalid")
	ErrExpired = errors.New("grant: expired")
)

const (
	formatVersion = 1
	payloadLen    = 1 + 1 + 4 + 8 + 16 + 16 // version, scope, expiry, subject, resource, couch
	// HMAC-SHA256 truncated to 128 bits: short URLs, still far beyond guessing
	macLen = 16
)

type Signer struct {
	key []byte
	now func() time.Time
}

func NewSigner(key []byte) *Signer {
	return &Signer{key: key, now: time.Now}
}

// Sign encodes g as a URL-safe token of 83 characters.
func (s *Signer) Sign(g Grant) string {
	buf := make([]byte, payloadLen, payloadLen+macLen)
	buf[0] = formatVersion
	buf[1] = byte(g.Scope)
	binary.BigEndian.PutUint32(buf[2:6], uint32(g.Expires.Unix()))
	binary.BigEndian.PutUint64(buf[6:14], uint64(g.Subject))
	copy(buf[14:30], g.Resource[:])
	copy(buf[30:46], g.Couch[:])
	return base64.RawURLEncoding.EncodeToString(append(buf, s.mac(buf)...))
}

// Issue signs g with an expiry ttl from now.
func (s *Signer) Issue(g Grant, ttl time.Duration) string {
	g.Expires = s.now().Add(ttl)
	return s.Sign(g)
}

// Verify decodes a token and checks its signature and expiry.
func (s *Signer) Verify(token string) (Grant, error) {
	// strict: the last character's spare bits must be zero, so a grant has
	// exactly one spelling
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != payloadLen+macLen || raw[0] != formatVersion {
		return Grant{}, ErrInvalid
	}
	payload, tag := raw[:payloadLen], raw[payloadLen:]
	if !hmac.Equal(tag, s.mac(payload)) {
		return Grant{}, ErrInvalid
	}
	g := Grant{
		Scope:   Scope(payload[1]),
		Expires: time.Unix(int64(binary.BigEndian.Uint32(payload[2:6])), 0),
		Subject: int64(binary.BigEndian.Uint64(payload[6:14])),
	}
	copy(g.Resource[:], payload[14:30])
	copy(g.Couch[:], payload[30:46])
	if !s.now().Before(g.Expires) {
		return g, ErrExpired
	}
	return g, nil
}

func (s *Signer) mac(payload []byte) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write(payload)
	return m.Sum(nil)[:macLen]
}

const secretKey = "grant.secret"

// LoadSecret returns the server's signing key, creating it on first boot.
// Deleting the setting revokes every outstanding grant.
func LoadSecret(ctx context.Context, set *settings.Store) ([]byte, error) {
	raw, err := set.Get(ctx, secretKey)
	if err != nil {
		return nil, err
	}
	var key []byte
	if raw != nil && json.Unmarshal(raw, &key) == nil && len(key) == 32 {
		return key, nil
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	stored, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	return key, set.Set(ctx, secretKey, stored)
}

type ctxKey struct{}

// WithGrant stores the request's verified grant.
func WithGrant(ctx context.Context, g Grant) context.Context {
	return context.WithValue(ctx, ctxKey{}, g)
}

// From returns the request's verified grant.
func From(ctx context.Context) (Grant, bool) {
	g, ok := ctx.Value(ctxKey{}).(Grant)
	return g, ok
}
