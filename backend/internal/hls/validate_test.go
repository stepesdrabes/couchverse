package hls

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
)

type file struct {
	body        []byte
	contentType string
}

// memory serves a presentation from a map keyed by path.
type memory map[string]file

func (m memory) Fetch(_ context.Context, u *url.URL) ([]byte, string, error) {
	f, ok := m[u.Path]
	if !ok {
		return nil, "", fmt.Errorf("HTTP 404")
	}
	return f.body, f.contentType, nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../media/mp4/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

const (
	m3u8 = "application/vnd.apple.mpegurl"
	vtt  = "text/vtt"
)

// presentation is a valid one-second fMP4 presentation: H.264 128x72 at 24 fps,
// an AAC stereo rendition, a subtitle rendition and an I-frame playlist.
func presentation(t *testing.T) memory {
	media := func(segment string, extra string) string {
		return "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:1\n#EXT-X-PLAYLIST-TYPE:VOD\n" + extra +
			"#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1.000,\n" + segment + "\n#EXT-X-ENDLIST\n"
	}
	return memory{
		"/master.m3u8": {[]byte(`#EXTM3U
#EXT-X-VERSION:7
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="stereo",NAME="English",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2",URI="audio/index.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,URI="subs/index.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=400000,AVERAGE-BANDWIDTH=380000,CODECS="avc1.42c00a,mp4a.40.2",RESOLUTION=128x72,FRAME-RATE=24.000,VIDEO-RANGE=SDR,AUDIO="stereo",SUBTITLES="subs",CLOSED-CAPTIONS=NONE
video/index.m3u8
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=300000,CODECS="avc1.42c00a",RESOLUTION=128x72,VIDEO-RANGE=SDR,URI="video/iframes.m3u8"
`), m3u8},
		"/video/index.m3u8":   {[]byte(media("seg.m4s", "#EXT-X-INDEPENDENT-SEGMENTS\n")), m3u8},
		"/video/iframes.m3u8": {[]byte(media("seg.m4s", "#EXT-X-I-FRAMES-ONLY\n")), m3u8},
		"/video/init.mp4":     {fixture(t, "avc.init.mp4"), "video/mp4"},
		"/video/seg.m4s":      {fixture(t, "avc.seg.m4s"), "video/mp4"},
		"/audio/index.m3u8":   {[]byte(media("seg.m4s", "")), m3u8},
		"/audio/init.mp4":     {fixture(t, "aac.init.mp4"), "audio/mp4"},
		"/audio/seg.m4s":      {fixture(t, "aac.seg.m4s"), "audio/mp4"},
		"/subs/index.m3u8": {[]byte("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:1\n#EXT-X-PLAYLIST-TYPE:VOD\n" +
			"#EXTINF:1.000,\n0.vtt\n#EXT-X-ENDLIST\n"), m3u8},
		"/subs/0.vtt": {[]byte("WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:0,LOCAL:00:00:00.000\n\n00:00:00.100 --> 00:00:00.900\nHello\n"), vtt},
	}
}

func rules(t *testing.T, m memory, severity Severity) []string {
	t.Helper()
	report, err := Validate(context.Background(), m, "http://test/master.m3u8", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range report.Issues {
		if i.Severity == severity {
			out = append(out, i.Rule)
		}
	}
	return out
}

func TestValidPresentation(t *testing.T) {
	m := presentation(t)
	if errs := rules(t, m, SeverityError); len(errs) > 0 {
		report, _ := Validate(context.Background(), m, "http://test/master.m3u8", Options{})
		t.Fatalf("errors %v\n%v", errs, report.Issues)
	}
}

func TestValidationRules(t *testing.T) {
	edit := func(path, from, to string) func(memory) {
		return func(m memory) {
			f := m[path]
			f.body = []byte(strings.Replace(string(f.body), from, to, 1))
			m[path] = f
		}
	}
	cases := []struct {
		rule   string
		change func(memory)
	}{
		{"variant.codecs", edit("/master.m3u8", `CODECS="avc1.42c00a,mp4a.40.2"`, `CODECS="avc1.640028"`)},
		{"variant.resolution", edit("/master.m3u8", "RESOLUTION=128x72,FRAME", "RESOLUTION=1920x1080,FRAME")},
		{"variant.bandwidth", edit("/master.m3u8", "BANDWIDTH=400000,", "BANDWIDTH=1000,")},
		{"variant.average-bandwidth", edit("/master.m3u8", "AVERAGE-BANDWIDTH=380000,", "")},
		{"variant.frame-rate", edit("/master.m3u8", "FRAME-RATE=24.000,", "")},
		{"variant.group", edit("/master.m3u8", `AUDIO="stereo"`, `AUDIO="surround"`)},
		{"media.channels", edit("/master.m3u8", `CHANNELS="2"`, `CHANNELS="6"`)},
		{"media.default-unique", edit("/master.m3u8", "#EXT-X-STREAM-INF",
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"stereo\",NAME=\"Czech\",LANGUAGE=\"cs\",DEFAULT=YES,AUTOSELECT=YES,CHANNELS=\"2\",URI=\"audio/index.m3u8\"\n#EXT-X-STREAM-INF")},
		{"media.language", edit("/master.m3u8", `LANGUAGE="en",DEFAULT=YES`, `LANGUAGE="english",DEFAULT=YES`)},
		{"media.autoselect", edit("/master.m3u8", "DEFAULT=YES,AUTOSELECT=YES", "DEFAULT=YES,AUTOSELECT=NO")},
		{"media.extinf", edit("/video/index.m3u8", "#EXTINF:1.000,", "#EXTINF:2.000,")},
		{"media.endlist", edit("/video/index.m3u8", "#EXT-X-ENDLIST\n", "")},
		{"fmp4.map", edit("/audio/index.m3u8", "#EXT-X-MAP:URI=\"init.mp4\"\n", "")},
		{"version.features", edit("/audio/index.m3u8", "#EXT-X-VERSION:7", "#EXT-X-VERSION:3")},
		{"iframe.tag", edit("/video/iframes.m3u8", "#EXT-X-I-FRAMES-ONLY\n", "")},
		{"iframe.codecs", edit("/master.m3u8", `CODECS="avc1.42c00a",RESOLUTION`, `CODECS="hvc1.1.6.L93.90",RESOLUTION`)},
		{"webvtt.timestamp-map", edit("/subs/0.vtt", "X-TIMESTAMP-MAP=MPEGTS:0,LOCAL:00:00:00.000\n", "")},
		{"webvtt.header", edit("/subs/0.vtt", "WEBVTT", "1\n00:00:01,000")},
		{"http.content-type", func(m memory) {
			f := m["/video/seg.m4s"]
			f.contentType = "application/octet-stream"
			m["/video/seg.m4s"] = f
		}},
		{"fetch", func(m memory) { delete(m, "/audio/seg.m4s") }},
		{"multivariant.durations", edit("/audio/index.m3u8", "#EXTINF:1.000,\nseg.m4s\n",
			"#EXTINF:1.000,\nseg.m4s\n#EXTINF:1.000,\nseg.m4s\n#EXTINF:1.000,\nseg.m4s\n#EXTINF:1.000,\nseg.m4s\n")},
		{"fmp4.timeline", edit("/audio/index.m3u8", "#EXTINF:1.000,\nseg.m4s\n", "#EXTINF:1.000,\nseg.m4s\n#EXTINF:1.000,\nseg.m4s\n")},
	}
	for _, c := range cases {
		t.Run(c.rule, func(t *testing.T) {
			m := presentation(t)
			c.change(m)
			got := append(rules(t, m, SeverityError), rules(t, m, SeverityWarning)...)
			if !slices.Contains(got, c.rule) {
				t.Errorf("rule %s not reported; got %v", c.rule, got)
			}
		})
	}
}

func TestBitrates(t *testing.T) {
	// a 2 MB segment among 1 MB ones peaks over the 0.5-1.5 target window
	durations := []float64{6, 6, 6, 6}
	sizes := []float64{750_000, 1_500_000, 750_000, 750_000}
	peak, average := Bitrates(durations, sizes, 6)
	if peak != 2_000_000 {
		t.Errorf("peak %.0f, want 2000000", peak)
	}
	if average != 1_250_000 {
		t.Errorf("average %.0f, want 1250000", average)
	}
}

func TestWebVTTSegment(t *testing.T) {
	v, err := ParseWebVTT("WEBVTT\n\nSTYLE\n::cue { color: red }\n\n1\n00:00:05.500 --> 00:00:07.000 align:start\nAcross\nlines\n\nNOTE skipped\n\n00:00:20.000 --> 00:00:21.000\nLater\n")
	if err != nil {
		t.Fatal(err)
	}
	seg := v.Segment(6, 12, 126000)
	for _, want := range []string{"X-TIMESTAMP-MAP=MPEGTS:126000,LOCAL:00:00:00.000", "::cue", "1\n00:00:05.500 --> 00:00:07.000 align:start\nAcross\nlines"} {
		if !strings.Contains(seg, want) {
			t.Errorf("segment lacks %q:\n%s", want, seg)
		}
	}
	if strings.Contains(seg, "Later") {
		t.Errorf("segment holds a cue from outside it:\n%s", seg)
	}
}

func TestPlaylistRoundTrip(t *testing.T) {
	m := &Multivariant{Version: 7, IndependentSegments: true,
		Renditions: []Rendition{{Type: "AUDIO", GroupID: "a", Name: "English", Language: "en", Default: true, Autoselect: true, Channels: "16/JOC", URI: "a.m3u8"}},
		Variants: []Variant{{Bandwidth: 2, AverageBandwidth: 1, Codecs: []string{"hvc1.2.4.L120.90", "ec-3"}, SupplementalCodecs: "dvh1.08.06/db1p",
			Width: 1920, Height: 1080, FrameRate: 23.976, VideoRange: "PQ", Audio: "a", ClosedCaptions: "NONE", URI: "v.m3u8"}},
	}
	pl, err := Parse(m.String())
	if err != nil {
		t.Fatal(err)
	}
	got := pl.Multivariant
	if got.Variants[0].SupplementalCodecs != "dvh1.08.06/db1p" || got.Renditions[0].Channels != "16/JOC" ||
		!slices.Equal(got.Variants[0].Codecs, m.Variants[0].Codecs) || got.Variants[0].FrameRate != 23.976 {
		t.Errorf("round trip lost attributes:\n%s", m.String())
	}
}
