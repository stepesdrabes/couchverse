package couch

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/playback"
	"couchverse/internal/flags"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// The Hub is the in-memory registry of live couch sessions. Nothing is
// persisted: a process restart ends every session and invalidates every couch
// cookie (the token index is gone). Lock order is always Hub.mu -> room.mu.

const (
	defaultMaxParticipants = 20
	defaultHostGrace       = 60 * time.Second
	roomIdleTTL            = 30 * time.Minute
	followerGrace          = 2 * time.Minute
	couchWatchInterval     = 20 * time.Second // accrual cadence for on-couch watch-time
)

var (
	errRoomFull = errors.New("couch: session is full")
	errNotLive  = errors.New("couch: session is no longer live")
)

// MediaResolver resolves the current media to its playable file set (satisfied
// by *catalog.Store). PlaybackBuilder builds a follower's player payload
// (satisfied by *playback.Stream). Narrow interfaces keep the Hub testable.
type MediaResolver interface {
	PrimaryMediaFileForTitle(ctx context.Context, titleID string) (*media.MediaFile, error)
	PrimaryMediaFileForEpisode(ctx context.Context, episodeID string) (*media.MediaFile, error)
	AudioSiblings(ctx context.Context, titleID, episodeID *string, excludeID string) ([]media.MediaFile, error)
}

type PlaybackBuilder interface {
	BuildPlayback(ctx context.Context, kind, id string, userID *int64, caps []string) (*playback.PlaybackInfo, error)
}

// CouchWatchRecorder persists the separate on-couch watch-time stat (satisfied
// by *analytics.Store). Best-effort; nil disables accrual (e.g. in tests).
type CouchWatchRecorder interface {
	RecordCouchWatch(ctx context.Context, titleID string, seconds int) error
}

// Deps are the collaborators the Hub needs. Settings backs the couchEnabled flag.
type Deps struct {
	Media     MediaResolver
	Playback  PlaybackBuilder
	Analytics CouchWatchRecorder
	Settings  *settings.Store
	Secure    bool
}

type Hub struct {
	deps            Deps
	secure          bool
	appCtx          context.Context
	epoch           time.Time
	maxParticipants int
	hostGrace       time.Duration

	mu      sync.RWMutex
	rooms   map[string]*room           // sessionID  -> room
	byShare map[string]*room           // shareToken -> room
	byHost  map[int64]*room            // host userID -> room (reclaim, one per user)
	byToken map[string]*participantRef // cookie token hash -> {room, participant}
}

type participantRef struct {
	room *room
	pid  string
}

func NewHub(appCtx context.Context, deps Deps) *Hub {
	h := &Hub{
		deps:            deps,
		secure:          deps.Secure,
		appCtx:          appCtx,
		epoch:           time.Now(),
		maxParticipants: defaultMaxParticipants,
		hostGrace:       defaultHostGrace,
		rooms:           map[string]*room{},
		byShare:         map[string]*room{},
		byHost:          map[int64]*room{},
		byToken:         map[string]*participantRef{},
	}
	go h.reapLoop(appCtx)
	if deps.Analytics != nil {
		go h.accrualLoop(appCtx)
	}
	return h
}

// accrualLoop periodically tallies follower watch-time per title (the separate
// on-couch stat). Best-effort: a failed record is dropped, never blocks a room.
func (h *Hub) accrualLoop(ctx context.Context) {
	t := time.NewTicker(couchWatchInterval)
	defer t.Stop()
	secs := int(couchWatchInterval.Seconds())
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.accrueWatch(ctx, secs)
		}
	}
}

func (h *Hub) accrueWatch(ctx context.Context, intervalSecs int) {
	type entry struct {
		titleID string
		seconds int
	}
	var entries []entry
	h.mu.RLock()
	for _, rm := range h.rooms {
		rm.mu.Lock()
		if rm.live && rm.state.Playing && !rm.state.Away && rm.titleID != "" {
			followers := 0
			for _, p := range rm.participants {
				if !p.IsHost && p.connCount > 0 {
					followers++
				}
			}
			if followers > 0 {
				entries = append(entries, entry{rm.titleID, intervalSecs * followers})
			}
		}
		rm.mu.Unlock()
	}
	h.mu.RUnlock()

	for _, e := range entries {
		_ = h.deps.Analytics.RecordCouchWatch(ctx, e.titleID, e.seconds)
	}
}

// LivePresence reports how many couch sessions are live right now and how many
// people are connected to them (host + followers with an open socket, anonymous
// included). Feeds the admin dashboard's live stats.
func (h *Hub) LivePresence() (sessions, viewers int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, rm := range h.rooms {
		rm.mu.Lock()
		if rm.live {
			sessions++
			for _, p := range rm.participants {
				if p.connCount > 0 {
					viewers++
				}
			}
		}
		rm.mu.Unlock()
	}
	return sessions, viewers
}

// nowMs is a monotonic millisecond clock anchored at process start. Relayed
// verbatim so all clients share one timeline and sidestep wall-clock skew.
func (h *Hub) nowMs() int64 { return time.Since(h.epoch).Milliseconds() }

// mediaRef identifies what the host is watching. An empty Kind means the host
// is on the browse screen ("choosing what to watch").
type mediaRef struct {
	Kind      string `json:"kind"`
	TitleID   string `json:"titleId,omitempty"`
	EpisodeID string `json:"episodeId,omitempty"`
}

func (m mediaRef) playbackID() string {
	if m.Kind == "episode" {
		return m.EpisodeID
	}
	return m.TitleID
}

// hostState is the single authoritative play-state, set by the host and relayed
// to followers. serverTimestamp/seq are stamped by the Hub so followers can
// drop stale frames and extrapolate position without trusting client clocks.
type hostState struct {
	Media             mediaRef `json:"media"`
	Playing           bool     `json:"playing"`
	PositionSeconds   float64  `json:"positionSeconds"`
	ServerTimestampMs int64    `json:"serverTimestamp"`
	Seq               uint64   `json:"seq"`
	Away              bool     `json:"away"`
}

type room struct {
	hub        *Hub
	sessionID  string
	shareToken string
	hostUserID int64

	mu              sync.Mutex
	live            bool
	lastActive      time.Time
	state           hostState
	titleID         string // series/movie title id of the current media (for watch-time)
	participants    map[string]*participant
	conns           map[*conn]struct{}
	allowedMediaIDs map[string]struct{}
	graceTimer      *time.Timer
}

func (rm *room) hostParticipantLocked() *participant {
	for _, p := range rm.participants {
		if p.IsHost {
			return p
		}
	}
	return nil
}

func (rm *room) hostConnsLocked() int {
	if p := rm.hostParticipantLocked(); p != nil {
		return p.connCount
	}
	return 0
}

func (rm *room) isHostParticipant(pid string) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	p := rm.participants[pid]
	return p != nil && p.IsHost
}

// resolveAllowed returns the media file ids a follower may stream for ref (the
// primary playable file plus its model-B audio siblings, so switching audio
// language stays authorized) and the title id the media belongs to (for the
// on-couch watch-time stat). Empty ref => empty set (host is choosing).
func (h *Hub) resolveAllowed(ctx context.Context, ref mediaRef) (map[string]struct{}, string, error) {
	set := map[string]struct{}{}
	var (
		primary *media.MediaFile
		err     error
	)
	switch ref.Kind {
	case "movie":
		primary, err = h.deps.Media.PrimaryMediaFileForTitle(ctx, ref.TitleID)
	case "episode":
		primary, err = h.deps.Media.PrimaryMediaFileForEpisode(ctx, ref.EpisodeID)
	default:
		return set, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	set[primary.ID] = struct{}{}
	if sibs, serr := h.deps.Media.AudioSiblings(ctx, primary.TitleID, primary.EpisodeID, primary.ID); serr == nil {
		for i := range sibs {
			set[sibs[i].ID] = struct{}{}
		}
	}
	titleID := ""
	if primary.TitleID != nil {
		titleID = *primary.TitleID
	}
	return set, titleID, nil
}

// createOrReclaim starts a session for the host, or returns and refreshes the
// host's existing live session (a refresh / second tab reclaims, never spawns a
// duplicate). DB resolution happens before the lock.
func (h *Hub) createOrReclaim(ctx context.Context, user *auth.User, ref mediaRef) (*room, *participant, string, error) {
	allowed, titleID, err := h.resolveAllowed(ctx, ref)
	if err != nil {
		return nil, nil, "", err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if rm := h.byHost[user.ID]; rm != nil {
		rm.mu.Lock()
		if rm.live {
			rm.state.Media = ref
			rm.allowedMediaIDs = allowed
			rm.titleID = titleID
			rm.lastActive = time.Now()
			host := rm.hostParticipantLocked()
			rm.mu.Unlock()
			token := h.issueTokenLocked(rm, host)
			return rm, host, token, nil
		}
		rm.mu.Unlock()
	}

	host := newParticipant(user, true)
	rm := &room{
		hub:             h,
		sessionID:       uuid.NewString(),
		shareToken:      h.freeShareCodeLocked(),
		hostUserID:      user.ID,
		live:            true,
		lastActive:      time.Now(),
		state:           hostState{Media: ref},
		titleID:         titleID,
		participants:    map[string]*participant{host.ID: host},
		conns:           map[*conn]struct{}{},
		allowedMediaIDs: allowed,
	}
	token := h.issueTokenLocked(rm, host)
	h.rooms[rm.sessionID] = rm
	h.byShare[rm.shareToken] = rm
	h.byHost[user.ID] = rm
	return rm, host, token, nil
}

// freeShareCodeLocked returns a 6-digit code not currently in use. Caller holds
// h.mu. Collisions are near-impossible with the handful of live sessions a
// self-hosted instance has, so a simple retry loop suffices.
func (h *Hub) freeShareCodeLocked() string {
	for {
		code := randDigits(6)
		if _, exists := h.byShare[code]; !exists {
			return code
		}
	}
}

// issueTokenLocked mints a fresh cookie token for a participant, replacing any
// previous one in the index. Caller holds h.mu.
func (h *Hub) issueTokenLocked(rm *room, p *participant) string {
	token, hash := newToken()
	if p.tokenHash != "" {
		delete(h.byToken, p.tokenHash)
	}
	p.tokenHash = hash
	h.byToken[hash] = &participantRef{room: rm, pid: p.ID}
	return token
}

// join adds a participant (logged-in or anonymous) to a session. A logged-in
// host opening their own share link reclaims the host seat.
func (h *Hub) join(rm *room, user *auth.User) (*participant, string, string, error) {
	h.mu.Lock()
	rm.mu.Lock()
	if !rm.live {
		rm.mu.Unlock()
		h.mu.Unlock()
		return nil, "", "", errNotLive
	}
	if user != nil && user.ID == rm.hostUserID {
		host := rm.hostParticipantLocked()
		rm.lastActive = time.Now()
		rm.mu.Unlock()
		token := h.issueTokenLocked(rm, host)
		h.mu.Unlock()
		return host, token, "host", nil
	}
	if len(rm.participants) >= h.maxParticipants {
		rm.mu.Unlock()
		h.mu.Unlock()
		return nil, "", "", errRoomFull
	}
	p := newParticipant(user, false)
	rm.participants[p.ID] = p
	rm.lastActive = time.Now()
	rm.mu.Unlock()
	token := h.issueTokenLocked(rm, p)
	h.mu.Unlock()

	rm.broadcastParticipants()
	return p, token, "follower", nil
}

// leaveByToken removes the participant identified by a cookie token. If the
// host leaves or the room empties, the session ends.
func (h *Hub) leaveByToken(rawToken string) {
	h.mu.Lock()
	hash := hashToken(rawToken)
	ref := h.byToken[hash]
	if ref == nil {
		h.mu.Unlock()
		return
	}
	delete(h.byToken, hash)
	rm := ref.room
	rm.mu.Lock()
	p := rm.participants[ref.pid]
	isHost := p != nil && p.IsHost
	delete(rm.participants, ref.pid)
	empty := len(rm.participants) == 0
	rm.mu.Unlock()
	ended := false
	if isHost || empty {
		h.endRoomLocked(rm, "host_left")
		ended = true
	}
	h.mu.Unlock()
	if !ended {
		rm.broadcastParticipants()
	}
}

// endByHostToken ends a session if the token belongs to its host.
func (h *Hub) endByHostToken(rawToken string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	ref := h.byToken[hashToken(rawToken)]
	if ref == nil {
		return false
	}
	rm := ref.room
	if !rm.isHostParticipant(ref.pid) {
		return false
	}
	h.endRoomLocked(rm, "host_ended")
	return true
}

// expireHost ends a session whose host has not reconnected within the grace
// window. Invoked from the grace timer goroutine.
func (h *Hub) expireHost(rm *room) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rm.mu.Lock()
	stillGone := rm.live && rm.hostConnsLocked() == 0
	rm.mu.Unlock()
	if stillGone {
		h.endRoomLocked(rm, "host_timeout")
	}
}

// startHostGrace arms (or re-arms) the timer that ends the session if the host
// does not reconnect.
func (h *Hub) startHostGrace(rm *room) {
	rm.mu.Lock()
	if rm.graceTimer != nil {
		rm.graceTimer.Stop()
	}
	rm.graceTimer = time.AfterFunc(h.hostGrace, func() { h.expireHost(rm) })
	rm.mu.Unlock()
}

// endRoomLocked marks a room dead, notifies + closes its connections, and purges
// it from every index. Caller holds h.mu. Every cookie that mapped to it now
// dangles, so the stream guard denies.
func (h *Hub) endRoomLocked(rm *room, reason string) {
	rm.mu.Lock()
	if !rm.live {
		rm.mu.Unlock()
		return
	}
	rm.live = false
	if rm.graceTimer != nil {
		rm.graceTimer.Stop()
		rm.graceTimer = nil
	}
	conns := make([]*conn, 0, len(rm.conns))
	for c := range rm.conns {
		conns = append(conns, c)
	}
	rm.mu.Unlock()

	frame := mustEnvelope(msgSessionEnded, sessionEndedData{Reason: reason})
	for _, c := range conns {
		c.enqueue(frame)
		c.beginClose()
	}

	delete(h.rooms, rm.sessionID)
	delete(h.byShare, rm.shareToken)
	if h.byHost[rm.hostUserID] == rm {
		delete(h.byHost, rm.hostUserID)
	}
	for hash, ref := range h.byToken {
		if ref.room == rm {
			delete(h.byToken, hash)
		}
	}
}

// AllowsAnon reports whether an anonymous request may stream mediaFileID: it
// must carry a valid couch cookie whose live session currently allows that file.
// Satisfies the stream-authorization port consumed by internal/server.
func (h *Hub) AllowsAnon(r *http.Request, mediaFileID string) bool {
	if mediaFileID == "" {
		return false
	}
	if !flags.Load(r.Context(), h.deps.Settings).CouchEnabled {
		return false
	}
	c, err := r.Cookie(CouchCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return h.allows(c.Value, mediaFileID)
}

// allows is the cookie-token core of AllowsAnon (no flag/cookie parsing), kept
// separate so the authorization rule can be unit-tested without an HTTP request.
func (h *Hub) allows(rawToken, mediaFileID string) bool {
	h.mu.RLock()
	ref := h.byToken[hashToken(rawToken)]
	h.mu.RUnlock()
	if ref == nil {
		return false
	}
	rm := ref.room
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if !rm.live {
		return false
	}
	_, ok := rm.allowedMediaIDs[mediaFileID]
	return ok
}

// lookup resolves a cookie token to its room + participant id.
func (h *Hub) lookup(rawToken string) (*room, string, bool) {
	if rawToken == "" {
		return nil, "", false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	ref := h.byToken[hashToken(rawToken)]
	if ref == nil {
		return nil, "", false
	}
	return ref.room, ref.pid, true
}

func (h *Hub) roomByShare(shareToken string) *room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byShare[shareToken]
}

// Shutdown ends every session on graceful server stop.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, rm := range h.rooms {
		h.endRoomLocked(rm, "server_shutdown")
	}
}

func (h *Hub) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.Shutdown()
			return
		case <-ticker.C:
			h.reapIdle()
		}
	}
}

// reapIdle ends abandoned sessions and prunes followers whose connections have
// been gone past the grace window (e.g. a closed tab that never sent /leave).
func (h *Hub) reapIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, rm := range h.rooms {
		rm.mu.Lock()
		idle := time.Since(rm.lastActive)
		var removed []string
		for pid, p := range rm.participants {
			if !p.IsHost && p.connCount == 0 && !p.disconnectedAt.IsZero() && time.Since(p.disconnectedAt) > followerGrace {
				delete(rm.participants, pid)
				removed = append(removed, pid)
			}
		}
		rm.mu.Unlock()

		if len(removed) > 0 {
			for hash, ref := range h.byToken {
				for _, pid := range removed {
					if ref.room == rm && ref.pid == pid {
						delete(h.byToken, hash)
					}
				}
			}
			rm.broadcastParticipants()
		}
		if idle > roomIdleTTL {
			h.endRoomLocked(rm, "idle")
		}
	}
}

// snapshot is the session state returned by create/join. Participant unexported
// fields are not serialized.
type snapshot struct {
	SessionID       string        `json:"sessionId"`
	ShareToken      string        `json:"shareToken"`
	MyParticipantID string        `json:"myParticipantId"`
	Role            string        `json:"role"`
	IsAnonymous     bool          `json:"isAnonymous"`
	State           hostState     `json:"state"`
	Participants    []participant `json:"participants"`
}

// couchInfo previews a session for the pre-join screen (no participant created).
type couchInfo struct {
	ShareCode    string            `json:"shareCode"`
	HostName     string            `json:"hostName"`
	HostAvatarID *string           `json:"hostAvatarId"`
	HostSeed     string            `json:"hostSeed"`
	Playing      bool              `json:"playing"`
	Participants int               `json:"participants"`
	Display      *couchInfoDisplay `json:"display"` // nil while the host is choosing
}

type couchInfoDisplay struct {
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle"`
	BackdropID     *string `json:"backdropId"`
	BackdropAccent string  `json:"backdropAccent"`
}

func (rm *room) infoPreview() (couchInfo, mediaRef) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	ci := couchInfo{
		ShareCode:    rm.shareToken,
		Playing:      rm.state.Playing,
		Participants: len(rm.participants),
	}
	if host := rm.hostParticipantLocked(); host != nil {
		ci.HostName = host.DisplayName
		ci.HostAvatarID = host.AvatarID
		ci.HostSeed = host.Seed
	}
	return ci, rm.state.Media
}

func (rm *room) snapshotFor(pid, role string) snapshot {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	parts := make([]participant, 0, len(rm.participants))
	for _, p := range rm.participants {
		parts = append(parts, *p)
	}
	isAnon := false
	if p := rm.participants[pid]; p != nil {
		isAnon = p.IsAnonymous
	}
	return snapshot{
		SessionID:       rm.sessionID,
		ShareToken:      rm.shareToken,
		MyParticipantID: pid,
		Role:            role,
		IsAnonymous:     isAnon,
		State:           rm.state,
		Participants:    parts,
	}
}
