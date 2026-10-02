package couch

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

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

type createCouchInput struct{ Body CouchStart }

// couchSessionOutput carries the participant's couch cookie, which
// authorizes the socket, the follower payload and anonymous streaming.
type couchSessionOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      CouchSession
}

// Create starts (or reclaims) a couch session for the logged-in host watching a
// movie/episode and sets the host's couch cookie.
func (h *Handlers) Create(ctx context.Context, in *createCouchInput) (*couchSessionOutput, error) {
	user := auth.UserFrom(ctx)
	ref := mediaRef{Kind: in.Body.Kind}
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
	return &couchSessionOutput{
		SetCookie: couchCookie(token, h.hub.secure),
		Body:      rm.snapshotFor(host.ID, "host"),
	}, nil
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
		if pi, err := h.hub.deps.Playback.BuildPlayback(ctx, ref.Kind, ref.playbackID(), nil, nil); err == nil {
			info.Display = &CouchInfoDisplay{
				Title:          pi.Display.Title,
				Subtitle:       pi.Display.Subtitle,
				BackdropID:     pi.Display.BackdropID,
				BackdropAccent: pi.Display.BackdropAccent,
			}
		}
	}
	return &couchInfoOutput{Body: info}, nil
}

type joinCouchInput struct {
	Token    string `path:"token" doc:"The session's share code."`
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
	user := auth.UserFrom(ctx)
	p, token, role, err := h.hub.join(rm, user)
	switch {
	case errors.Is(err, errRoomFull):
		return nil, httpx.Fail(http.StatusConflict, "session_full", "this couch session is full")
	case errors.Is(err, errNotLive):
		return nil, errNoSession("this couch session has ended")
	case err != nil:
		return nil, err
	}
	// only a logged-in follower counts; the host reclaiming their own link does not
	if user != nil && role == "follower" && h.hub.deps.Stats != nil {
		if err := h.hub.deps.Stats.RecordCouchJoined(ctx, user.ID); err != nil {
			slog.Warn("record couch joined", "err", err)
		}
	}
	return &couchSessionOutput{
		SetCookie: couchCookie(token, h.hub.secure),
		Body:      rm.snapshotFor(p.ID, role),
	}, nil
}

// participantInput identifies the caller by their couch cookie; the share
// token in the path is only routing.
type participantInput struct {
	Token  string `path:"token" doc:"The session's share code."`
	Cookie string `cookie:"couchverse_couch" doc:"The participant cookie set by createCouch or joinCouch."`
}

type clearCookieOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

// Leave drops the caller (identified by their couch cookie) from the session.
func (h *Handlers) Leave(_ context.Context, in *participantInput) (*clearCookieOutput, error) {
	if in.Cookie != "" {
		h.hub.leaveByToken(in.Cookie)
	}
	return &clearCookieOutput{SetCookie: clearedCouchCookie(h.hub.secure)}, nil
}

// End terminates the session; only the host's cookie may do so.
func (h *Handlers) End(_ context.Context, in *participantInput) (*clearCookieOutput, error) {
	if in.Cookie == "" || !h.hub.endByHostToken(in.Cookie) {
		return nil, httpx.Fail(http.StatusForbidden, "not_host", "only the host can end the session")
	}
	return &clearCookieOutput{SetCookie: clearedCouchCookie(h.hub.secure)}, nil
}

// CouchPlayback is a follower's player for the session's current media.
type CouchPlayback struct {
	Media  mediaRef               `json:"media"`
	Player *playback.PlaybackInfo `json:"player,omitempty" doc:"Absent while the host is choosing what to watch."`
}

type couchPlaybackInput struct {
	Token  string   `path:"token" doc:"The session's share code."`
	Cookie string   `cookie:"couchverse_couch" doc:"The participant cookie set by createCouch or joinCouch."`
	Caps   []string `query:"caps" doc:"Video codecs the client decodes beyond the h264/vp9/av1 baseline (e.g. hevc), for the direct-play decision."`
}

type couchPlaybackOutput struct{ Body CouchPlayback }

func errNoCouchSession() error {
	return httpx.Fail(http.StatusUnauthorized, "no_couch_session", "join the couch session first")
}

// Playback returns the follower player payload for the session's current media,
// authorized by the couch cookie. This is how a follower (incl. anonymous) gets
// its stream URLs without ever calling the auth-only /playback endpoint.
func (h *Handlers) Playback(ctx context.Context, in *couchPlaybackInput) (*couchPlaybackOutput, error) {
	rm, _, ok := h.hub.lookup(in.Cookie)
	if !ok {
		return nil, errNoCouchSession()
	}
	rm.mu.Lock()
	ref := rm.state.Media
	live := rm.live
	rm.lastActive = time.Now()
	rm.mu.Unlock()
	if !live {
		return nil, httpx.Fail(http.StatusGone, "session_ended", "this couch session has ended")
	}
	resp := CouchPlayback{Media: ref}
	if ref.Kind != "" {
		info, err := h.hub.deps.Playback.BuildPlayback(ctx, ref.Kind, ref.playbackID(), nil, in.Caps)
		if err != nil {
			return nil, err
		}
		resp.Player = info
	}
	return &couchPlaybackOutput{Body: resp}, nil
}
