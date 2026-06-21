package system

import (
	"net/http"

	"couchverse/internal/httpx"
)

// CouchPresence and TranscodePresence are the live-session signals the dashboard
// shows. They are satisfied by couch.Hub and playback.SessionManager; declaring
// them here keeps the system feature from importing those packages.
type CouchPresence interface {
	LivePresence() (sessions, viewers int)
}

type TranscodePresence interface {
	ActiveCount() int
}

// Live serves real-time presence: current streams, live couch sessions plus the
// people on them, and active instant-play transcodes.
type Live struct {
	store      *Store
	couch      CouchPresence
	transcodes TranscodePresence
}

func NewLive(st *Store, couch CouchPresence, transcodes TranscodePresence) *Live {
	return &Live{store: st, couch: couch, transcodes: transcodes}
}

func (h *Live) Get(w http.ResponseWriter, r *http.Request) {
	streams, err := h.store.ActiveStreamCount(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	sessions, viewers := h.couch.LivePresence()
	httpx.JSON(w, http.StatusOK, map[string]any{
		"streams":       streams,
		"couchSessions": sessions,
		"couchViewers":  viewers,
		"transcodes":    h.transcodes.ActiveCount(),
	})
}
