package hls

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

var epoch = time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)

// rendition makes segments of the given durations, named <prefix>-<n>.m4s,
// under one init section.
func rendition(prefix string, durations ...float64) []Segment {
	init := &Map{URI: prefix + "-init.mp4"}
	out := make([]Segment, len(durations))
	for i, d := range durations {
		out[i] = Segment{Duration: d, URI: fmt.Sprintf("%s-%d.m4s", prefix, i), Map: init}
	}
	return out
}

// testSchedule is a program (6 s video segments, audio cut elsewhere, aired
// for 20 s), a 7 s break and a program with long copied-source segments.
func testSchedule() *LiveSchedule {
	return &LiveSchedule{
		Start:  epoch,
		Origin: LiveOrigin{MediaSequence: map[string]int{"video": 100, "audio": 200}, DiscontinuitySequence: 5},
		Items: []LiveItem{
			{Duration: 20 * time.Second, Renditions: map[string][]Segment{
				"video": rendition("a/video", 6, 6, 6, 2, 6),
				"audio": rendition("a/audio", 6.02, 6.02, 6.02, 6.02, 6.02),
			}},
			{Duration: 7 * time.Second, Renditions: map[string][]Segment{
				"video": rendition("break/video", 6, 1),
				"audio": rendition("break/audio", 6.01, 0.99),
			}},
			{Duration: 31500 * time.Millisecond, Renditions: map[string][]Segment{
				"video": rendition("c/video", 10.5, 10.5, 10.5),
				"audio": rendition("c/audio", 6, 6, 6, 6, 6, 1.5),
			}},
		},
		TargetDuration: 6,
		Window:         30 * time.Second,
	}
}

func uris(m *Media) []string {
	out := make([]string, len(m.Segments))
	for i, s := range m.Segments {
		out[i] = s.URI
	}
	return out
}

func TestLiveFit(t *testing.T) {
	s := testSchedule()
	for _, key := range []string{"video", "audio"} {
		start := s.Start
		for i, item := range s.Items {
			segments := fit(item.Renditions[key], item.Duration.Seconds())
			var total float64
			for _, seg := range segments {
				total += seg.Duration
			}
			if math.Abs(total-item.Duration.Seconds()) > 1e-9 {
				t.Errorf("%s item %d lasts %.6fs, want %s", key, i, total, item.Duration)
			}
			start = start.Add(item.Duration)
		}
	}
	// the program's audio keeps a segment that starts before the cut and ends
	// on it; the break's 10 ms tail is dropped
	a := fit(s.Items[0].Renditions["audio"], 20)
	if len(a) != 4 || math.Abs(a[3].Duration-(20-3*6.02)) > 1e-9 {
		t.Errorf("program audio %+v", a)
	}
	if b := fit(s.Items[1].Renditions["audio"], 7); len(b) != 2 {
		t.Errorf("break audio %+v", b)
	}
	if v := fit(s.Items[0].Renditions["video"], 20); len(v) != 4 || v[3].Duration != 2 {
		t.Errorf("program video %+v", v)
	}
}

func TestLivePlaylist(t *testing.T) {
	s := testSchedule()
	// target duration: the 10.5 s segments raise the channel's 6
	now := epoch.Add(30 * time.Second)
	m, err := s.Playlist("video", now)
	if err != nil {
		t.Fatal(err)
	}
	if m.TargetDuration != 11 {
		t.Errorf("target duration %d, want 11", m.TargetDuration)
	}
	// the window is at least four target durations: everything since 18:00
	want := []string{"a/video-0.m4s", "a/video-1.m4s", "a/video-2.m4s", "a/video-3.m4s",
		"break/video-0.m4s", "break/video-1.m4s", "c/video-0.m4s"}
	if got := uris(m); !slices.Equal(got, want) {
		t.Fatalf("segments %v, want %v", got, want)
	}
	if m.MediaSequence != 100 || m.DiscontinuitySequence != 5 {
		t.Errorf("sequence %d/%d, want 100/5", m.MediaSequence, m.DiscontinuitySequence)
	}
	for i, seg := range m.Segments {
		starts := i == 4 || i == 6
		if seg.Discontinuity != starts || seg.ProgramDateTime.IsZero() != (!starts && i > 0) {
			t.Errorf("segment %d: discontinuity %v, date %v", i, seg.Discontinuity, seg.ProgramDateTime)
		}
	}
	if !m.Segments[4].ProgramDateTime.Equal(epoch.Add(20*time.Second)) || !m.Segments[6].ProgramDateTime.Equal(epoch.Add(27*time.Second)) {
		t.Errorf("item dates %v %v", m.Segments[4].ProgramDateTime, m.Segments[6].ProgramDateTime)
	}

	text := m.String()
	for _, line := range []string{
		"#EXT-X-DISCONTINUITY-SEQUENCE:5", "#EXT-X-PROGRAM-DATE-TIME:2026-10-12T18:00:00.000Z",
		"#EXT-X-DISCONTINUITY\n#EXT-X-PROGRAM-DATE-TIME:2026-10-12T18:00:27.000Z\n#EXT-X-MAP:URI=\"c/video-init.mp4\"\n#EXTINF:10.500000,",
	} {
		if !strings.Contains(text, line) {
			t.Errorf("playlist lacks %q:\n%s", line, text)
		}
	}
	if strings.Contains(text, "ENDLIST") || strings.Contains(text, "PLAYLIST-TYPE") {
		t.Errorf("a live playlist has no end or type:\n%s", text)
	}
	back, err := Parse(text)
	if err != nil || back.Media == nil || back.Media.DiscontinuitySequence != 5 ||
		!back.Media.Segments[6].ProgramDateTime.Equal(m.Segments[6].ProgramDateTime) {
		t.Errorf("round trip: %v %+v", err, back)
	}

	// later: the first item scrolls out, every segment keeps its numbers and
	// the playlist counts the discontinuity it no longer lists
	later, err := s.Playlist("video", epoch.Add(60*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if later.Segments[0].URI != "a/video-2.m4s" || later.MediaSequence != 102 || later.DiscontinuitySequence != 5 {
		t.Errorf("later starts at %s %d/%d", later.Segments[0].URI, later.MediaSequence, later.DiscontinuitySequence)
	}
	if !later.Segments[0].ProgramDateTime.Equal(epoch.Add(12 * time.Second)) {
		t.Errorf("a window opening mid-item dates its first segment %v", later.Segments[0].ProgramDateTime)
	}
	if got := later.Segments[len(later.Segments)-1].URI; got != "c/video-2.m4s" {
		t.Errorf("last segment %s", got)
	}

	// a schedule chained from a later origin numbers the same segments the same way
	chained := testSchedule()
	chained.Origin = s.Origin.After(s.Items[0])
	chained.Start = epoch.Add(s.Items[0].Duration)
	chained.Items = chained.Items[1:]
	at := epoch.Add(65 * time.Second)
	for _, key := range []string{"video", "audio"} {
		a, err1 := s.Playlist(key, at)
		b, err2 := chained.Playlist(key, at)
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		if a.String() != b.String() {
			t.Errorf("%s differs once chained:\n%s\n%s", key, a, b)
		}
	}

	if _, err := s.Playlist("video", epoch.Add(-time.Second)); !errors.Is(err, ErrOffAir) {
		t.Errorf("before the schedule: %v", err)
	}
	if _, err := s.Playlist("subtitles", now); err == nil {
		t.Error("a rendition no item has")
	}
}

func TestLiveRenditionsAgree(t *testing.T) {
	s := testSchedule()
	now := epoch.Add(50 * time.Second)
	video, _ := s.Playlist("video", now)
	audio, _ := s.Playlist("audio", now)
	dates := func(m *Media) []time.Time {
		var out []time.Time
		for _, seg := range m.Segments {
			if seg.Discontinuity {
				out = append(out, seg.ProgramDateTime)
			}
		}
		return out
	}
	if !slices.Equal(dates(video), dates(audio)) || video.DiscontinuitySequence != audio.DiscontinuitySequence {
		t.Errorf("items start at %v in video, %v in audio", dates(video), dates(audio))
	}

	// the program's audio segment that runs past the cut is listed trimmed
	s.Trim = func(uri string, keep float64) string { return fmt.Sprintf("%s?keep=%.3f", uri, keep) }
	audio, _ = s.Playlist("audio", epoch.Add(60*time.Second))
	var trimmed []string
	for _, seg := range audio.Segments {
		if strings.Contains(seg.URI, "?") {
			trimmed = append(trimmed, seg.URI)
		}
	}
	if !slices.Equal(trimmed, []string{"a/audio-3.m4s?keep=1.940"}) {
		t.Errorf("trimmed %v", trimmed)
	}
}

func TestCut(t *testing.T) {
	segments := rendition("v", 10.5, 10.5, 10.5, 8.5)
	for _, c := range []struct{ at, want float64 }{{30, 31.5}, {26, 21}, {2, 10.5}, {100, 40}} {
		if got := Cut(segments, c.at); got != c.want {
			t.Errorf("Cut(%v) = %v, want %v", c.at, got, c.want)
		}
	}
}

func TestCoverCodecs(t *testing.T) {
	cases := []struct{ in, want []string }{
		{[]string{"avc1.64001f", "mp4a.40.2", "avc1.640028", "avc1.4d401e", "mp4a.40.2"}, []string{"avc1.640028", "mp4a.40.2"}},
		{[]string{"avc1.42c01e", "avc1.64001f"}, []string{"avc1.64001f"}},
		{[]string{"avc1.42c01e", "avc1.42e01f"}, []string{"avc1.42c01f"}},
		{[]string{"hvc1.1.6.L93.90", "mp4a.40.2"}, []string{"hvc1.1.6.L93.90", "mp4a.40.2"}},
	}
	for _, c := range cases {
		if got := CoverCodecs(c.in); !slices.Equal(got, c.want) {
			t.Errorf("CoverCodecs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	if !codecCovers("avc1.640028", "avc1.42c01e") || codecCovers("avc1.64001f", "avc1.640028") || codecCovers("avc1.4d4028", "avc1.640028") {
		t.Error("codecCovers")
	}
}

// livePresentation is a live presentation of four one-second items (the
// fixture segments), each starting a discontinuity, with audio and an empty
// subtitle rendition.
func livePresentation(t *testing.T, now time.Time) memory {
	m := presentation(t)
	item := LiveItem{Duration: time.Second, Renditions: map[string][]Segment{
		"video": {{Duration: 1, URI: "../video/seg.m4s", Map: &Map{URI: "../video/init.mp4"}}},
		"audio": {{Duration: 1, URI: "../audio/seg.m4s", Map: &Map{URI: "../audio/init.mp4"}}},
		"subs":  {{Duration: 1, URI: "empty.vtt"}},
	}}
	s := &LiveSchedule{Start: epoch, Items: []LiveItem{item, item, item, item, item, item},
		Origin: LiveOrigin{MediaSequence: map[string]int{}}, TargetDuration: 1, Window: 4 * time.Second}
	for key, path := range map[string]string{"video": "/live/video.m3u8", "audio": "/live/audio.m3u8", "subs": "/live/subs.m3u8"} {
		pl, err := s.Playlist(key, now)
		if err != nil {
			t.Fatal(err)
		}
		m[path] = file{[]byte(pl.String()), m3u8}
	}
	mv := LiveMultivariant(
		Variant{Bandwidth: 400000, AverageBandwidth: 380000, Codecs: []string{"avc1.42c00a", "mp4a.40.2"},
			Width: 128, Height: 72, FrameRate: 24, VideoRange: "SDR", URI: "video.m3u8"},
		[]Rendition{{Name: "English", Language: "en", Channels: "2", URI: "audio.m3u8"}},
		[]Rendition{{Name: "English", Language: "en", URI: "subs.m3u8"}}, "en")
	m["/live/master.m3u8"] = file{[]byte(mv.String()), m3u8}
	m["/live/empty.vtt"] = file{[]byte((&WebVTT{}).Segment(0, 0, 126000)), vtt}
	return m
}

func liveRules(t *testing.T, m memory) []string {
	t.Helper()
	report, err := Validate(context.Background(), m, "http://test/live/master.m3u8", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range report.Issues {
		out = append(out, i.Rule)
	}
	return out
}

func TestValidLivePresentation(t *testing.T) {
	m := livePresentation(t, epoch.Add(5500*time.Millisecond))
	report, err := Validate(context.Background(), m, "http://test/live/master.m3u8", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) > 0 {
		t.Fatalf("issues %v\n%s", report.Issues, m["/live/video.m3u8"].body)
	}
}

func TestLiveValidationRules(t *testing.T) {
	edit := func(path, from, to string) func(memory) {
		return func(m memory) {
			f := m[path]
			if !strings.Contains(string(f.body), from) {
				panic(from)
			}
			f.body = []byte(strings.Replace(string(f.body), from, to, 1))
			m[path] = f
		}
	}
	cases := []struct {
		rule   string
		change func(memory)
	}{
		{"live.alignment", edit("/live/audio.m3u8", "18:00:02.000Z", "18:00:02.500Z")},
		{"live.alignment", edit("/live/subs.m3u8", "#EXT-X-DISCONTINUITY-SEQUENCE:1\n", "#EXT-X-DISCONTINUITY-SEQUENCE:2\n")},
		{"live.program-date-time", edit("/live/video.m3u8", "#EXT-X-PROGRAM-DATE-TIME:2026-10-12T18:00:03.000Z\n", "")},
		{"live.gap", edit("/live/video.m3u8", "18:00:03.000Z", "18:00:03.400Z")},
		{"media.program-date-time", edit("/live/video.m3u8", "#EXT-X-DISCONTINUITY\n#EXT-X-PROGRAM-DATE-TIME:2026-10-12T18:00:03.000Z", "#EXT-X-PROGRAM-DATE-TIME:2026-10-12T18:00:03.400Z")},
		{"live.window", edit("/live/video.m3u8", "#EXT-X-TARGETDURATION:1", "#EXT-X-TARGETDURATION:2")},
		{"variant.resolution", edit("/live/master.m3u8", "RESOLUTION=128x72", "RESOLUTION=64x36")},
		{"variant.codecs", edit("/live/master.m3u8", `CODECS="avc1.42c00a,mp4a.40.2"`, `CODECS="avc1.42c009,mp4a.40.2"`)},
		{"media.channels", edit("/live/master.m3u8", `CHANNELS="2"`, `CHANNELS="1"`)},
	}
	for _, c := range cases {
		t.Run(c.rule, func(t *testing.T) {
			m := livePresentation(t, epoch.Add(5500*time.Millisecond))
			c.change(m)
			if got := liveRules(t, m); !slices.Contains(got, c.rule) {
				t.Errorf("rule %s not reported; got %v", c.rule, got)
			}
		})
	}
	// a live variant covers its items with a higher profile, size and channel count
	m := livePresentation(t, epoch.Add(5500*time.Millisecond))
	edit("/live/master.m3u8", `CODECS="avc1.42c00a,mp4a.40.2",RESOLUTION=128x72`, `CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x1080`)(m)
	edit("/live/master.m3u8", `CHANNELS="2"`, `CHANNELS="6"`)(m)
	if got := liveRules(t, m); len(got) > 0 {
		t.Errorf("a covering variant: %v", got)
	}
}
