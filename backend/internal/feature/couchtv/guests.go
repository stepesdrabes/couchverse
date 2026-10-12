package couchtv

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Guests (D23) watch the live channel without an account: anyone with the channel
// link types a name and gets a guest token, as a cookie scoped to the channel's
// routes (or in the body for a native client, sent back as GuestTokenHeader). The
// link's code is stored hashed; rotating it or turning it off ends every guest.

const (
	// GuestCookie carries a guest's token, only to the channel's routes (/api/v1/tv).
	GuestCookie = "couchverse_tv"
	// GuestTokenHeader carries the same token for clients without a cookie jar.
	GuestTokenHeader = "X-Couch-Tv-Token"
	guestCookiePath  = "/api/v1/tv"
	// guestKeep is how long a guest lasts without tuning in; the cookie as well.
	guestKeep = 30 * 24 * time.Hour
	// guestCheckTTL is how long the stream trusts a guest check; ending the guests
	// clears it at once.
	guestCheckTTL = 30 * time.Second
)

var errNoGuest = errors.New("couchtv: no such guest")

// Guest is a viewer who joined through the channel link.
type Guest struct {
	ID   string
	Name string
}

// secret returns a random URL-safe token and its hex SHA-256 hash, which is all
// that is stored.
func secret() (token, hash string) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		panic("couchtv: crypto/rand failed: " + err.Error())
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashSecret(token)
}

func hashSecret(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// CreateGuest stores a guest who joined through the link with linkHash.
func (s *Store) CreateGuest(ctx context.Context, name, tokenHash, linkHash string) (Guest, error) {
	g := Guest{Name: name}
	err := s.q.QueryRow(ctx, `INSERT INTO couchtv_guests (token_hash, name, link_hash) VALUES ($1, $2, $3) RETURNING id`,
		tokenHash, name, linkHash).Scan(&g.ID)
	return g, err
}

// SeeGuest reads the guest holding a token and notes that they are around; it
// answers the link they joined through.
func (s *Store) SeeGuest(ctx context.Context, tokenHash string) (Guest, string, error) {
	var g Guest
	var link string
	err := s.q.QueryRow(ctx, `UPDATE couchtv_guests SET seen_at = now() WHERE token_hash = $1 RETURNING id, name, link_hash`,
		tokenHash).Scan(&g.ID, &g.Name, &link)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, "", db.ErrNotFound
	}
	return g, link, err
}

// GuestLink is the link a guest joined through.
func (s *Store) GuestLink(ctx context.Context, id string) (string, error) {
	var link string
	err := s.q.QueryRow(ctx, `SELECT link_hash FROM couchtv_guests WHERE id = $1`, id).Scan(&link)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", db.ErrNotFound
	}
	return link, err
}

// EndGuests forgets every guest.
func (s *Store) EndGuests(ctx context.Context) error {
	_, err := s.q.Exec(ctx, `DELETE FROM couchtv_guests`)
	return err
}

// Guests runs the channel link: joining, guest tokens, and the admin's rotate and
// switch.
type Guests struct {
	store    *Store
	settings *settings.Store
	hub      *Hub
	secure   bool
	joinRate *rateLimiter
	now      func() time.Time

	mu      sync.Mutex
	checked map[string]guestCheck
}

type guestCheck struct {
	ok bool
	at time.Time
}

// NewGuests runs the channel link; hub is told when the guests end. secure marks
// the guest cookie Secure.
func NewGuests(st *Store, set *settings.Store, hub *Hub, secure bool) *Guests {
	return &Guests{store: st, settings: set, hub: hub, secure: secure, joinRate: newRateLimiter(10, time.Minute),
		now: time.Now, checked: map[string]guestCheck{}}
}

// Lookup is the guest holding token while the link they joined through still works.
func (g *Guests) Lookup(ctx context.Context, token string) (Guest, error) {
	if token == "" {
		return Guest{}, errNoGuest
	}
	guest, link, err := g.store.SeeGuest(ctx, hashSecret(token))
	if errors.Is(err, db.ErrNotFound) {
		return Guest{}, errNoGuest
	}
	if err != nil {
		return Guest{}, err
	}
	if !g.linkWorks(ctx, link) {
		return Guest{}, errNoGuest
	}
	return guest, nil
}

// linkWorks reports whether a link (by its hash) is the channel's, and on.
func (g *Guests) linkWorks(ctx context.Context, hash string) bool {
	stored := loadStored(ctx, g.settings)
	return stored.GuestLink.Enabled && stored.GuestLinkHash != "" &&
		subtle.ConstantTimeCompare([]byte(hash), []byte(stored.GuestLinkHash)) == 1
}

// Allows reports whether a guest's channel grant still holds: the guest exists and
// the link they joined through works. The stream asks it on every request a
// guest's grant carries, so the answer is kept for a moment.
func (g *Guests) Allows(ctx context.Context, guestID string) bool {
	now := g.now()
	g.mu.Lock()
	c, ok := g.checked[guestID]
	g.mu.Unlock()
	if ok && now.Sub(c.at) < guestCheckTTL {
		return c.ok
	}
	link, err := g.store.GuestLink(ctx, guestID)
	allowed := err == nil && g.linkWorks(ctx, link)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		// a database hiccup is no reason to cut a guest off, nor to remember it
		return true
	}
	g.mu.Lock()
	if len(g.checked) > 1000 {
		g.checked = map[string]guestCheck{}
	}
	g.checked[guestID] = guestCheck{ok: allowed, at: now}
	g.mu.Unlock()
	return allowed
}

// end forgets every guest and closes their sockets.
func (g *Guests) end(ctx context.Context) error {
	if err := g.store.EndGuests(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	g.checked = map[string]guestCheck{}
	g.mu.Unlock()
	if g.hub != nil {
		g.hub.EndGuests()
	}
	return nil
}

func guestCookie(token string, secure bool) http.Cookie {
	return http.Cookie{
		Name:     GuestCookie,
		Value:    token,
		Path:     guestCookiePath,
		MaxAge:   int(guestKeep.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// guestToken is a request's guest token: the header, else the cookie.
func guestToken(header, cookie string) string {
	if header != "" {
		return header
	}
	return cookie
}

// CouchTvJoin is what a guest types to join.
type CouchTvJoin struct {
	Name string `json:"name" minLength:"1" maxLength:"32" doc:"The name the guest shows as to everyone watching."`
}

// CouchTvGuest is a guest who joined through the channel link.
type CouchTvGuest struct {
	ID    string `json:"id" format:"uuid"`
	Name  string `json:"name"`
	Token string `json:"token,omitempty" doc:"The guest token, only with delivery=body: send it as the X-Couch-Tv-Token header where a browser sends the channel cookie."`
}

type joinInput struct {
	Delivery string `query:"delivery" enum:"cookie,body" default:"cookie" doc:"body returns the guest token in the response instead of setting the channel cookie."`
	Code     string `path:"code" maxLength:"64" doc:"The code from the channel link."`
	Body     CouchTvJoin
	clientIP string
}

// Resolve captures the caller's address for the join rate limit; the composition
// root has already reduced RemoteAddr to the bare client IP.
func (in *joinInput) Resolve(ctx huma.Context) []error {
	in.clientIP = ctx.RemoteAddr()
	return nil
}

type joinOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      CouchTvGuest
}

// guestName trims a typed name and refuses one without a visible character.
func guestName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", false
	}
	return name, true
}

// Join lets a guest in through the channel link, as long as it works.
func (g *Guests) Join(ctx context.Context, in *joinInput) (*joinOutput, error) {
	if !g.joinRate.allow(in.clientIP, g.now()) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many join attempts, slow down")
	}
	name, ok := guestName(in.Body.Name)
	if !ok {
		return nil, httpx.Fail(http.StatusBadRequest, "invalid_name", "type a name to show")
	}
	link := hashSecret(in.Code)
	if !g.linkWorks(ctx, link) {
		return nil, httpx.Fail(http.StatusNotFound, "no_channel_link", "this Couch TV link does not work")
	}
	token, hash := secret()
	guest, err := g.store.CreateGuest(ctx, name, hash, link)
	if err != nil {
		return nil, err
	}
	out := &joinOutput{Body: CouchTvGuest{ID: guest.ID, Name: guest.Name}}
	if in.Delivery == "body" {
		out.Body.Token = token
	} else {
		out.SetCookie = []http.Cookie{guestCookie(token, g.secure)}
	}
	return out, nil
}

// CouchTvLink is a new channel link, shown this once.
type CouchTvLink struct {
	Code string           `json:"code" doc:"The link's code; only its hash is kept."`
	Path string           `json:"path" doc:"The web page guests open: the server's address followed by this path."`
	Link CouchTvGuestLink `json:"link"`
}

// CouchTvLinkSwitch turns the channel link on or off.
type CouchTvLinkSwitch struct {
	Enabled bool `json:"enabled"`
}

type linkOutput struct{ Body CouchTvLink }

// Rotate makes a new channel link (turning it on) and ends every guest of the old one.
func (g *Guests) Rotate(ctx context.Context, _ *struct{}) (*linkOutput, error) {
	code, hash := secret()
	stored := loadStored(ctx, g.settings)
	now := g.now().UTC().Truncate(time.Second)
	stored.GuestLink = CouchTvGuestLink{Enabled: true, CreatedAt: &now}
	stored.GuestLinkHash = hash
	if err := g.save(ctx, stored); err != nil {
		return nil, err
	}
	if err := g.end(ctx); err != nil {
		return nil, err
	}
	return &linkOutput{Body: CouchTvLink{Code: code, Path: "/tv/join/" + code, Link: stored.GuestLink}}, nil
}

type switchInput struct{ Body CouchTvLinkSwitch }

type switchOutput struct{ Body CouchTvGuestLink }

// Switch turns the channel link on or off; off ends every guest.
func (g *Guests) Switch(ctx context.Context, in *switchInput) (*switchOutput, error) {
	stored := loadStored(ctx, g.settings)
	if in.Body.Enabled && stored.GuestLinkHash == "" {
		return nil, httpx.Fail(http.StatusConflict, "no_channel_link", "there is no link yet: make one first")
	}
	stored.GuestLink.Enabled = in.Body.Enabled
	if err := g.save(ctx, stored); err != nil {
		return nil, err
	}
	if !in.Body.Enabled {
		if err := g.end(ctx); err != nil {
			return nil, err
		}
	}
	return &switchOutput{Body: stored.GuestLink}, nil
}

func (g *Guests) save(ctx context.Context, stored storedConfig) error {
	body, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return g.settings.Set(ctx, settingsKey, body)
}
