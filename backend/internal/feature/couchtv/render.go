package couchtv

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// Job types of the rendering pipeline (plan 6.3). The app composes breaks; the render
// sidecar (`couchverse render`) registers the other two, which the app never does.
const (
	ComposeJob  = "couchtv_compose"
	RenderJob   = "couchtv_render"
	AssembleJob = "couchtv_assemble"
)

// RenderPayload names the clip a couchtv_render job renders.
type RenderPayload struct {
	ClipID string `json:"clipId"`
}

// AssemblePayload names the break a couchtv_assemble job assembles.
type AssemblePayload struct {
	BreakID string `json:"breakId"`
}

// rendererKey is the settings key where the sidecar publishes its bundle's template versions
// (the composer keys clips with them) and when it last worked.
const rendererKey = "couchtv_render"

// RendererState is what the sidecar publishes.
type RendererState struct {
	Versions map[string]int `json:"versions"`
	SeenAt   time.Time      `json:"seenAt"`
}

func loadRendererState(ctx context.Context, st *settings.Store) (RendererState, bool) {
	var s RendererState
	raw, err := st.Get(ctx, rendererKey)
	if err != nil || raw == nil || json.Unmarshal(raw, &s) != nil || len(s.Versions) == 0 {
		return s, false
	}
	return s, true
}

// The package layout of a rendered clip or break under DATA_DIR, like a Playback v2
// package (playback.ReadPackageDir reads it): the video in source/, the sound as AAC
// stereo in audio-1-aac/, each with its rendition.json, and a master.m3u8 over both.
const (
	videoDir = "source"
	audioDir = "audio-1-aac"
	// samplesPerFrame is the render's 48 kHz audio per 25 fps frame.
	samplesPerFrame = 48000 / FPS
	// partFrames bounds one audio render of a long break: the page holds a part's whole
	// sound twice over (the offline render and its WAV), which must stay small next to the
	// sidecar's 1 GB.
	partFrames = 120 * FPS
)

// videoCodec is every clip's encoding (D16): H.264 High at level 4.0 from the captured
// JPEGs (full-range BT.601) to limited-range BT.709, an IDR every 2 s and at the first
// frame, so clips concatenate by stream copy and a break segments every 6 s.
var videoCodec = []string{"-map", "0:v",
	"-vf", "scale=in_range=full:out_range=tv:in_color_matrix=bt601:out_color_matrix=bt709,format=yuv420p",
	"-c:v", "libx264", "-preset", "veryfast", "-tune", "animation", "-crf", "18",
	"-profile:v", "high", "-level:v", "4.0", "-g", "50", "-keyint_min", "50", "-sc_threshold", "0",
	"-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709", "-color_range", "tv"}

// audioCodec is every clip's and break's sound: AAC stereo, 192 kb/s (D16).
var audioCodec = []string{"-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "48000"}

// Renderer is the render sidecar: one headless Chromium driven over CDP renders clips
// frame by frame (6.7) and assembles breaks, one job at a time.
type Renderer struct {
	Store    *Store
	Jobs     *jobs.Store
	Settings *settings.Store
	Artwork  *artwork.Service
	DataDir  string
	FFmpeg   string
	// Chromium is the browser binary; empty finds one the way chromedp does.
	Chromium string
	// Bundle is the broadcast package's built render page (render.html and its assets).
	Bundle fs.FS
	// RestartEvery opens a fresh Chromium after that many jobs, which bounds its leaks
	// (decoded artwork is cached for the page's life).
	RestartEvery int

	server  *mediaServer
	browser *browser
	jobs    int
}

// Start serves the bundle, opens Chromium, publishes the template versions and asks the
// app to compose, so breaks start rendering as soon as a sidecar runs.
func (r *Renderer) Start(ctx context.Context) error {
	if _, err := fs.Stat(r.Bundle, "render.html"); err != nil {
		return errors.New("the broadcast bundle is not built (make build)")
	}
	r.server = &mediaServer{bundle: r.Bundle, dataDir: r.DataDir, ffmpeg: r.FFmpeg, store: r.Store, art: r.Artwork}
	if err := r.server.start(); err != nil {
		return err
	}
	b, err := r.openBrowser()
	if err != nil {
		return err
	}
	versions, err := b.versions(ctx)
	if err != nil {
		return err
	}
	if err := r.publish(ctx, versions); err != nil {
		return err
	}
	_, err = r.Jobs.EnqueueJobOnce(ctx, ComposeJob, struct{}{}, jobs.EnqueueOpts{})
	return err
}

// Close stops Chromium and the server.
func (r *Renderer) Close() {
	if r.browser != nil {
		r.browser.close()
	}
	if r.server != nil {
		r.server.close()
	}
}

func (r *Renderer) publish(ctx context.Context, versions map[string]int) error {
	if versions == nil {
		state, _ := loadRendererState(ctx, r.Settings)
		versions = state.Versions
	}
	body, err := json.Marshal(RendererState{Versions: versions, SeenAt: time.Now()})
	if err != nil {
		return err
	}
	return r.Settings.Set(ctx, rendererKey, body)
}

// openBrowser is the page to work in: the open one, or a fresh one when there is none or
// it has done its share of jobs.
func (r *Renderer) openBrowser() (*browser, error) {
	if r.browser != nil && (r.RestartEvery <= 0 || r.jobs < r.RestartEvery) {
		r.jobs++
		return r.browser, nil
	}
	r.dropBrowser()
	b := &browser{execPath: r.Chromium}
	if err := b.open(r.server.url("/render.html")); err != nil {
		return nil, err
	}
	r.browser, r.jobs = b, 1
	return b, nil
}

// dropBrowser closes the browser after a failure, so the next job starts afresh.
func (r *Renderer) dropBrowser() {
	if r.browser != nil {
		r.browser.close()
		r.browser = nil
	}
}

// Render is the couchtv_render job: one clip, frame by frame.
func (r *Renderer) Render(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p RenderPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	clip, err := r.Store.Clip(ctx, p.ClipID)
	if errors.Is(err, db.ErrNotFound) {
		return nil // pruned since
	}
	if err != nil {
		return err
	}
	if clip.State != ClipReady {
		if err := r.Store.setClipState(ctx, clip.ID, ClipRendering, ""); err != nil {
			return err
		}
		start := time.Now()
		frames, version, err := r.renderClip(ctx, clip, report)
		if err != nil {
			r.dropBrowser()
			if ctx.Err() == nil {
				r.Store.setClipState(context.WithoutCancel(ctx), clip.ID, ClipFailed, err.Error())
			}
			return fmt.Errorf("clip %s (%s): %w", clip.ID, clip.Kind, err)
		}
		rel := filepath.Join("couchtv", "clips", clip.Key)
		size := media.DirSize(filepath.Join(r.DataDir, rel))
		took := time.Since(start)
		if err := r.Store.clipRendered(ctx, clip.ID, frames, rel, size, version, took); err != nil {
			return err
		}
		slog.Info("couchtv: rendered a clip", "kind", clip.Kind, "frames", frames, "took", took.Round(time.Millisecond),
			"fps", math.Round(float64(frames)/took.Seconds()*100)/100)
	}
	if err := r.publish(ctx, nil); err != nil {
		return err
	}
	ready, err := r.Store.assemblable(ctx, clip.ID)
	if err != nil {
		return err
	}
	for _, b := range ready {
		if err := queueAssembly(ctx, r.Jobs, b); err != nil {
			return err
		}
	}
	return nil
}

// queueAssembly asks the sidecar to assemble a break, the ones that air soonest first
// (the evergreen break before all).
func queueAssembly(ctx context.Context, js *jobs.Store, b Break) error {
	_, err := js.EnqueueJobOnce(ctx, AssembleJob, AssemblePayload{BreakID: b.ID}, jobs.EnqueueOpts{Priority: urgency(b.StartsAt)})
	return err
}

// urgency is a job priority that puts what airs sooner first (the queue claims the highest
// first); zero, the highest, for what airs any time.
func urgency(airs time.Time) int {
	if airs.IsZero() {
		return 0
	}
	return -int(airs.Unix() / 60)
}

// renderClip renders a clip into its package: every frame captured and piped to the
// encoder (a frame identical to the one before it is reused without a capture), then its
// sound.
func (r *Renderer) renderClip(ctx context.Context, clip *Clip, report func(int)) (frames, version int, err error) {
	if err := r.server.prepare(ctx, clip.Spec); err != nil {
		return 0, 0, err
	}
	b, err := r.openBrowser()
	if err != nil {
		return 0, 0, err
	}
	loaded, err := b.load(ctx, clip.Spec)
	if err != nil {
		return 0, 0, err
	}
	frames = framesOf(loaded.Duration)
	if clip.Frames > 0 && frames != clip.Frames {
		return 0, 0, fmt.Errorf("the clip is %d frames, the break counted on %d", frames, clip.Frames)
	}
	if len(loaded.Missing) > 0 {
		slog.Warn("couchtv: a clip renders without some media", "clipId", clip.ID, "missing", loaded.Missing)
	}

	work, err := r.workDir("clip")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(work)
	encoder, err := r.startEncoder(ctx, work)
	if err != nil {
		return 0, 0, err
	}
	var last []byte
	for i := range frames {
		changed, err := b.frame(ctx, i)
		if err == nil && (changed || last == nil) {
			last, err = b.capture(ctx)
		}
		if err == nil {
			_, err = encoder.stdin.Write(last)
		}
		if err != nil {
			encoder.abort()
			return 0, 0, fmt.Errorf("frame %d: %w", i, err)
		}
		if i%25 == 0 {
			report(i * 90 / frames)
		}
	}
	if err := encoder.finish(); err != nil {
		return 0, 0, err
	}

	raw := filepath.Join(work, "sound.f32")
	if err := r.renderSound(ctx, b, nil, raw, frames); err != nil {
		return 0, 0, err
	}
	if err := r.encodeSound(ctx, raw, work); err != nil {
		return 0, 0, err
	}
	if err := describe(work, frames); err != nil {
		return 0, 0, err
	}
	return frames, loaded.Version, r.place(work, filepath.Join("couchtv", "clips", clip.Key))
}

func (r *Renderer) workDir(kind string) (string, error) {
	dir := filepath.Join(r.DataDir, "couchtv", "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, kind+"-")
}

// place moves a finished package to its place under DATA_DIR, replacing an older one.
func (r *Renderer) place(work, rel string) error {
	dst := filepath.Join(r.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	os.RemoveAll(dst)
	return os.Rename(work, dst)
}

// encoder is a running ffmpeg turning piped JPEG frames into the video rendition.
type encoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr strings.Builder
}

func (r *Renderer) startEncoder(ctx context.Context, work string) (*encoder, error) {
	args := playback.InputArgs("-", "-f", "image2pipe", "-framerate", fmt.Sprint(FPS), "-c:v", "mjpeg")
	args = append(args, playback.RenditionArgs(filepath.Join(work, videoDir), videoCodec, true)...)
	if err := os.MkdirAll(filepath.Join(work, videoDir), 0o755); err != nil {
		return nil, err
	}
	e := &encoder{cmd: exec.CommandContext(ctx, r.FFmpeg, args...)}
	e.cmd.Stderr = &e.stderr
	var err error
	if e.stdin, err = e.cmd.StdinPipe(); err != nil {
		return nil, err
	}
	return e, e.cmd.Start()
}

func (e *encoder) finish() error {
	e.stdin.Close()
	if err := e.cmd.Wait(); err != nil {
		return fmt.Errorf("encode: %v: %s", err, strings.TrimSpace(e.stderr.String()))
	}
	return nil
}

func (e *encoder) abort() {
	e.stdin.Close()
	e.cmd.Process.Kill()
	e.cmd.Wait()
}

// renderSound renders the loaded clip's sound, or each part of a break's (parts loaded in
// turn), and writes it to raw as interleaved 32-bit float samples, checked against frames.
func (r *Renderer) renderSound(ctx context.Context, b *browser, parts []BreakSpec, raw string, frames int) error {
	out, err := os.Create(raw)
	if err != nil {
		return err
	}
	defer out.Close()
	render := func() error {
		wav := raw + ".wav"
		defer os.Remove(wav)
		if err := b.renderAudio(ctx, "/upload/"+r.server.expect(wav)); err != nil {
			return err
		}
		return appendSamples(out, wav)
	}
	if parts == nil {
		if err := render(); err != nil {
			return err
		}
	}
	for _, part := range parts {
		spec, err := json.Marshal(part)
		if err != nil {
			return err
		}
		if _, err := b.load(ctx, spec); err != nil {
			return err
		}
		if err := render(); err != nil {
			return err
		}
	}
	info, err := out.Stat()
	if err != nil {
		return err
	}
	if want := int64(frames) * samplesPerFrame * 2 * 4; info.Size() != want {
		return fmt.Errorf("the sound is %d bytes, %d frames take %d", info.Size(), frames, want)
	}
	return nil
}

// appendSamples appends a render's samples (a 32-bit float stereo WAV with the plain
// 44-byte header the broadcast package writes) to out.
func appendSamples(out io.Writer, wav string) error {
	f, err := os.Open(wav)
	if err != nil {
		return err
	}
	defer f.Close()
	var head [44]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return err
	}
	if string(head[36:40]) != "data" || binary.LittleEndian.Uint16(head[20:]) != 3 || binary.LittleEndian.Uint16(head[22:]) != 2 {
		return errors.New("the page's sound is not a stereo float WAV")
	}
	_, err = io.CopyN(out, f, int64(binary.LittleEndian.Uint32(head[40:])))
	return err
}

// encodeSound encodes raw samples into the package's audio rendition.
func (r *Renderer) encodeSound(ctx context.Context, raw, work string) error {
	dir := filepath.Join(work, audioDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	args := append(playback.InputArgs(raw, "-f", "f32le", "-ar", "48000", "-ac", "2"),
		playback.RenditionArgs(dir, append([]string{"-map", "0:a"}, audioCodec...), false)...)
	if out, err := exec.CommandContext(ctx, r.FFmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("encode sound: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// describe writes the renditions' rendition.json and the master playlist, checking that
// the video is exactly frames long.
func describe(work string, frames int) error {
	video, err := playback.DescribeRendition(filepath.Join(work, videoDir), playback.RenditionInfo{FrameRate: FPS, VideoRange: "SDR"})
	if err != nil {
		return err
	}
	if math.Abs(video.Duration-frameSeconds(frames)) > 0.001 {
		return fmt.Errorf("the video is %.3fs, %d frames take %.3fs", video.Duration, frames, frameSeconds(frames))
	}
	audio, err := playback.DescribeRendition(filepath.Join(work, audioDir), playback.RenditionInfo{StreamIndex: 1, Language: "en", Name: "Couch TV", Default: true})
	if err != nil {
		return err
	}
	m := &hls.Multivariant{Version: 7, IndependentSegments: true,
		Renditions: []hls.Rendition{{Type: "AUDIO", GroupID: "audio", Name: audio.Name, Language: audio.Language,
			Default: true, Autoselect: true, Channels: audio.Channels, URI: audioDir + "/index.m3u8"}},
		Variants: []hls.Variant{{Bandwidth: video.Peak + audio.Peak, AverageBandwidth: video.Average + audio.Average,
			Codecs: []string{video.Codec, audio.Codec}, Width: video.Width, Height: video.Height, FrameRate: FPS,
			VideoRange: "SDR", Audio: "audio", ClosedCaptions: "NONE", URI: videoDir + "/index.m3u8"}}}
	return os.WriteFile(filepath.Join(work, "master.m3u8"), []byte(m.String()), 0o644)
}

// Assemble is the couchtv_assemble job: a break's sound in one render (in parts when it is
// long), its clips' video joined by stream copy, both segmented into the break's package.
func (r *Renderer) Assemble(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p AssemblePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	b, err := r.Store.Break(ctx, p.BreakID)
	if errors.Is(err, db.ErrNotFound) {
		return nil // replanned since
	}
	if err != nil || b.State == BreakReady {
		return err
	}
	clips := make([]*Clip, len(b.Recipe.Clips))
	for i, rc := range b.Recipe.Clips {
		c, err := r.Store.Clip(ctx, rc.ClipID)
		if err != nil {
			return err
		}
		if c.State != ClipReady {
			return nil // the clip's render queues the assembly again
		}
		if c.Frames != rc.Frames {
			return r.failBreak(ctx, b, fmt.Errorf("clip %s is %d frames, the recipe counted on %d", c.ID, c.Frames, rc.Frames))
		}
		clips[i] = c
	}
	if err := r.Store.setBreakState(ctx, b.ID, BreakRendering, ""); err != nil {
		return err
	}
	if err := r.assemble(ctx, b, clips, report); err != nil {
		r.dropBrowser()
		if ctx.Err() == nil {
			r.failBreak(context.WithoutCancel(ctx), b, err)
		}
		return fmt.Errorf("break %s: %w", b.ID, err)
	}
	rel := filepath.Join("couchtv", "breaks", b.ID)
	if err := r.Store.breakAssembled(ctx, b.ID, rel, b.Recipe.Frames, media.DirSize(filepath.Join(r.DataDir, rel))); err != nil {
		return err
	}
	slog.Info("couchtv: assembled a break", "breakId", b.ID, "airingId", b.AiringID, "frames", b.Recipe.Frames, "clips", len(clips))
	return r.publish(ctx, nil)
}

func (r *Renderer) failBreak(ctx context.Context, b *Break, err error) error {
	slog.Warn("couchtv: a break failed to assemble", "breakId", b.ID, "err", err)
	return r.Store.setBreakState(ctx, b.ID, BreakFailed, err.Error())
}

func (r *Renderer) assemble(ctx context.Context, b *Break, clips []*Clip, report func(int)) error {
	work, err := r.workDir("break")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	br, err := r.openBrowser()
	if err != nil {
		return err
	}
	raw := filepath.Join(work, "sound.f32")
	if err := r.renderSound(ctx, br, soundParts(b.Recipe.Sound), raw, b.Recipe.Frames); err != nil {
		return err
	}
	report(40)

	// each clip's video as one fragmented MP4 (its init and segments end to end), joined
	// by the concat demuxer at exact lengths
	var list strings.Builder
	list.WriteString("ffconcat version 1.0\n")
	for i, c := range clips {
		joined := filepath.Join(work, fmt.Sprintf("clip%03d.mp4", i))
		if err := joinSegments(filepath.Join(r.DataDir, c.Path, videoDir), joined); err != nil {
			return fmt.Errorf("clip %s: %w", c.ID, err)
		}
		fmt.Fprintf(&list, "file '%s'\nduration %.3f\n", filepath.Base(joined), frameSeconds(c.Frames))
	}
	listPath := filepath.Join(work, "clips.ffconcat")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}
	out := filepath.Join(work, "package")
	for _, dir := range []string{videoDir, audioDir} {
		if err := os.MkdirAll(filepath.Join(out, dir), 0o755); err != nil {
			return err
		}
	}
	args := playback.InputArgs(listPath, "-f", "concat", "-safe", "0")
	args = append(args, "-f", "f32le", "-ar", "48000", "-ac", "2", "-i", raw)
	args = append(args, playback.RenditionArgs(filepath.Join(out, videoDir), []string{"-map", "0:v", "-c:v", "copy"}, true)...)
	args = append(args, playback.RenditionArgs(filepath.Join(out, audioDir), append([]string{"-map", "1:a"}, audioCodec...), false)...)
	if o, err := exec.CommandContext(ctx, r.FFmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("mux: %v: %s", err, strings.TrimSpace(string(o)))
	}
	report(90)
	if err := describe(out, b.Recipe.Frames); err != nil {
		return err
	}
	return r.place(out, filepath.Join("couchtv", "breaks", b.ID))
}

// joinSegments writes a rendition's init section and segments as one fragmented MP4.
func joinSegments(dir, dst string) error {
	text, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		return err
	}
	pl, err := hls.Parse(string(text))
	if err != nil || pl.Media == nil {
		return fmt.Errorf("%s: not a media playlist", dir)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	parts := []string{"init.mp4"}
	for _, s := range pl.Media.Segments {
		parts = append(parts, s.URI)
	}
	for _, name := range parts {
		in, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	return out.Close()
}

// soundParts splits a break's sound into renders that start at most partFrames apart,
// each cut where a clip's sound starts (only the outgoing clip's ring-out is lost there).
func soundParts(s BreakSpec) []BreakSpec {
	var parts []BreakSpec
	from := 0 // where the current part starts in the break
	for _, item := range s.Items {
		at := framesOf(item.At)
		if len(parts) == 0 || at-from >= partFrames {
			if len(parts) > 0 {
				from = at
			}
			parts = append(parts, BreakSpec{Template: s.Template, Loudness: s.Loudness})
		}
		item.At = frameSeconds(at - from)
		parts[len(parts)-1].Items = append(parts[len(parts)-1].Items, item)
	}
	return parts
}
