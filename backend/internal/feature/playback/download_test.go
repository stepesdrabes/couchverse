package playback

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"couchverse/internal/media"
)

// file turns a decision sample into the media file row a download reads.
func file(s Source) (*media.MediaFile, []media.AudioStream) {
	return &media.MediaFile{
		Container: s.Container, VideoCodec: s.VideoCodec, Video: s.Video, ProbeVersion: media.ProbeVersion,
		Width: s.Width, Height: s.Height, Bitrate: s.Bitrate, DurationSeconds: 60,
	}, s.Audio
}

func TestPlanDownload(t *testing.T) {
	newFFmpeg := Features{Major: 7, DolbyVision: true, ToneMap: true}
	h264At720 := with(h264MP4, func(s *Source) { s.Width, s.Height, s.Bitrate = 1280, 720, 2_000_000 })
	czech := with(hevcHDR10MKV, func(s *Source) { s.Audio[1].Lang = "cze"; s.Audio[0].Lang = "eng" })
	cases := []struct {
		name    string
		profile DeviceProfile
		source  Source
		quality string
		langs   []string
		spec    string
		height  int
	}{
		{"HDR10 HEVC stays as is for an HDR device", appleTV, hevcHDR10MKV, QualityOriginal, nil, "v=copy;a=1:copy", 0},
		{"a browser gets the top rung and AAC 5.1", chrome, hevcHDR10MKV, QualityOriginal, nil, "v=h264:1080:6000000;a=1:aac6", 1080},
		{"an SDR phone gets HDR as SDR and stereo", sdrPhone, hevcHDR10MKV, QualityOriginal, nil, "v=h264:1080:6000000;a=1:aac2", 1080},
		{"a lower rung transcodes", appleTV, h264MP4, "720p", nil, "v=h264:720:3000000;a=1:copy", 720},
		{"a rung above the source copies it", appleTV, h264At720, "1080p", nil, "v=copy;a=1:copy", 0},
		{"a rung never upscales", appleTV, vp9WebM, "1080p", nil, "v=h264:1080:3000000;a=1:aac2", 1080},
		{"a small source keeps its height", chrome, with(vp9WebM, func(s *Source) { s.Height = 601 }), "720p", nil, "v=h264:600:3000000;a=1:aac2", 600},
		{"the rung bitrate is capped at the source", chrome, with(vp9WebM, func(s *Source) { s.Bitrate = 1_000_000 }), "720p", nil, "v=h264:720:1000000;a=1:aac2", 720},
		{"requested languages in order", appleTV, czech, QualityOriginal, []string{"cs", "en"}, "v=copy;a=2:copy,1:copy", 0},
		{"an unknown language falls back to the default track", appleTV, czech, QualityOriginal, []string{"de"}, "v=copy;a=1:copy", 0},
		{"Dolby Vision 7 copies its HDR10 base layer", appleTV, dv7MKV, QualityOriginal, nil, "v=copy;a=1:aac6", 0},
	}
	for _, c := range cases {
		mf, tracks := file(c.source)
		plan, err := PlanDownload(c.profile, mf, tracks, c.quality, c.langs, newFFmpeg)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := plan.Spec(); got != c.spec {
			t.Errorf("%s: spec %s, want %s", c.name, got, c.spec)
		}
		if !plan.Copy && plan.Rendition.Height != c.height {
			t.Errorf("%s: height %d, want %d", c.name, plan.Rendition.Height, c.height)
		}
	}
}

func TestPlanDownloadRefusals(t *testing.T) {
	mf, tracks := file(hevcSDRMP4)
	noH264 := DeviceProfile{Video: []VideoSupport{{Codec: "av1"}}, Audio: []AudioSupport{{Codec: "aac"}}}
	if _, err := PlanDownload(noH264, mf, tracks, QualityOriginal, nil, Features{}); !errors.Is(err, ErrNoDownload) {
		t.Errorf("a device decoding neither the source nor H.264: %v", err)
	}
	if _, err := PlanDownload(appleTV, mf, tracks, "4320p", nil, Features{}); err == nil {
		t.Error("an unknown quality was planned")
	}
	// without dvh1 a copied profile 5 stream plays nowhere
	mf, tracks = file(dv5MP4)
	plan, err := PlanDownload(appleTV, mf, tracks, QualityOriginal, nil, Features{Major: 5})
	if err != nil || plan.Copy {
		t.Errorf("Dolby Vision 5 on an old ffmpeg: %+v %v", plan, err)
	}
}

func TestDownloadArgs(t *testing.T) {
	mf, tracks := file(with(hevcHDR10MKV, func(s *Source) { s.Audio[0].Lang, s.Audio[0].Title = "eng", "Atmos" }))
	plan, err := PlanDownload(chrome, mf, tracks, "720p", nil, Features{ToneMap: true})
	if err != nil {
		t.Fatal(err)
	}
	subs := []DownloadSubtitle{{Path: "/s/en.vtt", Lang: "en", Label: "English"}, {Path: "/s/cs.vtt", Lang: "cs", Forced: true}}
	args := strings.Join(DownloadArgs(mf, "/in.mkv", plan, subs, "/out.mp4", "libx264", "veryfast", Features{ToneMap: true}), " ")
	for _, want := range []string{
		"-i /in.mkv -i /s/en.vtt -i /s/cs.vtt",
		"-map 0:v:0 -vf scale=-2:720,zscale",
		"-c:v libx264 -preset veryfast -profile:v high",
		"-map 0:1 -c:a:0 aac -ac:a:0 6 -b:a:0 384k -metadata:s:a:0 language=eng -metadata:s:a:0 title=Atmos -disposition:a:0 default",
		"-map 1:0 -c:s:0 mov_text -metadata:s:s:0 language=eng -disposition:s:0 0 -metadata:s:s:0 title=English",
		"-map 2:0 -c:s:1 mov_text -metadata:s:s:1 language=ces -disposition:s:1 forced",
		"-movflags +faststart -f mp4 /out.mp4",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q:\n%s", want, args)
		}
	}

	copied, err := PlanDownload(appleTV, mf, tracks, QualityOriginal, nil, Features{})
	if err != nil {
		t.Fatal(err)
	}
	args = strings.Join(DownloadArgs(mf, "/in.mkv", copied, nil, "/out.mp4", "libx264", "veryfast", Features{}), " ")
	if !strings.Contains(args, "-map 0:v:0 -c:v copy -tag:v hvc1") || !strings.Contains(args, "-c:a:0 copy") {
		t.Errorf("a copy re-encodes:\n%s", args)
	}
	if slices.Contains(strings.Fields(args), "-vf") {
		t.Errorf("a copy filters:\n%s", args)
	}
}

func TestISO639(t *testing.T) {
	for in, want := range map[string]string{"en": "eng", "eng": "eng", "cze": "ces", "cs": "ces", "": "und", "xx-nope": "und"} {
		if got := media.ISO639(in); got != want {
			t.Errorf("ISO639(%q) = %q, want %q", in, got, want)
		}
	}
}
