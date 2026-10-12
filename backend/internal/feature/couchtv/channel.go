package couchtv

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"sync"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
)

// masterAhead is how far ahead the multivariant playlist's variant covers what
// airs: a viewer keeps the playlist from tuning in.
const masterAhead = 6 * time.Hour

// LiveAiring is a schedule row as the stream reads it, with its feed and origin
// once it went on air.
type LiveAiring struct {
	Airing
	Origin *hls.LiveOrigin
	Feed   *Feed
}

func (a LiveAiring) pinned() bool {
	return a.Origin != nil && a.Feed != nil
}

// liveStore is what the stream reads and writes of the schedule (*Store).
type liveStore interface {
	// LiveAirings lists the airings overlapping (from, to] in order, after the
	// last one that ended by from.
	LiveAirings(ctx context.Context, from, to time.Time) ([]LiveAiring, error)
	// PinAiring stores an airing's origin and feed unless it has them already,
	// and returns what is stored.
	PinAiring(ctx context.Context, id string, origin hls.LiveOrigin, feed Feed) (LiveAiring, error)
	// PinnedAiring reads an airing that went on air (db.ErrNotFound otherwise).
	PinnedAiring(ctx context.Context, id string) (*LiveAiring, error)
	// ReadyBreak is an airing's assembled break (db.ErrNotFound while it is not ready).
	ReadyBreak(ctx context.Context, airingID string) (*Break, error)
	// Evergreen is the newest assembled evergreen break (db.ErrNotFound while none is).
	Evergreen(ctx context.Context) (*Break, error)
	// Standby is the newest rendered standby card (db.ErrNotFound while none is).
	Standby(ctx context.Context) (*Clip, error)
	// FileSubtitles are a media file's subtitle tracks.
	FileSubtitles(ctx context.Context, fileID string) ([]liveSubtitle, error)
}

// Channel is the live stream computed from the schedule.
type Channel struct {
	store liveStore
	media liveMedia
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time

	// mu makes one pinning walk at a time
	mu sync.Mutex
	// pinned caches what went on air, which never changes
	cache sync.Mutex
	pins  map[string]LiveAiring
	built map[string][]hls.LiveItem
}

// NewChannel airs the schedule in st from the packages and breaks under dataDir.
func NewChannel(st *Store, m *DiskMedia) *Channel {
	return &Channel{store: st, media: m}
}

func (c *Channel) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// lineup walks the airings from the one on air at from to the last that starts
// by until, each with its feed. With pin, every airing that is on air at now
// gets its feed and origin stored, in order, chained from the one before it;
// the rest carry the feed they would air if nothing changed.
func (c *Channel) lineup(ctx context.Context, from, until, now time.Time, pin bool) ([]LiveAiring, error) {
	if pin {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	// an airing after a hole goes on air when the slate filling the hole does,
	// before its own start: the rows up to maxGap later are read for it
	rows, err := c.store.LiveAirings(ctx, from.Add(-time.Minute), until.Add(maxGap))
	if err != nil {
		return nil, err
	}
	var out []LiveAiring
	var prev *LiveAiring
	for i := range rows {
		a := rows[i]
		if a.Feed == nil {
			chained := prev != nil && a.StartsAt.Sub(prev.Feed.End()) <= maxGap
			start := a.StartsAt.Truncate(time.Millisecond)
			if chained {
				start = prev.Feed.End()
			} else if !a.EndsAt.After(from.Add(-time.Minute)) {
				continue // ended before the window and nobody saw it: nothing chains from it
			}
			if start.After(until) {
				break
			}
			feed, fallback := c.decide(ctx, a, start, nextProgram(rows[i+1:]))
			a.Feed = &feed
			if pin && !start.After(now) {
				if fallback != "" {
					slog.Warn("couchtv: an airing goes on air without its own media", "airingId", a.ID, "kind", a.Kind, "airs", fallback)
				}
				origin := freshOrigin(start)
				if chained && prev.pinned() {
					origin = chain(*prev.Origin, c.items(*prev))
				}
				stored, err := c.store.PinAiring(ctx, a.ID, origin, feed)
				if err != nil {
					return nil, err
				}
				a = stored
			}
		}
		rows[i] = a
		prev = &rows[i]
		out = append(out, a)
	}
	return out, nil
}

// nextProgram is when the first program among rows starts; zero for none.
func nextProgram(rows []LiveAiring) time.Time {
	for _, r := range rows {
		if r.Kind == KindProgram {
			return r.StartsAt
		}
	}
	return time.Time{}
}

// decide is what an airing airs from start: a program from its scheduled time
// (a filler covers a gap before it) to its cut, never past the next program's
// start; a break to its end (breakParts). A program that cannot air leaves a
// filler on until its planned end. fallback says what stands in when the
// airing's own media cannot air.
func (c *Channel) decide(ctx context.Context, a LiveAiring, start, next time.Time) (f Feed, fallback string) {
	f = Feed{Start: start}
	switch a.Kind {
	case KindProgram:
		begin := start
		if gap := a.StartsAt.Sub(start); gap >= minPart {
			f.add(c.fill(ctx, gap))
			begin = a.StartsAt
		}
		limit := time.Duration(math.MaxInt64)
		if !next.IsZero() {
			limit = next.Sub(begin)
		}
		p, missing := c.programPart(ctx, a, limit)
		if missing == "" {
			f.add(p)
		} else {
			p := c.fill(ctx, a.EndsAt.Sub(begin))
			f.add(p)
			fallback = fillerName(p) + " (" + missing + ")"
		}
	case KindBreak:
		var parts []LivePart
		parts, fallback = c.breakParts(ctx, a.ID, a.EndsAt.Sub(start))
		for _, p := range parts {
			f.add(p)
		}
	}
	return f, fallback
}

// programPart is the part a program airs, or what it misses.
func (c *Channel) programPart(ctx context.Context, a LiveAiring, limit time.Duration) (LivePart, string) {
	switch {
	case a.MediaFileID == "":
		return LivePart{}, "no media file"
	case limit <= 0:
		return LivePart{}, "the next program starts first"
	}
	pkg, err := c.media.Package(a.MediaFileID)
	if err != nil {
		return LivePart{}, "no package: " + err.Error()
	}
	video, cut := airable(pkg, a.CreditsAt)
	if video == nil {
		return LivePart{}, "nothing in the package can air"
	}
	if l := limit.Seconds(); cut > l {
		cut = min(l, lastBoundaryBefore(video.Playlist.Segments, l))
	}
	subs, err := c.store.FileSubtitles(ctx, a.MediaFileID)
	if err != nil {
		slog.Warn("couchtv: a program's subtitles", "airingId", a.ID, "err", err)
	}
	return LivePart{Kind: PartProgram, Ms: milliseconds(cut), File: a.MediaFileID, Video: video.Name,
		Audio: pickAudio(pkg.Stereo), Subtitles: pickSubtitles(subs)}, ""
}

// breakParts fill span with the break's assembled package, else the evergreen
// break cut to the gap, else a filler; fallback names what stands in. The
// break's own package shorter than the gap airs after a filler, so its up next
// still counts down into the program; the evergreen break has no up next, so a
// filler follows it.
func (c *Channel) breakParts(ctx context.Context, id string, span time.Duration) (parts []LivePart, fallback string) {
	if span <= 0 {
		return nil, ""
	}
	missing := "not ready"
	b, err := c.store.ReadyBreak(ctx, id)
	switch {
	case err == nil:
		m, err := c.media.Break(b.Path)
		if err == nil {
			return c.fitBreak(ctx, b.Path, m, span, true), ""
		}
		missing = "unreadable: " + err.Error()
	case !errors.Is(err, db.ErrNotFound):
		missing = err.Error()
	}
	if e, err := c.store.Evergreen(ctx); err == nil {
		if m, err := c.media.Break(e.Path); err == nil {
			return c.fitBreak(ctx, e.Path, m, span, false), "the evergreen break (" + missing + ")"
		}
	}
	p := c.fill(ctx, span)
	return []LivePart{p}, fillerName(p) + " (" + missing + ")"
}

// fitBreak airs a break's package in span: cut short when it is longer, with a
// filler before it (padFirst) or after it when it is shorter.
func (c *Channel) fitBreak(ctx context.Context, path string, b *breakMedia, span time.Duration, padFirst bool) []LivePart {
	p := LivePart{Kind: PartBreak, Path: path, Video: b.Video.Name, Ms: span.Milliseconds()}
	if b.Audio != nil {
		p.Audio = map[string]string{}
		for _, l := range Languages {
			p.Audio[l] = b.Audio.Name // the mix is the same in every language
		}
	}
	if span-b.duration() < minPart {
		return []LivePart{p}
	}
	p.Ms = b.duration().Milliseconds()
	pad := c.fill(ctx, span-p.duration())
	if padFirst {
		return []LivePart{pad, p}
	}
	return []LivePart{p, pad}
}

// fill is what airs for d where nothing else can: the standby card looped, else
// the slate.
func (c *Channel) fill(ctx context.Context, d time.Duration) LivePart {
	if card, err := c.store.Standby(ctx); err == nil {
		if _, err := c.media.Fill(card.Path); err == nil {
			return LivePart{Kind: PartStandby, Ms: d.Milliseconds(), File: card.ID, Path: card.Path}
		}
	}
	return slatePart(d)
}

func fillerName(p LivePart) string {
	if p.Kind == PartStandby {
		return "the standby card"
	}
	return "the slate"
}

// items are an airing's live items. A part whose media is gone airs the slate
// for its time, so the stream goes on.
func (c *Channel) items(a LiveAiring) []hls.LiveItem {
	if a.pinned() {
		c.cache.Lock()
		cached, ok := c.built[a.ID]
		c.cache.Unlock()
		if ok {
			return cached
		}
	}
	out := make([]hls.LiveItem, 0, len(a.Feed.Parts))
	complete := true
	for _, p := range a.Feed.Parts {
		item, err := partItem(a.ID, p, c.media)
		if err != nil {
			slog.Warn("couchtv: a part's media is unreadable; the slate airs instead", "airingId", a.ID, "kind", p.Kind, "err", err)
			item, _ = partItem(a.ID, slatePart(p.duration()), c.media)
			complete = false
		}
		out = append(out, item)
	}
	if a.pinned() && complete {
		c.remember(a, out)
	}
	return out
}

// remember caches a pinned airing (and its items, when given); the cache is
// dropped wholesale when it grows, since only the airings around now are asked for.
func (c *Channel) remember(a LiveAiring, items []hls.LiveItem) {
	c.cache.Lock()
	defer c.cache.Unlock()
	if c.pins == nil || len(c.pins) > 256 || len(c.built) > 256 {
		c.pins, c.built = map[string]LiveAiring{}, map[string][]hls.LiveItem{}
	}
	c.pins[a.ID] = a
	if items != nil {
		c.built[a.ID] = items
	}
}

// pinnedAiring reads an airing that went on air.
func (c *Channel) pinnedAiring(ctx context.Context, id string) (*LiveAiring, error) {
	c.cache.Lock()
	a, ok := c.pins[id]
	c.cache.Unlock()
	if ok {
		return &a, nil
	}
	stored, err := c.store.PinnedAiring(ctx, id)
	if err != nil {
		return nil, err
	}
	c.remember(*stored, nil)
	return stored, nil
}

// Schedule is the live presentation at now: the airings on air in the window,
// pinned. After the last one, if nothing follows, the slate holds the air.
func (c *Channel) Schedule(ctx context.Context, now time.Time) (*hls.LiveSchedule, error) {
	opens := now.Add(-liveWindow)
	airings, err := c.lineup(ctx, opens, now, now, true)
	if err != nil {
		return nil, err
	}
	var on []LiveAiring
	for _, a := range airings {
		switch {
		case !a.pinned():
		// a numbering that starts afresh (after a hole in the schedule) starts the presentation
		case len(on) > 0 && !a.Feed.Start.Equal(on[len(on)-1].Feed.End()),
			len(on) > 0 && !a.Feed.Start.After(opens):
			on = []LiveAiring{a}
		default:
			on = append(on, a)
		}
	}
	if len(on) == 0 || on[len(on)-1].Feed.End().Before(now.Add(-maxGap)) {
		return nil, hls.ErrOffAir
	}
	s := &hls.LiveSchedule{Start: on[0].Feed.Start, Origin: *on[0].Origin, TargetDuration: TargetDuration,
		Window: liveWindow, Trim: trimURI}
	for _, a := range on {
		s.Items = append(s.Items, c.items(a)...)
	}
	if end := on[len(on)-1].Feed.End(); !end.After(now) {
		item, err := partItem("", c.fill(ctx, now.Sub(end)+liveWindow), c.media)
		if err != nil {
			return nil, err
		}
		s.Items = append(s.Items, item)
	}
	return s, nil
}

// Playlist is the media playlist of rendition key at now.
func (c *Channel) Playlist(ctx context.Context, key string, now time.Time) (*hls.Media, error) {
	if !slices.Contains(liveKeys(), key) {
		return nil, hls.ErrOffAir
	}
	s, err := c.Schedule(ctx, now)
	if err != nil {
		return nil, err
	}
	return s.Playlist(key, now)
}

// Master is the channel's multivariant playlist: one video variant whose codecs,
// resolution, frame rate and bandwidths cover what airs from the window to
// masterAhead, Czech and English audio (the display language's the default)
// and subtitles.
func (c *Channel) Master(ctx context.Context, lang string) (*hls.Multivariant, error) {
	now := c.now()
	airings, err := c.lineup(ctx, now.Add(-liveWindow), now.Add(masterAhead), now, false)
	if err != nil {
		return nil, err
	}
	video := hls.Variant{URI: videoKey + ".m3u8", VideoRange: "SDR"}
	var codecs []string
	var videoPeak, videoAverage, audioPeak, audioAverage int64
	addVideo := func(info playback.RenditionInfo) {
		codecs = append(codecs, info.Codec)
		video.Width, video.Height = max(video.Width, info.Width), max(video.Height, info.Height)
		video.FrameRate = max(video.FrameRate, info.FrameRate)
		videoPeak, videoAverage = max(videoPeak, info.Peak), max(videoAverage, info.Average)
	}
	addAudio := func(info playback.RenditionInfo) {
		codecs = append(codecs, info.Codec)
		audioPeak, audioAverage = max(audioPeak, info.Peak), max(audioAverage, info.Average)
	}
	addFill := func(path string) {
		if f, err := c.media.Fill(path); err == nil {
			addVideo(f.Video.info)
			addAudio(f.Audio.info)
		}
	}
	addFill("")
	for _, a := range airings {
		for _, p := range a.Feed.Parts {
			if p.filler() {
				addFill(p.Path)
				continue
			}
			renditions, err := partRenditions(p, c.media)
			if err != nil {
				continue
			}
			if v := renditions[p.Video]; v != nil {
				addVideo(v.Info)
			}
			for _, name := range p.Audio {
				if r := renditions[name]; r != nil {
					addAudio(r.Info)
				}
			}
		}
	}
	video.Codecs = hls.CoverCodecs(codecs)
	video.Bandwidth, video.AverageBandwidth = videoPeak+audioPeak, videoAverage+audioAverage
	var audio, subs []hls.Rendition
	for _, l := range Languages {
		audio = append(audio, hls.Rendition{Name: languageNames[l], Language: l, Channels: "2", URI: audioKey(l) + ".m3u8"})
		subs = append(subs, hls.Rendition{Name: languageNames[l], Language: l, URI: subsKey(l) + ".m3u8"})
	}
	return hls.LiveMultivariant(video, audio, subs, lang), nil
}

// LineupPart is a stretch of the stream: one part of an airing's feed.
type LineupPart struct {
	AiringID string
	Kind     string
	Start    time.Time
	End      time.Time
	// File is a program's media file, Path a break's rendition directory.
	File, Path string
}

// Lineup lists what airs between from and to, part by part: as it aired where
// it went on air, else as it will air if nothing changes.
func (c *Channel) Lineup(ctx context.Context, from, to time.Time) ([]LineupPart, error) {
	airings, err := c.lineup(ctx, from, to, c.now(), false)
	if err != nil {
		return nil, err
	}
	var out []LineupPart
	for _, a := range airings {
		at := a.Feed.Start
		for _, p := range a.Feed.Parts {
			end := at.Add(p.duration())
			if end.After(from) && at.Before(to) {
				out = append(out, LineupPart{AiringID: a.ID, Kind: p.Kind, Start: at, End: end, File: p.File, Path: p.Path})
			}
			at = end
		}
	}
	return out, nil
}
