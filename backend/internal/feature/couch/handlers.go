package couch

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/playback"
	"couchverse/internal/httpx"
)

type Handlers struct {
	hub      *Hub
	joinRate *rateLimiter
}

// Create starts (or reclaims) a couch session for the logged-in host watching a
// movie/episode and sets the host's couch cookie.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "log in to host a couch session")
		return
	}
	var req struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	ref := mediaRef{Kind: req.Kind}
	switch req.Kind {
	case "movie":
		ref.TitleID = req.ID
	case "episode":
		ref.EpisodeID = req.ID
	default:
		httpx.BadRequest(w, "kind must be movie or episode")
		return
	}
	if req.ID == "" {
		httpx.BadRequest(w, "missing media id")
		return
	}
	rm, host, token, created, err := h.hub.createOrReclaim(r.Context(), user, ref)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if created && h.hub.deps.Stats != nil {
		// best effort - the counter must never fail hosting a session
		if err := h.hub.deps.Stats.RecordCouchHosted(r.Context(), user.ID); err != nil {
			slog.Warn("record couch hosted", "err", err)
		}
	}
	setCouchCookie(w, token, h.hub.secure)
	httpx.JSON(w, http.StatusCreated, rm.snapshotFor(host.ID, "host"))
}

// Info previews a session (host, what's playing, participant count) without
// joining, for the pre-join "Start watching" screen. Public.
func (h *Handlers) Info(w http.ResponseWriter, r *http.Request) {
	rm := h.hub.roomByShare(chi.URLParam(r, "token"))
	if rm == nil {
		httpx.Error(w, http.StatusNotFound, "no_session", "this couch session does not exist or has ended")
		return
	}
	info, ref := rm.infoPreview()
	if ref.Kind != "" {
		if pi, err := h.hub.deps.Playback.BuildPlayback(r.Context(), ref.Kind, ref.playbackID(), nil, nil); err == nil {
			info.Display = &couchInfoDisplay{
				Title:          pi.Display.Title,
				Subtitle:       pi.Display.Subtitle,
				BackdropID:     pi.Display.BackdropID,
				BackdropAccent: pi.Display.BackdropAccent,
			}
		}
	}
	httpx.JSON(w, http.StatusOK, info)
}

// Join adds the caller (logged-in or anonymous) to a session via its share token
// and sets their couch cookie.
func (h *Handlers) Join(w http.ResponseWriter, r *http.Request) {
	if !h.joinRate.allow(r.RemoteAddr) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many join attempts, slow down")
		return
	}
	rm := h.hub.roomByShare(chi.URLParam(r, "token"))
	if rm == nil {
		httpx.Error(w, http.StatusNotFound, "no_session", "this couch session does not exist or has ended")
		return
	}
	user := auth.UserFrom(r.Context())
	p, token, role, err := h.hub.join(rm, user)
	if err != nil {
		switch {
		case errors.Is(err, errRoomFull):
			httpx.Error(w, http.StatusConflict, "session_full", "this couch session is full")
		case errors.Is(err, errNotLive):
			httpx.Error(w, http.StatusNotFound, "no_session", "this couch session has ended")
		default:
			httpx.Internal(w, err)
		}
		return
	}
	// only a logged-in follower counts; the host reclaiming their own link does not
	if user != nil && role == "follower" && h.hub.deps.Stats != nil {
		if err := h.hub.deps.Stats.RecordCouchJoined(r.Context(), user.ID); err != nil {
			slog.Warn("record couch joined", "err", err)
		}
	}
	setCouchCookie(w, token, h.hub.secure)
	httpx.JSON(w, http.StatusOK, rm.snapshotFor(p.ID, role))
}

// Leave drops the caller (identified by their couch cookie) from the session.
func (h *Handlers) Leave(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CouchCookie); err == nil {
		h.hub.leaveByToken(c.Value)
	}
	clearCouchCookie(w, h.hub.secure)
	httpx.JSON(w, http.StatusNoContent, nil)
}

// End terminates the session; only the host's cookie may do so.
func (h *Handlers) End(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CouchCookie)
	if err != nil || !h.hub.endByHostToken(c.Value) {
		httpx.Error(w, http.StatusForbidden, "not_host", "only the host can end the session")
		return
	}
	clearCouchCookie(w, h.hub.secure)
	httpx.JSON(w, http.StatusNoContent, nil)
}

type couchPlaybackResp struct {
	Media  mediaRef               `json:"media"`
	Player *playback.PlaybackInfo `json:"player"` // nil while the host is choosing
}

// Playback returns the follower player payload for the session's current media,
// authorized by the couch cookie. This is how a follower (incl. anonymous) gets
// its stream URLs without ever calling the auth-only /playback endpoint.
func (h *Handlers) Playback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CouchCookie)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "no_couch_session", "join the couch session first")
		return
	}
	rm, _, ok := h.hub.lookup(c.Value)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no_couch_session", "join the couch session first")
		return
	}
	rm.mu.Lock()
	ref := rm.state.Media
	live := rm.live
	rm.lastActive = time.Now()
	rm.mu.Unlock()
	if !live {
		httpx.Error(w, http.StatusGone, "session_ended", "this couch session has ended")
		return
	}
	resp := couchPlaybackResp{Media: ref}
	if ref.Kind != "" {
		caps := strings.Split(r.URL.Query().Get("caps"), ",")
		info, berr := h.hub.deps.Playback.BuildPlayback(r.Context(), ref.Kind, ref.playbackID(), nil, caps)
		if berr != nil {
			httpx.StoreErr(w, berr)
			return
		}
		resp.Player = info
	}
	httpx.JSON(w, http.StatusOK, resp)
}
