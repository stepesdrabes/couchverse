package couchtv

import (
	"cmp"
	"context"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"couchverse/internal/feature/auth"
)

// The Hub is the channel's room (plan 6.9): who is watching, their reactions and
// their reports of the program date on screen, which become progress, watch time
// and the facts ranks will score. It lives in memory and has no host; a restart
// drops the sockets, which reconnect.

const (
	// presenceListed bounds the viewers a presence frame names; the count is everyone.
	presenceListed = 24
	// accrueEvery is how often reports become progress, watch time and stats: off
	// the socket's read path, where a database round trip would stall reads.
	accrueEvery = 20 * time.Second
	// reportGap is the longest silence between two reports that still counts as watching.
	reportGap = 30 * time.Second
	// progressAfter is how long a signed-in viewer watches a program before it
	// shows in their Continue watching.
	progressAfter = 2 * time.Minute
	// finishedWithin counts a program watched to within this of where it left the
	// air as finished.
	finishedWithin = 30 * time.Second
	// progressKeep forgets what a viewer watched once they stopped for this long.
	progressKeep = 10 * time.Minute
	// a viewer's reactions: at most reactionBurst within reactionWindow
	reactionBurst  = 3
	reactionWindow = time.Second
	// scheduleSettle gathers the changes of one plan run into one schedule_changed.
	scheduleSettle = 2 * time.Second
)

// TvWatchRecorder keeps the channel's watch time (satisfied by *analytics.Store):
// userID 0 for a guest, titleID "" for a break or a filler. Best effort; nil
// disables it.
type TvWatchRecorder interface {
	RecordTvWatch(ctx context.Context, userID int64, titleID string, seconds int) error
}

// ProgressRecorder saves where a signed-in viewer got to in a program, so their
// Continue watching picks it up (satisfied by *catalog.Store). Nil disables it.
type ProgressRecorder interface {
	UpsertProgress(ctx context.Context, userID int64, titleID, episodeID *string, position, duration int, at *time.Time) error
}

// TvStatsRecorder takes the per-user facts ranks will score the channel by (plan
// 6.9: XP for TV minutes, the time-of-day, social and loyalty achievements).
// internal/app hands it *ranks.Store once ranks implements it, so couchtv never
// imports ranks. Best effort; nil disables it.
type TvStatsRecorder interface {
	// RecordTvViewing is a stretch a signed-in viewer watched of one airing.
	RecordTvViewing(ctx context.Context, v TvViewing) error
	// RecordTvReactions counts the reactions a signed-in viewer sent.
	RecordTvReactions(ctx context.Context, userID int64, count int) error
	// RecordTvAudience is the most viewers a signed-in viewer watched with,
	// each time it grows.
	RecordTvAudience(ctx context.Context, userID int64, viewers int) error
}

// TvViewing is a stretch a signed-in viewer watched of one part of an airing.
type TvViewing struct {
	UserID   int64
	AiringID string
	// Kind is the part's: a program, a break, or a filler (standby, slate).
	Kind string
	// Fill is the rule that chose a program (strip, premiere, saga...); empty for
	// the rest.
	Fill    string
	TitleID string
	// Start is the program date the stretch began at: its time of day is the
	// picture's.
	Start   time.Time
	Seconds int
}

// lineupReader is what the reports are mapped onto (*Channel).
type lineupReader interface {
	Lineup(ctx context.Context, from, to time.Time) ([]LineupPart, error)
}

// durationReader is how long the programs' files are (*Store).
type durationReader interface {
	FileDurations(ctx context.Context, ids []string) (map[string]time.Duration, error)
}

// HubDeps are the Hub's collaborators.
type HubDeps struct {
	Lineup    lineupReader
	Durations durationReader
	Watch     TvWatchRecorder
	Progress  ProgressRecorder
	Stats     TvStatsRecorder
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time
}

type Hub struct {
	deps HubDeps
	ctx  context.Context

	mu      sync.Mutex
	viewers map[string]*viewer
	joined  uint64
	changed *time.Timer

	// progress is what each signed-in viewer is watching; only accrue touches it
	progress map[string]*watching
}

// viewer is one person watching: a signed-in user on any number of devices, or a
// guest. It outlives its last socket until the next accrual has counted its
// reports.
type viewer struct {
	CouchTvViewer
	userID    int64 // 0 for a guest
	order     uint64
	conns     map[*conn]struct{}
	spans     []stretch // program dates watched since the last accrual
	reactions []time.Time
	reacted   int // reactions since the last accrual
	audience  int // the most viewers seen while connected
}

// stretch is a run of program dates on someone's screen.
type stretch struct{ from, to time.Time }

// watching is the program a signed-in viewer watches, for the progress rule.
type watching struct {
	airing  string
	watched time.Duration
	seen    time.Time
}

// NewHub runs the room until ctx ends.
func NewHub(ctx context.Context, deps HubDeps) *Hub {
	h := newHub(ctx, deps)
	go h.accrueLoop()
	return h
}

// newHub is a room whose accruals are left to the caller (tests).
func newHub(ctx context.Context, deps HubDeps) *Hub {
	return &Hub{deps: deps, ctx: ctx, viewers: map[string]*viewer{}, progress: map[string]*watching{}}
}

func (h *Hub) now() time.Time {
	if h.deps.Now != nil {
		return h.deps.Now()
	}
	return time.Now()
}

// userViewer is a signed-in viewer as everyone sees them.
func userViewer(u *auth.User) CouchTvViewer {
	return CouchTvViewer{ID: "user-" + strconv.FormatInt(u.ID, 10), DisplayName: u.DisplayName, AvatarID: u.AvatarID, Seed: u.Username}
}

// guestViewer is a guest as everyone sees them.
func guestViewer(g Guest) CouchTvViewer {
	return CouchTvViewer{ID: "guest-" + g.ID, DisplayName: g.Name, Seed: g.ID, Guest: true}
}

// attach adds a socket of a viewer: the socket hears hello first, everyone else
// the new presence when someone new tuned in.
func (h *Hub) attach(c *conn, me CouchTvViewer, userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := h.viewers[me.ID]
	if v == nil {
		h.joined++
		v = &viewer{order: h.joined, conns: map[*conn]struct{}{}}
		h.viewers[me.ID] = v
	}
	// a name or an avatar changed since the viewer's last socket
	v.CouchTvViewer, v.userID = me, userID
	arrived := len(v.conns) == 0
	v.conns[c] = struct{}{}
	c.viewer = v
	presence := h.presenceLocked()
	c.enqueue(envelope(msgHello, CouchTvHello{ServerTime: h.now().UnixMilli(), Me: me, Presence: presence}))
	if arrived {
		h.broadcastLocked(envelope(msgPresence, presence), c)
	}
}

// detach removes a socket; the viewer leaves the presence with their last one.
func (h *Hub) detach(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := c.viewer
	delete(v.conns, c)
	if len(v.conns) == 0 {
		h.broadcastLocked(envelope(msgPresence, h.presenceLocked()), nil)
	}
}

// presenceLocked is who is watching, in the order they tuned in. Caller holds h.mu.
func (h *Hub) presenceLocked() CouchTvPresence {
	var here []*viewer
	for _, v := range h.viewers {
		if len(v.conns) > 0 {
			here = append(here, v)
		}
	}
	slices.SortFunc(here, func(a, b *viewer) int { return cmp.Compare(a.order, b.order) })
	out := CouchTvPresence{Count: len(here), Viewers: []CouchTvViewer{}}
	for _, v := range here[:min(len(here), presenceListed)] {
		out.Viewers = append(out.Viewers, v.CouchTvViewer)
	}
	return out
}

// broadcastLocked queues a frame for every socket but except. Caller holds h.mu;
// queueing never blocks.
func (h *Hub) broadcastLocked(frame []byte, except *conn) {
	for _, v := range h.viewers {
		for c := range v.conns {
			if c != except {
				c.enqueue(frame)
			}
		}
	}
}

// react relays a viewer's reaction to everyone, unless they send too many.
func (h *Hub) react(c *conn, emoji string) {
	emoji = sanitizeEmoji(emoji)
	if emoji == "" {
		return
	}
	now := h.now()
	h.mu.Lock()
	defer h.mu.Unlock()
	v := c.viewer
	v.reactions = recent(v.reactions, now, reactionWindow)
	if len(v.reactions) >= reactionBurst {
		return
	}
	v.reactions = append(v.reactions, now)
	v.reacted++
	h.broadcastLocked(envelope(msgReaction, CouchTvReaction{ViewerID: v.ID, Emoji: emoji}), nil)
}

// report takes a socket's report of the program date on screen: the stretch since
// its previous report counts as watched when the picture moved on about as much
// as the clock did. A report from the future or long gone is dropped.
func (h *Hub) report(c *conn, programDate int64) {
	now := h.now()
	pd := time.UnixMilli(programDate)
	if pd.After(now.Add(time.Minute)) || pd.Before(now.Add(-airingReach)) {
		return
	}
	last := c.last
	c.last = &reported{programDate: pd, at: now}
	if last == nil {
		return
	}
	elapsed, moved := now.Sub(last.at), pd.Sub(last.programDate)
	if elapsed > reportGap || moved <= 0 || moved > elapsed*3/2+time.Second {
		return
	}
	h.mu.Lock()
	c.viewer.spans = append(c.viewer.spans, stretch{from: last.programDate, to: pd})
	h.mu.Unlock()
}

// ScheduleChanged tells everyone watching that the schedule changed, once for a
// burst of changes.
func (h *Hub) ScheduleChanged() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.changed != nil {
		return
	}
	h.changed = time.AfterFunc(scheduleSettle, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.changed = nil
		h.broadcastLocked(envelope(msgScheduleChanged, nil), nil)
	})
}

// EndGuests closes every guest's sockets: the channel link was turned off or
// rotated.
func (h *Hub) EndGuests() {
	h.mu.Lock()
	var conns []*conn
	for _, v := range h.viewers {
		if v.Guest {
			for c := range v.conns {
				conns = append(conns, c)
			}
		}
	}
	h.mu.Unlock()
	for _, c := range conns {
		go c.ws.Close(websocket.StatusPolicyViolation, "the channel link ended")
	}
}

// Viewers is how many people are watching now.
func (h *Hub) Viewers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.presenceLocked().Count
}

func (h *Hub) accrueLoop() {
	t := time.NewTicker(accrueEvery)
	defer t.Stop()
	for {
		select {
		case <-h.ctx.Done():
			// what was reported since the last tick still counts
			h.accrue(context.Background())
			return
		case <-t.C:
			h.accrue(h.ctx)
		}
	}
}

// batch is what one viewer did since the last accrual.
type batch struct {
	viewer   CouchTvViewer
	userID   int64
	spans    []stretch
	reacted  int
	audience int // the new most viewers they watched with; 0 when it did not grow
}

// accrue turns what everyone did since the last tick into watch time, progress
// and stats: it takes the reports under the lock and does its I/O outside it.
func (h *Hub) accrue(ctx context.Context) {
	now := h.now()
	var batches []batch
	h.mu.Lock()
	here := h.presenceLocked().Count
	for id, v := range h.viewers {
		b := batch{viewer: v.CouchTvViewer, userID: v.userID, spans: v.spans, reacted: v.reacted}
		v.spans, v.reacted = nil, 0
		if len(v.conns) > 0 && here > v.audience {
			v.audience = here
			b.audience = here
		}
		if len(v.conns) == 0 {
			delete(h.viewers, id)
		}
		if len(b.spans) > 0 || b.reacted > 0 || b.audience > 0 {
			batches = append(batches, b)
		}
	}
	h.mu.Unlock()

	h.count(ctx, batches, now)
	if stats := h.deps.Stats; stats != nil {
		for _, b := range batches {
			if b.userID == 0 {
				continue
			}
			if b.reacted > 0 {
				logFailure("reactions", stats.RecordTvReactions(ctx, b.userID, b.reacted))
			}
			if b.audience > 0 {
				logFailure("audience", stats.RecordTvAudience(ctx, b.userID, b.audience))
			}
		}
	}
	for id, w := range h.progress {
		if now.Sub(w.seen) > progressKeep {
			delete(h.progress, id)
		}
	}
}

// watchKey is a row of the channel's watch time.
type watchKey struct {
	userID  int64
	titleID string
}

// saving is a progress save: where a signed-in viewer got to in a program.
type saving struct {
	userID int64
	part   LineupPart
	at     time.Duration // into the program
}

// count maps the stretches watched onto what aired and records them.
func (h *Hub) count(ctx context.Context, batches []batch, now time.Time) {
	var from, to time.Time
	for i := range batches {
		batches[i].spans = merge(batches[i].spans)
		for _, s := range batches[i].spans {
			if from.IsZero() || s.from.Before(from) {
				from = s.from
			}
			if s.to.After(to) {
				to = s.to
			}
		}
	}
	if from.IsZero() || h.deps.Lineup == nil {
		return
	}
	parts, err := h.deps.Lineup.Lineup(ctx, from, to)
	if err != nil {
		slog.Warn("couchtv: the reports could not be counted", "err", err)
		return
	}
	watched := map[watchKey]time.Duration{}
	var saves []saving
	var viewings []TvViewing
	for _, b := range batches {
		var save *saving
		for _, s := range b.spans {
			for _, p := range parts {
				start, end := s.from, s.to
				if p.Start.After(start) {
					start = p.Start
				}
				if p.End.Before(end) {
					end = p.End
				}
				if !end.After(start) {
					continue
				}
				d := end.Sub(start)
				key := watchKey{userID: b.userID}
				if p.Kind == PartProgram {
					key.titleID = p.TitleID
				}
				watched[key] += d
				if b.userID == 0 {
					continue
				}
				fill := ""
				if p.Kind == PartProgram {
					fill = p.Fill
				}
				viewings = append(viewings, TvViewing{UserID: b.userID, AiringID: p.AiringID, Kind: p.Kind, Fill: fill,
					TitleID: key.titleID, Start: start, Seconds: seconds(d)})
				if p.Kind != PartProgram {
					continue
				}
				w := h.progress[b.viewer.ID]
				if w == nil || w.airing != p.AiringID {
					w = &watching{airing: p.AiringID}
					h.progress[b.viewer.ID] = w
				}
				w.watched += d
				w.seen = now
				if w.watched >= progressAfter {
					save = &saving{userID: b.userID, part: p, at: end.Sub(p.Start)}
				}
			}
		}
		if save != nil {
			saves = append(saves, *save)
		}
	}

	if rec := h.deps.Watch; rec != nil {
		for key, d := range watched {
			if n := seconds(d); n > 0 {
				logFailure("watch time", rec.RecordTvWatch(ctx, key.userID, key.titleID, n))
			}
		}
	}
	if stats := h.deps.Stats; stats != nil {
		for _, v := range viewings {
			if v.Seconds > 0 {
				logFailure("viewing", stats.RecordTvViewing(ctx, v))
			}
		}
	}
	h.saveProgress(ctx, saves)
}

// saveProgress saves where signed-in viewers got to; a program watched to where it
// left the air counts as finished.
func (h *Hub) saveProgress(ctx context.Context, saves []saving) {
	if len(saves) == 0 || h.deps.Progress == nil || h.deps.Durations == nil {
		return
	}
	var files []string
	for _, s := range saves {
		files = append(files, s.part.File)
	}
	durations, err := h.deps.Durations.FileDurations(ctx, files)
	if err != nil {
		slog.Warn("couchtv: progress could not be saved", "err", err)
		return
	}
	for _, s := range saves {
		length := durations[s.part.File]
		if length <= 0 {
			continue
		}
		position := s.at
		if s.part.End.Sub(s.part.Start)-s.at <= finishedWithin {
			position = length
		}
		var titleID, episodeID *string
		if s.part.EpisodeID != "" {
			episodeID = &s.part.EpisodeID
		} else {
			titleID = &s.part.TitleID
		}
		logFailure("progress", h.deps.Progress.UpsertProgress(ctx, s.userID, titleID, episodeID,
			seconds(min(position, length)), seconds(length), nil))
	}
}

// merge sorts stretches and joins the ones that overlap, so two devices showing
// the same picture count once.
func merge(spans []stretch) []stretch {
	slices.SortFunc(spans, func(a, b stretch) int { return a.from.Compare(b.from) })
	var out []stretch
	for _, s := range spans {
		if n := len(out); n > 0 && !s.from.After(out[n-1].to) {
			if s.to.After(out[n-1].to) {
				out[n-1].to = s.to
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

func seconds(d time.Duration) int {
	return int(math.Round(d.Seconds()))
}

func logFailure(what string, err error) {
	if err != nil {
		slog.Warn("couchtv: a viewer's "+what+" was not recorded", "err", err)
	}
}

// sanitizeEmoji bounds a reaction to a short unicode string (the picker yields a
// single grapheme; the full set cannot be allow-listed).
func sanitizeEmoji(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 32 || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 8 {
		return ""
	}
	return s
}
