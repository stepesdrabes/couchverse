package couch

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/playback"
	"couchverse/internal/httpx"
)

type Handlers struct {
	hub      *Hub
	joinRate *rateLimiter
}

// hostSignedIn gates hosting ahead of input parsing: only a logged-in user may
// host, and an anonymous caller learns that before anything else.
func hostSignedIn(ctx context.Context) error {
	if auth.UserFrom(ctx) == nil {
		return httpx.Fail(http.StatusUnauthorized, "unauthorized", "log in to host a couch session")
	}
	return nil
}

// CouchStart names what the host is watching when they open a couch.
type CouchStart struct {
	Kind string `json:"kind" enum:"movie,episode"`
	ID   string `json:"id" format:"uuid" doc:"The movie's title id or the episode id."`
}

// deliver puts a new participant token where the client asked for it: browsers get the
// httpOnly couch cookie, native apps (no cookie jar) get it in the body and send it back as
// X-Couch-Token. Huma reads no parameters from embedded structs, so each input declares its
// own delivery field.
func deliver(delivery, token string, session CouchSession, secure bool) *couchSessionOutput {
	if delivery == "body" {
		session.ParticipantToken = token
		return &couchSessionOutput{Body: session}
	}
	return &couchSessionOutput{SetCookie: []http.Cookie{couchCookie(token, secure)}, Body: session}
}

type createCouchInput struct {
	Delivery string `query:"delivery" enum:"cookie,body" default:"cookie" doc:"body returns the participant token in the response instead of setting the couch cookie."`
	Body     CouchStart
}

// couchSessionOutput carries the participant's token (as the couch cookie or in the body),
// which authorizes the socket, the follower payload and anonymous streaming.
type couchSessionOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      CouchSession
}

// Create starts (or reclaims) a couch session for the logged-in host watching a
// movie/episode and sets the host's couch cookie.
func (h *Handlers) Create(ctx context.Context, in *createCouchInput) (*couchSessionOutput, error) {
	user := auth.UserFrom(ctx)
	ref := CouchMediaRef{Kind: in.Body.Kind}
	if ref.Kind == "movie" {
		ref.TitleID = in.Body.ID
	} else {
		ref.EpisodeID = in.Body.ID
	}
	rm, host, token, created, err := h.hub.createOrReclaim(ctx, user, ref)
	if err != nil {
		return nil, err
	}
	if created && h.hub.deps.Stats != nil {
		// best effort - the counter must never fail hosting a session
		if err := h.hub.deps.Stats.RecordCouchHosted(ctx, user.ID); err != nil {
			slog.Warn("record couch hosted", "err", err)
		}
	}
	session := rm.snapshotFor(host.ID, roleHost)
	session.ArtworkGrant = h.artworkGrant(user)
	return deliver(in.Delivery, token, session, h.hub.secure), nil
}

// artworkGrant lets a guest without an account load the couch's artwork.
func (h *Handlers) artworkGrant(user *auth.User) string {
	var subject int64
	if user != nil {
		subject = user.ID
	}
	return artwork.IssueGrant(h.hub.deps.Grants, subject).Grant
}

type shareInput struct {
	Token string `path:"token" doc:"The session's share code."`
}

type couchInfoOutput struct{ Body CouchInfo }

func errNoSession(message string) error {
	return httpx.Fail(http.StatusNotFound, "no_session", message)
}

// Info previews a session (host, what's playing, participant count) without
// joining, for the pre-join "Start watching" screen. Public.
func (h *Handlers) Info(ctx context.Context, in *shareInput) (*couchInfoOutput, error) {
	rm := h.hub.roomByShare(in.Token)
	if rm == nil {
		return nil, errNoSession("this couch session does not exist or has ended")
	}
	info, ref := rm.infoPreview()
	if ref.Kind != "" {
		if pi, err := h.hub.deps.Playback.BuildPlayback(ctx, ref.Kind, ref.playbackID(), playback.Viewer{}, playback.LegacyProfile(nil)); err == nil {
			info.Display = &CouchInfoDisplay{
				Title:          pi.Display.Title,
				Subtitle:       pi.Display.Subtitle,
				BackdropID:     pi.Display.BackdropID,
				BackdropAccent: pi.Display.BackdropAccent,
			}
		}
	}
	info.ArtworkGrant = h.artworkGrant(auth.UserFrom(ctx))
	return &couchInfoOutput{Body: info}, nil
}

type joinCouchInput struct {
	Delivery string `query:"delivery" enum:"cookie,body" default:"cookie" doc:"body returns the participant token in the response instead of setting the couch cookie."`
	Token    string `path:"token" doc:"The session's share code."`
	Remote   bool   `query:"remote" doc:"Join as a remote for the host's own player; 403 not_host for anyone but the host's account, which joins as a remote either way."`
	Cookie   string `cookie:"couchverse_couch" doc:"The participant cookie this browser already holds, if any: a browser playing for the session as its host cannot join it too (409 already_hosting)."`
	clientIP string
}

// Resolve captures the caller's address for the join rate limit; the
// composition root has already reduced RemoteAddr to the bare client IP.
func (in *joinCouchInput) Resolve(ctx huma.Context) []error {
	in.clientIP = ctx.RemoteAddr()
	return nil
}

// Join adds the caller (logged-in or anonymous) to a session via its share token
// and sets their couch cookie.
func (h *Handlers) Join(ctx context.Context, in *joinCouchInput) (*couchSessionOutput, error) {
	if !h.joinRate.allow(in.clientIP) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many join attempts, slow down")
	}
	rm := h.hub.roomByShare(in.Token)
	if rm == nil {
		return nil, errNoSession("this couch session does not exist or has ended")
	}
	// a browser keeps one couch cookie for all its tabs: a tab joining the session another
	// tab plays for as its host would take the player's token away
	if in.Delivery != "body" && h.hub.playsFor(rm, in.Cookie) {
		return nil, httpx.Fail(http.StatusConflict, "already_hosting", "this browser is hosting the session")
	}
	user := auth.UserFrom(ctx)
	p, token, role, err := h.hub.join(rm, user, in.Remote)
	switch {
	case errors.Is(err, errNotHost):
		return nil, httpx.Fail(http.StatusForbidden, "not_host", "only the host can join as a remote")
	case errors.Is(err, errRoomFull):
		return nil, httpx.Fail(http.StatusConflict, "session_full", "this couch session is full")
	case errors.Is(err, errNotLive):
		return nil, errNoSession("this couch session has ended")
	case err != nil:
		return nil, err
	}
	// only a logged-in follower counts; the host's own remote does not
	if user != nil && role == roleFollower && h.hub.deps.Stats != nil {
		if err := h.hub.deps.Stats.RecordCouchJoined(ctx, user.ID); err != nil {
			slog.Warn("record couch joined", "err", err)
		}
	}
	session := rm.snapshotFor(p.ID, role)
	session.ArtworkGrant = h.artworkGrant(user)
	return deliver(in.Delivery, token, session, h.hub.secure), nil
}

// participantToken is the caller's token: the X-Couch-Token header, else the couch cookie.
// The share token in the path is only routing.
func participantToken(header, cookie string) string {
	if header != "" {
		return header
	}
	return cookie
}

type participantInput struct {
	Token      string `path:"token" doc:"The session's share code."`
	Cookie     string `cookie:"couchverse_couch" doc:"The participant cookie set by createCouch or joinCouch."`
	CouchToken string `header:"X-Couch-Token" doc:"The participant token from createCouch or joinCouch with delivery=body."`
}

type clearCookieOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

// Leave drops the caller (identified by their couch cookie) from the session.
func (h *Handlers) Leave(_ context.Context, in *participantInput) (*clearCookieOutput, error) {
	if token := participantToken(in.CouchToken, in.Cookie); token != "" {
		h.hub.leaveByToken(token)
	}
	return &clearCookieOutput{SetCookie: clearedCouchCookie(h.hub.secure)}, nil
}

// End terminates the session; only one of the host's devices (a remote included) may.
func (h *Handlers) End(_ context.Context, in *participantInput) (*clearCookieOutput, error) {
	if token := participantToken(in.CouchToken, in.Cookie); token == "" || !h.hub.endByHostToken(token) {
		return nil, httpx.Fail(http.StatusForbidden, "not_host", "only the host can end the session")
	}
	return &clearCookieOutput{SetCookie: clearedCouchCookie(h.hub.secure)}, nil
}

// CouchPlayback is a follower's player for the session's current media.
type CouchPlayback struct {
	Media  CouchMediaRef          `json:"media"`
	Player *playback.PlaybackInfo `json:"player,omitempty" doc:"Absent while the host is choosing what to watch."`
}

type couchPlaybackInput struct {
	Token      string   `path:"token" doc:"The session's share code."`
	Cookie     string   `cookie:"couchverse_couch" doc:"The participant cookie set by createCouch or joinCouch."`
	CouchToken string   `header:"X-Couch-Token" doc:"The participant token from createCouch or joinCouch with delivery=body."`
	Caps       []string `query:"caps" doc:"Video codecs the client decodes beyond the h264/vp9/av1 baseline (e.g. hevc). Superseded by resolveCouchPlayback, which takes a full device profile."`
}

type resolveCouchPlaybackInput struct {
	Token      string `path:"token" doc:"The session's share code."`
	Cookie     string `cookie:"couchverse_couch" doc:"The participant cookie set by createCouch or joinCouch."`
	CouchToken string `header:"X-Couch-Token" doc:"The participant token from createCouch or joinCouch with delivery=body."`
	Body       playback.DeviceProfile
}

type couchPlaybackOutput struct{ Body CouchPlayback }

func errNoCouchSession() error {
	return httpx.Fail(http.StatusUnauthorized, "no_couch_session", "join the couch session first")
}

// Playback returns the follower player payload for the browser baseline plus ?caps.
func (h *Handlers) Playback(ctx context.Context, in *couchPlaybackInput) (*couchPlaybackOutput, error) {
	return h.playback(ctx, participantToken(in.CouchToken, in.Cookie), playback.LegacyProfile(in.Caps))
}

// ResolvePlayback returns the follower player payload for the device profile in the body.
func (h *Handlers) ResolvePlayback(ctx context.Context, in *resolveCouchPlaybackInput) (*couchPlaybackOutput, error) {
	return h.playback(ctx, participantToken(in.CouchToken, in.Cookie), in.Body)
}

// playback builds the follower player payload for the session's current media,
// authorized by the participant token. This is how a follower (incl. anonymous) gets
// its media grants, bound to the session, without the auth-only /playback.
func (h *Handlers) playback(ctx context.Context, token string, profile playback.DeviceProfile) (*couchPlaybackOutput, error) {
	who, ok := h.hub.lookup(token)
	if !ok {
		return nil, errNoCouchSession()
	}
	rm, pid := who.room, who.pid
	rm.mu.Lock()
	ref := rm.state.Media
	live := rm.live
	rm.lastActive = time.Now()
	rm.mu.Unlock()
	viewer := playback.Viewer{Couch: pid}
	if user := auth.UserFrom(ctx); user != nil {
		viewer.UserID = user.ID
	}
	if !live {
		return nil, httpx.Fail(http.StatusGone, "session_ended", "this couch session has ended")
	}
	resp := CouchPlayback{Media: ref}
	if ref.Kind != "" {
		info, err := h.hub.deps.Playback.BuildPlayback(ctx, ref.Kind, ref.playbackID(), viewer, profile)
		if err != nil {
			return nil, err
		}
		resp.Player = info
	}
	return &couchPlaybackOutput{Body: resp}, nil
}
