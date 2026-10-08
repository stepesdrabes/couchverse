package playback

import (
	"slices"
	"strings"
	"testing"

	"couchverse/internal/feature/library"
	"couchverse/internal/hls"
	"couchverse/internal/media"
)

func rung(dir string, height int, codec string, peak int64) rendition {
	return rendition{dir: dir, info: renditionInfo{Codec: codec, Width: height * 16 / 9, Height: height,
		FrameRate: 23.976, VideoRange: "SDR", Peak: peak, Average: peak * 9 / 10}}
}

func audioRend(dir string, stream int, codec, channels, lang, name string, def bool, peak int64) rendition {
	return rendition{dir: dir, info: renditionInfo{Codec: codec, Channels: channels, StreamIndex: stream,
		Language: lang, Name: name, Default: def, Peak: peak, Average: peak}}
}

func renditionSet() fileRenditions {
	surround := audioRend("audio-1-eac3", 1, "ec-3", "16/JOC", "en", "English", true, 768_000)
	return fileRenditions{
		source: &rendition{dir: "source", info: renditionInfo{Codec: "hvc1.2.4.L150.90", SupplementalCodec: "dvh1.08.06/db1p",
			Width: 3840, Height: 2160, FrameRate: 24, VideoRange: "PQ", Peak: 25_000_000, Average: 18_000_000}},
		ladder: []rendition{rung("480p", 480, "avc1.64001e", 1_300_000), rung("1080p", 1080, "avc1.640028", 6_500_000),
			rung("720p", 720, "avc1.64001f", 3_300_000)},
		audio: []audioTrack{
			{stereo: audioRend("audio-1-aac", 1, "mp4a.40.2", "2", "en", "English", true, 170_000), surround: &surround},
			{stereo: audioRend("audio-2-aac", 2, "mp4a.40.2", "2", "cs", "", false, 170_000)},
		},
		trickplay: &rendition{dir: "trickplay", info: renditionInfo{Codec: "avc1.64000d", Width: 320, Height: 180, Peak: 40_000, Average: 30_000}},
	}
}

func TestBuildMasterOriginal(t *testing.T) {
	subs := []media.Subtitle{{ID: "s1", Lang: "eng", Label: "English"}, {ID: "s2", Lang: "cs", Label: "English", Forced: true}}
	pl, err := buildMaster(renditionSet(), subs, Master{Video: "original", Surround: []string{"eac3"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Variants) != 2 {
		t.Fatalf("variants %d, want the source once per audio group", len(pl.Variants))
	}
	surround, stereo := pl.Variants[0], pl.Variants[1]
	if surround.Audio != "surround" || stereo.Audio != "stereo" {
		t.Errorf("groups %s, %s: surround should come first", surround.Audio, stereo.Audio)
	}
	// the Czech track has no surround rendition, so its stereo one joins the group
	if !slices.Equal(surround.Codecs, []string{"hvc1.2.4.L150.90", "ec-3", "mp4a.40.2"}) {
		t.Errorf("surround codecs %v", surround.Codecs)
	}
	if surround.Bandwidth != 25_000_000+768_000 || surround.SupplementalCodecs != "dvh1.08.06/db1p" || surround.VideoRange != "PQ" {
		t.Errorf("surround variant %+v", surround)
	}
	text := pl.String()
	for _, want := range []string{
		`GROUP-ID="surround",NAME="English",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="16/JOC",URI="audio-1-eac3/index.m3u8"`,
		`GROUP-ID="surround",NAME="Čeština",LANGUAGE="cs",DEFAULT=NO,AUTOSELECT=YES,CHANNELS="2",URI="audio-2-aac/index.m3u8"`,
		// two tracks labelled alike stay distinguishable
		`TYPE=SUBTITLES,GROUP-ID="subs",NAME="English 2",LANGUAGE="cs",DEFAULT=NO,AUTOSELECT=YES,FORCED=YES,URI="subtitles/s2/index.m3u8"`,
		`#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=40000,AVERAGE-BANDWIDTH=30000,CODECS="avc1.64000d",RESOLUTION=320x180,VIDEO-RANGE=SDR,URI="trickplay/iframes.m3u8"`,
		"#EXT-X-INDEPENDENT-SEGMENTS",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("playlist lacks %s\n%s", want, text)
		}
	}
}

func TestBuildMasterLadder(t *testing.T) {
	pl, err := buildMaster(renditionSet(), nil, Master{Video: "ladder"})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, v := range pl.Variants {
		order = append(order, v.URI)
		if v.Audio != "stereo" || v.Subtitles != "" {
			t.Errorf("variant %s: audio %q subtitles %q", v.URI, v.Audio, v.Subtitles)
		}
	}
	// AVPlayer starts with the first variant, so the one nearest 720p leads
	want := []string{"720p/index.m3u8", "1080p/index.m3u8", "480p/index.m3u8"}
	if !slices.Equal(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
	if _, err := hls.Parse(pl.String()); err != nil {
		t.Error(err)
	}
}

func TestLegacyMaster(t *testing.T) {
	mf := &media.MediaFile{Width: 1920, Height: 1080, Bitrate: 8_000_000}
	variants := []library.TranscodeVariant{
		{Name: "source", Mode: "copy", Status: "ready", Format: "ts"},
		{Name: "720p", Mode: "transcode", Height: 720, VideoBitrate: 3_000_000, AudioBitrate: 128_000, Status: "ready", Format: "ts"},
		{Name: "multiaudio", Mode: "copy", Status: "ready", Format: "ts"},
		{Name: "480p", Mode: "transcode", Height: 480, Status: "queued", Format: "ts"},
		{Name: "1080p", Mode: "transcode", Height: 1080, Status: "ready", Format: "fmp4"},
	}
	text := legacyMaster(mf, variants)
	for _, want := range []string{"RESOLUTION=1920x1080,NAME=\"source\"\nsource/index.m3u8", "BANDWIDTH=3128000,RESOLUTION=1280x720"} {
		if !strings.Contains(text, want) {
			t.Errorf("legacy master lacks %q:\n%s", want, text)
		}
	}
	// the multiaudio remux has its own master; listing it here pointed at a missing playlist
	for _, absent := range []string{"multiaudio", "480p", "1080p"} {
		if strings.Contains(text, absent) {
			t.Errorf("legacy master lists %s:\n%s", absent, text)
		}
	}
}

func TestPreparedState(t *testing.T) {
	got := preparedState([]library.TranscodeVariant{
		{Name: "source", Status: "ready", Format: "fmp4"},
		{Name: "audio", Status: "processing", Format: "fmp4"},
		{Name: "720p", Status: "failed", Format: "fmp4"},
		{Name: "1080p", Status: "queued", Format: "fmp4"},
		{Name: "trickplay", Status: "ready", Format: "fmp4"},
		{Name: "multiaudio", Status: "ready", Format: "ts"},
	})
	want := Prepared{Original: PackageReady, Audio: PackagePending, Ladder: PackagePending, Legacy: PackageReady, LegacyMultiAudio: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// A scope film's rungs are 1920x800, 1280x532 and 854x354: the 800-line one is
// nearest 720 lines but is its 1080p rung, so the ladder starts at 1280x532.
func TestOrderLadderByClass(t *testing.T) {
	rung := func(width, height int) rendition {
		return rendition{info: renditionInfo{Width: width, Height: height}}
	}
	heights := []int{}
	for _, r := range orderLadder([]rendition{rung(854, 354), rung(1920, 800), rung(1280, 532)}) {
		heights = append(heights, r.info.Height)
	}
	if want := []int{532, 800, 354}; !slices.Equal(heights, want) {
		t.Errorf("ladder heights %v, want %v", heights, want)
	}
}
