package couchtv

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
)

// fakeStore is the schedule in memory.
type fakeStore struct {
	rows      []LiveAiring
	breaks    map[string]string
	evergreen string
	standby   *Clip
	subs      map[string][]liveSubtitle
}

func (f *fakeStore) LiveAirings(_ context.Context, from, to time.Time) ([]LiveAiring, error) {
	var before *LiveAiring
	var out []LiveAiring
	for _, r := range f.rows {
		switch {
		case !r.EndsAt.After(from):
			if before == nil || !r.EndsAt.Before(before.EndsAt) {
				before = &r
			}
		case !r.StartsAt.After(to):
			out = append(out, r)
		}
	}
	if before != nil {
		out = append([]LiveAiring{*before}, out...)
	}
	return out, nil
}

func (f *fakeStore) PinAiring(_ context.Context, id string, origin hls.LiveOrigin, feed Feed) (LiveAiring, error) {
	for i := range f.rows {
		if r := &f.rows[i]; r.ID == id {
			if r.Origin == nil {
				r.Origin, r.Feed, r.State = &origin, &feed, "aired"
			}
			return *r, nil
		}
	}
	return LiveAiring{}, db.ErrNotFound
}

func (f *fakeStore) PinnedAiring(_ context.Context, id string) (*LiveAiring, error) {
	for _, r := range f.rows {
		if r.ID == id && r.pinned() {
			return &r, nil
		}
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) ReadyBreak(_ context.Context, id string) (*Break, error) {
	if path := f.breaks[id]; path != "" {
		return &Break{AiringID: id, Kind: "airing", State: BreakReady, Path: path}, nil
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) Standby(context.Context) (*Clip, error) {
	if f.standby != nil {
		return f.standby, nil
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) Evergreen(context.Context) (*Break, error) {
	if f.evergreen != "" {
		return &Break{Kind: "evergreen", State: BreakReady, Path: f.evergreen}, nil
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) FileSubtitles(_ context.Context, id string) ([]liveSubtitle, error) {
	return f.subs[id], nil
}

// fakeMedia holds packages and breaks in memory.
type fakeMedia struct {
	packages map[string]*playback.Package
	breaks   map[string]*breakMedia
}

func (m fakeMedia) Package(id string) (*playback.Package, error) {
	if p := m.packages[id]; p != nil {
		return p, nil
	}
	return nil, errors.New("no package")
}

func (m fakeMedia) Break(path string) (*breakMedia, error) {
	if b := m.breaks[path]; b != nil {
		return b, nil
	}
	return nil, errors.New("no break")
}

// Fill is the slate, or a standby card of two 6 s segments.
func (fakeMedia) Fill(path string) (*fill, error) {
	video := playback.RenditionInfo{Codec: "avc1.640028", Width: 1920, Height: 1080, FrameRate: 25, Peak: 100_000, Average: 100_000}
	audio := playback.RenditionInfo{Codec: "mp4a.40.2", Channels: "2", Peak: 130_000, Average: 130_000}
	switch path {
	case "":
		return &fill{Video: &loop{durations: []float64{2}, info: video}, Audio: &loop{durations: []float64{2.005333}, info: audio}}, nil
	case "clips/standby":
		return &fill{Video: &loop{durations: []float64{6, 6}, info: video}, Audio: &loop{durations: []float64{6.02, 5.98}, info: audio}}, nil
	}
	return nil, errors.New("no filler")
}

// rendition makes a finished rendition with segments of the given lengths.
func rendition(name string, info playback.RenditionInfo, durations ...float64) playback.PackageRendition {
	m := &hls.Media{Version: 7, PlaylistType: "VOD", EndList: true}
	init := &hls.Map{URI: "init.mp4"}
	for i, d := range durations {
		m.Segments = append(m.Segments, hls.Segment{Duration: d, URI: fmt.Sprintf("seg_%05d.m4s", i), Map: init})
		info.Duration += d
	}
	return playback.PackageRendition{Name: name, Info: info, Playlist: m}
}

var (
	at0    = time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)
	h264   = playback.RenditionInfo{Codec: "avc1.64001f", Width: 1280, Height: 720, FrameRate: 24, VideoRange: "SDR", Peak: 3_000_000, Average: 2_000_000}
	rung   = playback.RenditionInfo{Codec: "avc1.640028", Width: 1920, Height: 1080, FrameRate: 30, VideoRange: "SDR", Peak: 6_000_000, Average: 5_000_000}
	stereo = func(lang string, def bool) playback.RenditionInfo {
		return playback.RenditionInfo{Codec: "mp4a.40.2", Channels: "2", Language: lang, Default: def, Peak: 170_000, Average: 160_000}
	}
)

func uuidOf(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

func programRow(id, file int, start time.Time, length time.Duration, credits time.Duration) LiveAiring {
	return LiveAiring{Airing: Airing{ID: uuidOf(id), Kind: KindProgram, StartsAt: start, EndsAt: start.Add(length),
		MediaFileID: uuidOf(file), CreditsAt: credits, State: StatePlanned}}
}

func breakRow(id int, start, end time.Time) LiveAiring {
	return LiveAiring{Airing: Airing{ID: uuidOf(id), Kind: KindBreak, StartsAt: start, EndsAt: end, State: StatePlanned}}
}

func sec(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// testChannel airs, from 18:00:
//
//	harbor   0-20     a copied 720p source, English (default) and Czech audio, English subtitles
//	break    20-27    its rendition (7 s)
//	night    27-51    long copied segments, so the 1080p rung; credits at 25 s cut at 24; English audio only
//	break    51-90    not ready: the 20 s evergreen break, then 19 s of slate
//	missing  90-120   no package: the slate
//	(gap)    120-130  nothing planned: the slate opens the next airing
//	harbor   130-150  again
//	then nothing: the slate holds the air
func testChannel() (*Channel, *fakeStore) {
	files := fakeMedia{
		packages: map[string]*playback.Package{
			uuidOf(101): {
				Source: ptr(rendition("source", h264, 6, 6, 6, 2)),
				Stereo: []playback.PackageRendition{
					rendition("audio-1-aac", stereo("en", true), 6.02, 6.02, 6.02, 6.02),
					rendition("audio-2-aac", stereo("cs", false), 6.02, 6.02, 6.02, 6.02),
				},
			},
			uuidOf(102): {
				Source: ptr(rendition("source", rung, 16.6, 16.6)),
				Ladder: []playback.PackageRendition{rendition("1080p", rung, 6, 6, 6, 6, 6, 1.5)},
				Stereo: []playback.PackageRendition{rendition("audio-1-aac", stereo("en", false), 6, 6, 6, 6, 6, 1.5)},
			},
		},
		breaks: map[string]*breakMedia{
			"breaks/b1": {Video: ptr(rendition("video", rung, 6, 1)), Audio: ptr(rendition("audio", stereo("", false), 6.01, 0.99))},
			"breaks/e1": {Video: ptr(rendition("video", rung, 6, 6, 6, 2)), Audio: ptr(rendition("audio", stereo("", false), 6, 6, 6, 2))},
		},
	}
	st := &fakeStore{
		rows: []LiveAiring{
			programRow(1, 101, at0, 20*time.Second, 0),
			breakRow(2, at0.Add(20*time.Second), at0.Add(27*time.Second)),
			programRow(3, 102, at0.Add(27*time.Second), 24*time.Second, 25*time.Second),
			breakRow(4, at0.Add(51*time.Second), at0.Add(90*time.Second)),
			programRow(5, 103, at0.Add(90*time.Second), 30*time.Second, 0),
			programRow(6, 101, at0.Add(130*time.Second), 20*time.Second, 0),
		},
		breaks:    map[string]string{uuidOf(2): "breaks/b1"},
		evergreen: "breaks/e1",
		subs:      map[string][]liveSubtitle{uuidOf(101): {{ID: uuidOf(901), Lang: "en"}, {ID: uuidOf(902), Lang: "en", Forced: true}}},
	}
	return &Channel{store: st, media: files}, st
}

func uris(m *hls.Media) []string {
	out := make([]string, len(m.Segments))
	for i, s := range m.Segments {
		out[i] = s.URI
	}
	return out
}

func playlist(t *testing.T, c *Channel, key string, now time.Time) *hls.Media {
	t.Helper()
	m, err := c.Playlist(context.Background(), key, now)
	if err != nil {
		t.Fatalf("%s at %s: %v", key, now.Format("15:04:05.000"), err)
	}
	return m
}

func TestLiveLineup(t *testing.T) {
	c, _ := testChannel()
	// asked for at the end, every airing goes on air in order
	parts, err := c.Lineup(context.Background(), at0, at0.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	type span struct {
		kind       string
		start, end float64
	}
	var got []span
	for _, p := range parts {
		got = append(got, span{p.Kind, p.Start.Sub(at0).Seconds(), p.End.Sub(at0).Seconds()})
	}
	want := []span{
		{PartProgram, 0, 20}, {PartBreak, 20, 27}, {PartProgram, 27, 51},
		{PartBreak, 51, 71}, {PartSlate, 71, 90}, {PartSlate, 90, 120},
		{PartSlate, 120, 130}, {PartProgram, 130, 150},
	}
	if !slices.Equal(got, want) {
		t.Errorf("lineup\n got %v\nwant %v", got, want)
	}
	if parts[2].File != uuidOf(102) || parts[3].Path != "breaks/e1" {
		t.Errorf("parts %+v", parts)
	}
}

func TestLivePlaylists(t *testing.T) {
	c, st := testChannel()
	now := at0.Add(54500 * time.Millisecond)
	video := playlist(t, c, videoKey, now)
	if video.TargetDuration != TargetDuration {
		t.Errorf("target duration %d", video.TargetDuration)
	}
	p1, b1, p3, b4 := "../airings/"+uuidOf(1)+"/", "../airings/"+uuidOf(2)+"/", "../airings/"+uuidOf(3)+"/", "../airings/"+uuidOf(4)+"/"
	want := []string{
		p1 + "video/seg_00000.m4s", p1 + "video/seg_00001.m4s", p1 + "video/seg_00002.m4s", p1 + "video/seg_00003.m4s",
		b1 + "video/seg_00000.m4s", b1 + "video/seg_00001.m4s",
		// the copied source's 16.6 s segments exceed the target: the 1080p rung airs, cut at 24 s (credits at 25)
		p3 + "video/seg_00000.m4s", p3 + "video/seg_00001.m4s", p3 + "video/seg_00002.m4s", p3 + "video/seg_00003.m4s",
		// the break is not ready: the evergreen break airs
		b4 + "video/seg_00000.m4s",
	}
	if got := uris(video); !slices.Equal(got, want) {
		t.Fatalf("video\n got %v\nwant %v", got, want)
	}
	for i, seg := range video.Segments {
		item := i == 4 || i == 6 || i == 10
		if seg.Discontinuity != item || (i == 0 || item) == seg.ProgramDateTime.IsZero() {
			t.Errorf("segment %d: discontinuity %v, date %v", i, seg.Discontinuity, seg.ProgramDateTime)
		}
	}
	for i, at := range map[int]float64{0: 0, 4: 20, 6: 27, 10: 51} {
		if !video.Segments[i].ProgramDateTime.Equal(at0.Add(sec(at))) {
			t.Errorf("segment %d dated %s, want +%gs", i, video.Segments[i].ProgramDateTime, at)
		}
	}
	if v := playlist(t, c, videoKey, at0.Add(40*time.Second)); v.Segments[len(v.Segments)-1].URI != p3+"video/seg_00002.m4s" {
		t.Errorf("a segment is listed as soon as it goes on air: %v", uris(v))
	}

	// the program's audio is cut where its video is: the segment past 20 s is trimmed
	cs := playlist(t, c, audioKey("cs"), now)
	if got := cs.Segments[3].URI; got != p1+"audio-cs/seg_00003.m4s?keep=1.940" {
		t.Errorf("the overrunning audio segment is %s", got)
	}
	// Czech is the program's own track, English its default one; the second
	// program has only English, so the Czech rendition airs the original
	if got := cs.Segments[0].URI; got != p1+"audio-cs/seg_00000.m4s" {
		t.Errorf("cs audio %s", got)
	}
	f1, _ := st.PinnedAiring(context.Background(), uuidOf(1))
	if a := f1.Feed.Parts[0].Audio; a["cs"] != "audio-2-aac" || a["en"] != "audio-1-aac" {
		t.Errorf("harbor audio %v", a)
	}
	f3, _ := st.PinnedAiring(context.Background(), uuidOf(3))
	if a := f3.Feed.Parts[0]; a.Audio["cs"] != "audio-1-aac" || a.Audio["en"] != "audio-1-aac" || a.Video != "1080p" {
		t.Errorf("night %+v", a)
	}
	// the slate's silence fills the audio of a slate item
	slateAudio := playlist(t, c, audioKey("cs"), at0.Add(75*time.Second))
	last := slateAudio.Segments[len(slateAudio.Segments)-1]
	if !strings.HasPrefix(last.URI, "../slate/audio/") || last.Map.URI != "../slate/audio/init.mp4" {
		t.Errorf("slate audio %+v", last)
	}

	// subtitles: English where the program has them, empty WebVTT elsewhere
	en := playlist(t, c, subsKey("en"), now)
	csSubs := playlist(t, c, subsKey("cs"), now)
	if en.Segments[0].URI != p1+"subs-en/0.vtt" || en.Segments[3].URI != p1+"subs-en/3.vtt" || en.Segments[3].Duration != 2 {
		t.Errorf("en subtitles %v", uris(en))
	}
	for _, m := range []*hls.Media{csSubs, en} {
		for _, s := range m.Segments[4:] {
			if s.URI != emptySubtitles {
				t.Errorf("subtitles past the program: %s", s.URI)
			}
		}
	}
	if slices.ContainsFunc(csSubs.Segments, func(s hls.Segment) bool { return s.URI != emptySubtitles }) {
		t.Errorf("cs subtitles %v", uris(csSubs))
	}
	if f1.Feed.Parts[0].Subtitles["en"] != uuidOf(901) {
		t.Errorf("the full track before the forced one: %v", f1.Feed.Parts[0].Subtitles)
	}

	// every rendition starts every item at the same date, under the same number
	dates := func(m *hls.Media) []time.Time {
		var out []time.Time
		for _, s := range m.Segments {
			if s.Discontinuity {
				out = append(out, s.ProgramDateTime)
			}
		}
		return out
	}
	for _, key := range liveKeys() {
		m := playlist(t, c, key, now)
		if !slices.Equal(dates(m), dates(video)) || m.DiscontinuitySequence != video.DiscontinuitySequence {
			t.Errorf("%s starts its items at %v, the video at %v", key, dates(m), dates(video))
		}
	}
}

// numbering records the media sequence number every segment URI gets, and what
// every number names: once listed, a segment never changes.
type numbering map[string]map[string]string

func (n numbering) check(t *testing.T, key string, m *hls.Media) {
	t.Helper()
	if n[key] == nil {
		n[key] = map[string]string{}
	}
	for i, s := range m.Segments {
		seq := fmt.Sprint(m.MediaSequence + i)
		listed := fmt.Sprintf("%s %.6f", s.URI, s.Duration)
		if was, ok := n[key][seq]; ok && was != listed {
			t.Errorf("%s: %s lists %s, earlier %s", key, seq, listed, was)
		}
		n[key][seq] = listed
		// a filler's URIs repeat across items
		if uri, _, _ := strings.Cut(s.URI, "?"); !strings.HasPrefix(uri, "../slate/") && !strings.HasPrefix(uri, "../standby/") {
			if was, ok := n[key][uri]; ok && was != seq {
				t.Errorf("%s: %s numbered %s, earlier %s", key, uri, seq, was)
			}
			n[key][uri] = seq
		}
	}
}

func TestLiveNumbering(t *testing.T) {
	c, st := testChannel()
	seen := numbering{}
	var texts []string
	for at := at0; at.Before(at0.Add(170 * time.Second)); at = at.Add(700 * time.Millisecond) {
		for _, key := range liveKeys() {
			m := playlist(t, c, key, at)
			seen.check(t, key, m)
			if key == videoKey {
				texts = append(texts, m.String())
			}
		}
	}
	for _, r := range st.rows {
		if !r.pinned() {
			t.Errorf("airing %s never went on air", r.ID)
		}
	}
	// origins chain: each airing starts where the one before it ended
	for i := 1; i < len(st.rows); i++ {
		prev, cur := st.rows[i-1], st.rows[i]
		want := chain(*prev.Origin, c.items(prev))
		if fmt.Sprint(*cur.Origin) != fmt.Sprint(want) {
			t.Errorf("airing %d origin %v, want %v", i, *cur.Origin, want)
		}
	}

	// a restarted server lists the very same playlists from what is stored
	restarted := &Channel{store: st, media: c.media}
	i := 0
	for at := at0; at.Before(at0.Add(170 * time.Second)); at = at.Add(700 * time.Millisecond) {
		if got := playlist(t, restarted, videoKey, at).String(); got != texts[i] {
			t.Fatalf("after a restart at %s:\n%s\nbefore:\n%s", at.Format("15:04:05.000"), got, texts[i])
		}
		i++
	}
}

func TestLiveEdges(t *testing.T) {
	c, _ := testChannel()
	ctx := context.Background()
	if _, err := c.Playlist(ctx, videoKey, at0.Add(-time.Second)); !errors.Is(err, hls.ErrOffAir) {
		t.Errorf("before the first airing: %v", err)
	}
	if _, err := c.Playlist(ctx, "audio-de", at0); !errors.Is(err, hls.ErrOffAir) {
		t.Errorf("a rendition the channel lacks: %v", err)
	}
	// after the last airing the slate holds the air, numbered on
	tail := playlist(t, c, videoKey, at0.Add(3*time.Minute))
	lastProgram := slices.IndexFunc(tail.Segments, func(s hls.Segment) bool { return strings.Contains(s.URI, uuidOf(6)) })
	if lastProgram < 0 || !strings.HasPrefix(tail.Segments[len(tail.Segments)-1].URI, "../slate/video/") ||
		!tail.Segments[len(tail.Segments)-1].ProgramDateTime.IsZero() {
		t.Errorf("the tail %v", uris(tail))
	}
	if _, err := c.Playlist(ctx, videoKey, at0.Add(7*time.Hour)); !errors.Is(err, hls.ErrOffAir) {
		t.Errorf("hours after the last airing: %v", err)
	}

	// nobody watched for a while: the airing on air starts a fresh numbering,
	// above anything numbered before
	c2, st2 := testChannel()
	st2.rows = append(st2.rows, programRow(7, 101, at0.Add(10*time.Hour), 20*time.Second, 0))
	early := playlist(t, c2, videoKey, at0.Add(10*time.Second))
	late := playlist(t, c2, videoKey, at0.Add(10*time.Hour+10*time.Second))
	if late.MediaSequence <= early.MediaSequence || late.DiscontinuitySequence <= early.DiscontinuitySequence ||
		!late.Segments[0].ProgramDateTime.Equal(at0.Add(10*time.Hour)) {
		t.Errorf("fresh numbering %d/%d after %d/%d", late.MediaSequence, late.DiscontinuitySequence, early.MediaSequence, early.DiscontinuitySequence)
	}

	// a program never runs into the next one: it is cut at the last boundary before it
	c3, st3 := testChannel()
	st3.rows[2].EndsAt = at0.Add(40 * time.Second)
	st3.rows[3] = programRow(4, 101, at0.Add(40*time.Second), 20*time.Second, 0)
	parts, err := c3.Lineup(ctx, at0, at0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if parts[2].End.Sub(at0) != 39*time.Second || parts[3].Kind != PartSlate || parts[3].End.Sub(at0) != 40*time.Second {
		t.Errorf("capped program %+v %+v", parts[2], parts[3])
	}
}

func TestLiveMaster(t *testing.T) {
	c, _ := testChannel()
	c.Now = func() time.Time { return at0.Add(10 * time.Second) }
	m, err := c.Master(context.Background(), "en")
	if err != nil {
		t.Fatal(err)
	}
	text := m.String()
	for _, want := range []string{
		`CODECS="avc1.640028,mp4a.40.2"`, "RESOLUTION=1920x1080", "FRAME-RATE=30.000", "BANDWIDTH=6170000", "AVERAGE-BANDWIDTH=5160000",
		`TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2",URI="audio-en.m3u8"`,
		`TYPE=AUDIO,GROUP-ID="audio",NAME="Čeština",LANGUAGE="cs",DEFAULT=NO`,
		`TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,URI="subs-en.m3u8"`,
		"\nvideo.m3u8\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("master lacks %s:\n%s", want, text)
		}
	}
	// any other display language gets Czech first
	m, _ = c.Master(context.Background(), "de")
	if !strings.Contains(m.String(), `NAME="Čeština",LANGUAGE="cs",DEFAULT=YES`) {
		t.Errorf("default audio:\n%s", m)
	}
}

func TestAirable(t *testing.T) {
	source := func(info playback.RenditionInfo, durations ...float64) *playback.Package {
		return &playback.Package{Source: ptr(rendition("source", info, durations...))}
	}
	hevc := playback.RenditionInfo{Codec: "hvc1.2.4.L120.90", Width: 1920, Height: 1080, VideoRange: "PQ"}
	high10 := h264
	high10.Codec = "avc1.6e0028"
	cases := []struct {
		name    string
		pkg     *playback.Package
		credits time.Duration
		video   string
		cut     float64
	}{
		{"a copied H.264 source airs to its end", source(h264, 6, 6, 6, 3), 0, "source", 21},
		{"to the boundary nearest the credits", source(h264, 6, 6, 6, 3), 13 * time.Second, "source", 12},
		{"credits past the end are ignored", source(h264, 6, 6), 30 * time.Second, "source", 12},
		{"segments up to the target air as copied", source(h264, 10.5, 10.5, 5), 0, "source", 26},
		{"longer ones before the cut need the rung", &playback.Package{Source: ptr(rendition("source", h264, 6, 16.6, 6)),
			Ladder: []playback.PackageRendition{rendition("720p", h264, 6, 6, 6, 4.6)}}, 0, "720p", 22.6},
		{"a long segment after the credits does not", source(h264, 6, 6, 16.6), 12 * time.Second, "source", 12},
		{"HEVC airs from the tallest rung up to 1080p", &playback.Package{Source: ptr(rendition("source", hevc, 6, 6)),
			Ladder: []playback.PackageRendition{rendition("2160p", playback.RenditionInfo{Codec: "avc1.640033", Width: 3840, Height: 2160}, 6, 6),
				rendition("1080p", rung, 6, 6), rendition("720p", h264, 6, 6)}}, 0, "1080p", 12},
		{"High 10 is not 8-bit", source(high10, 6, 6), 0, "", 0},
		{"nothing to air", &playback.Package{}, 0, "", 0},
	}
	for _, c := range cases {
		video, cut := airable(c.pkg, c.credits)
		name := ""
		if video != nil {
			name = video.Name
		}
		if name != c.video || cut != c.cut {
			t.Errorf("%s: %s cut at %g, want %s at %g", c.name, name, cut, c.video, c.cut)
		}
	}
}

func TestPickAudio(t *testing.T) {
	cases := []struct {
		tracks []playback.RenditionInfo
		want   map[string]string
	}{
		{[]playback.RenditionInfo{stereo("en", true), stereo("cs", false)}, map[string]string{"cs": "a1", "en": "a0"}},
		{[]playback.RenditionInfo{stereo("ja", false), stereo("en", true)}, map[string]string{"cs": "a1", "en": "a1"}},
		{[]playback.RenditionInfo{stereo("und", false)}, map[string]string{"cs": "a0", "en": "a0"}},
		// a commentary after the main English track does not take its place
		{[]playback.RenditionInfo{stereo("cs", false), stereo("en", true), stereo("en", false)}, map[string]string{"cs": "a0", "en": "a1"}},
		{nil, nil},
	}
	for _, c := range cases {
		var tracks []playback.PackageRendition
		for i, info := range c.tracks {
			tracks = append(tracks, rendition(fmt.Sprintf("a%d", i), info, 6))
		}
		if got := pickAudio(tracks); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%v: %v, want %v", c.tracks, got, c.want)
		}
	}
}

// TestLiveStandby airs the standby card where nothing else can, once one is
// rendered: after the evergreen break, for a program that cannot air, in a hole
// and after the last airing, looping across its passes.
func TestLiveStandby(t *testing.T) {
	c, st := testChannel()
	st.standby = &Clip{ID: uuidOf(950), Kind: ClipStandby, State: ClipReady, Path: "clips/standby"}
	parts, err := c.Lineup(context.Background(), at0, at0.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, p := range parts {
		kinds = append(kinds, p.Kind)
	}
	want := []string{PartProgram, PartBreak, PartProgram, PartBreak, PartStandby, PartStandby, PartStandby, PartProgram}
	if !slices.Equal(kinds, want) {
		t.Errorf("parts %v, want %v", kinds, want)
	}

	// the 19 s after the evergreen break: two passes, the copies numbered on
	video := playlist(t, c, videoKey, at0.Add(89*time.Second))
	var standby []string
	for _, s := range video.Segments {
		if strings.HasPrefix(s.URI, "../standby/") {
			standby = append(standby, fmt.Sprintf("%s %g", s.URI, s.Duration))
		}
	}
	prefix := "../standby/" + uuidOf(950) + "/video/"
	if wantURIs := []string{prefix + "0.m4s 6", prefix + "1.m4s 6", prefix + "2.m4s 6", prefix + "3.m4s?keep=1.000 1"}; !slices.Equal(standby, wantURIs) {
		t.Errorf("standby copies %v, want %v", standby, wantURIs)
	}
	seen := numbering{}
	for at := at0; at.Before(at0.Add(170 * time.Second)); at = at.Add(700 * time.Millisecond) {
		for _, key := range liveKeys() {
			seen.check(t, key, playlist(t, c, key, at))
		}
	}
}
