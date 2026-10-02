package system

import "context"

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

type LiveStats struct {
	Streams       int `json:"streams" doc:"Players that reported progress in the last minute."`
	CouchSessions int `json:"couchSessions"`
	CouchViewers  int `json:"couchViewers"`
	Transcodes    int `json:"transcodes" doc:"Active instant-play transcode sessions."`
}

type liveOutput struct{ Body LiveStats }

func (h *Live) Get(ctx context.Context, _ *struct{}) (*liveOutput, error) {
	streams, err := h.store.ActiveStreamCount(ctx)
	if err != nil {
		return nil, err
	}
	sessions, viewers := h.couch.LivePresence()
	return &liveOutput{Body: LiveStats{
		Streams:       streams,
		CouchSessions: sessions,
		CouchViewers:  viewers,
		Transcodes:    h.transcodes.ActiveCount(),
	}}, nil
}
