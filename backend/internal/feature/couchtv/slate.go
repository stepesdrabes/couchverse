package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
)

// slateVersion names the slate's design; changing it makes the next start
// generate it again.
const slateVersion = 1

// Slate is the last resort, what airs when nothing else can and no standby card
// is rendered (a break not ready without an evergreen one, a program that cannot
// air, a hole in the schedule): a plain dark 1080p25 frame with the channel's
// name and silence. It is generated once with ffmpeg as one 2 s segment per track
// (H.264 without B-frames, one GOP, so it can be cut anywhere; silent AAC) and
// looped.
type Slate struct {
	dir    string
	ffmpeg string

	once sync.Once
	err  error
	fill fill
}

// NewSlate keeps the slate under dataDir/cache/couchtv/slate.
func NewSlate(dataDir, ffmpeg string) *Slate {
	return &Slate{dir: filepath.Join(dataDir, "cache", "couchtv", "slate"), ffmpeg: ffmpeg}
}

// nominalSlate is listed while the slate cannot be made (no ffmpeg): its files
// are missing.
var nominalSlate = fill{
	Video: &loop{durations: []float64{2}, info: playback.RenditionInfo{Codec: "avc1.640028", Width: 1920, Height: 1080, FrameRate: 25}},
	Audio: &loop{durations: []float64{2.021333}, info: playback.RenditionInfo{Codec: "mp4a.40.2", Channels: "2"}},
}

// Ensure makes the slate (or reads the one made before) once per process.
func (s *Slate) Ensure(ctx context.Context) error {
	s.once.Do(func() {
		if s.err = s.load(); s.err == nil {
			return
		}
		start := time.Now()
		if s.err = s.generate(ctx); s.err == nil {
			s.err = s.load()
		}
		if s.err != nil {
			slog.Error("couchtv: the slate could not be made", "err", s.err)
			return
		}
		slog.Info("couchtv: made the slate", "took", time.Since(start).Round(time.Millisecond))
	})
	return s.err
}

// Fill is the slate's video and audio to loop.
func (s *Slate) Fill() *fill {
	if s.Ensure(context.Background()) != nil {
		return &nominalSlate
	}
	return &s.fill
}

type slateMarker struct {
	Version int `json:"version"`
}

func (s *Slate) load() error {
	raw, err := os.ReadFile(filepath.Join(s.dir, "slate.json"))
	if err != nil {
		return err
	}
	var m slateMarker
	if json.Unmarshal(raw, &m) != nil || m.Version != slateVersion {
		return errors.New("an older slate")
	}
	for _, t := range []struct {
		name string
		loop **loop
	}{{"video", &s.fill.Video}, {"audio", &s.fill.Audio}} {
		l, err := openLoop(filepath.Join(s.dir, t.name), playback.RenditionInfo{})
		if err != nil {
			return err
		}
		if len(l.names) != 1 || len(l.in.Tracks) != 1 {
			return fmt.Errorf("couchtv: the slate's %s is not one segment of one track", t.name)
		}
		tr := l.in.Tracks[0]
		bits := l.bitrate()
		l.info = playback.RenditionInfo{Codec: tr.Codec, Peak: bits, Average: bits, Duration: l.seconds()}
		if tr.Handler == "vide" {
			l.info.Width, l.info.Height, l.info.FrameRate, l.info.VideoRange = tr.Width, tr.Height, 25, "SDR"
		} else {
			l.info.Channels = fmt.Sprint(tr.Channels)
		}
		*t.loop = l
	}
	return nil
}

// slateText draws the channel's name; ffmpeg builds without drawtext (no
// freetype) make the plain frame.
const slateText = ",drawtext=text='Couch TV':fontcolor=0xe8eaf2:fontsize=96:x=(w-text_w)/2:y=(h-text_h)/2"

func (s *Slate) generate(ctx context.Context) error {
	if err := os.RemoveAll(s.dir); err != nil {
		return err
	}
	for _, d := range []string{"video", "audio"} {
		if err := os.MkdirAll(filepath.Join(s.dir, d), 0o755); err != nil {
			return err
		}
	}
	// the packages' timeline: every rendition starts 1.4 s in
	output := func(dir string, codec ...string) []string {
		return append(codec, "-output_ts_offset", "1.4", "-f", "hls", "-hls_time", "4", "-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4", "-hls_segment_options", "movflags=+frag_discont+negative_cts_offsets",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", filepath.Join(s.dir, dir, "seg_%05d.m4s"), filepath.Join(s.dir, dir, "index.m3u8"))
	}
	run := func(text string) error {
		args := []string{"-hide_banner", "-loglevel", "error", "-y",
			"-f", "lavfi", "-i", "color=c=0x0d1017:s=1920x1080:r=25" + text,
			"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo"}
		args = append(args, output("video", "-map", "0:v", "-frames:v", "50", "-c:v", "libx264", "-preset", "veryfast",
			"-profile:v", "high", "-bf", "0", "-g", "50", "-keyint_min", "50", "-sc_threshold", "0", "-crf", "23",
			"-pix_fmt", "yuv420p", "-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709")...)
		args = append(args, output("audio", "-map", "1:a", "-t", "2", "-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000")...)
		out, err := exec.CommandContext(ctx, s.ffmpeg, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
		return nil
	}
	if err := run(slateText); err != nil {
		slog.Info("couchtv: the slate goes without its text", "err", err)
		if err := run(""); err != nil {
			return err
		}
	}
	marker, err := json.Marshal(slateMarker{Version: slateVersion})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "slate.json"), marker, 0o644)
}

// emptyWebVTT is a subtitle segment without cues, on the packages' timeline.
var emptyWebVTT = (&hls.WebVTT{}).Segment(0, 0, subtitleMPEGTS)
