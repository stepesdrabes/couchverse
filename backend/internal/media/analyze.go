package media

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// The measurements behind media analysis (library's markers, Couch TV's promo picks):
// ffmpeg reads a stretch of a file and hands back numbers. Every run is background work
// (BackgroundCmd).

// FingerprintItem is the span of one chromaprint item in seconds: algorithm 1 hashes
// 4096-sample frames at 11025 Hz, a third of a frame apart.
const FingerprintItem = 4096.0 / 3 / 11025

// LoudnessStep is the spacing of momentary loudness values.
const LoudnessStep = 0.1

func analysisCmd(ctx context.Context, ffmpegPath string, args ...string) *exec.Cmd {
	return BackgroundCmd(ctx, ffmpegPath, append([]string{"-hide_banner", "-nostdin"}, args...)...)
}

// seekArgs read [from, from+length) of file; a zero length reads to the end. The seek
// is passed even at 0, so output times count from the file's start whatever its first
// timestamp.
func seekArgs(file string, from, length float64) []string {
	args := []string{"-ss", strconv.FormatFloat(from, 'f', 3, 64)}
	if length > 0 {
		args = append(args, "-t", strconv.FormatFloat(length, 'f', 3, 64))
	}
	return append(args, "-i", file)
}

func analysisError(ctx context.Context, err error, stderr string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(stderr))
}

var (
	chromaprintMu sync.Mutex
	chromaprint   = map[string]bool{}
)

// HasChromaprint reports whether ffmpeg was built with the chromaprint muxer (Debian's
// is; Homebrew's is not), checking once per ffmpeg.
func HasChromaprint(ffmpegPath string) bool {
	chromaprintMu.Lock()
	defer chromaprintMu.Unlock()
	has, ok := chromaprint[ffmpegPath]
	if !ok {
		out, _ := exec.Command(ffmpegPath, "-hide_banner", "-muxers").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if f := strings.Fields(line); len(f) >= 2 && f[1] == "chromaprint" {
				has = true
			}
		}
		chromaprint[ffmpegPath] = has
	}
	return has
}

// Fingerprint is the chromaprint of a stretch of the file's first audio track, mixed
// to mono: one 32-bit item every FingerprintItem seconds.
func Fingerprint(ctx context.Context, ffmpegPath, file string, from, length float64) ([]uint32, error) {
	args := append([]string{"-loglevel", "error"}, seekArgs(file, from, length)...)
	cmd := analysisCmd(ctx, ffmpegPath, append(args, "-map", "0:a:0", "-ac", "1",
		"-f", "chromaprint", "-fp_format", "raw", "-")...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, analysisError(ctx, err, stderr.String())
	}
	items := make([]uint32, len(out)/4)
	for i := range items {
		items[i] = binary.LittleEndian.Uint32(out[4*i:])
	}
	return items, nil
}

// Loudness is the EBU R128 loudness of a stretch of audio.
type Loudness struct {
	// Momentary[i] is the momentary loudness (LUFS over 400 ms) at (i+1) x
	// LoudnessStep into the stretch; silence reads far below -70.
	Momentary []float64
	// Integrated is the gated loudness of the whole stretch (LUFS; -70 for silence).
	Integrated float64
}

var (
	momentaryRe  = regexp.MustCompile(`\bt:\s*[\d.]+\s.*\bM:\s*(-?[\d.]+|-?inf|nan)`)
	integratedRe = regexp.MustCompile(`^\s*I:\s*(-?[\d.]+|-?inf)\s*LUFS`)
)

// MeasureLoudness runs ebur128 over a stretch of the file's first audio track.
func MeasureLoudness(ctx context.Context, ffmpegPath, file string, from, length float64) (*Loudness, error) {
	args := append([]string{"-nostats"}, seekArgs(file, from, length)...)
	cmd := analysisCmd(ctx, ffmpegPath, append(args, "-map", "0:a:0", "-af", "ebur128=framelog=info", "-f", "null", "-")...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	l := &Loudness{Integrated: -70}
	var other []string
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		if m := momentaryRe.FindStringSubmatch(line); m != nil {
			l.Momentary = append(l.Momentary, parseLUFS(m[1]))
		} else if m := integratedRe.FindStringSubmatch(line); m != nil {
			l.Integrated = max(parseLUFS(m[1]), -70)
		} else if len(other) < 20 {
			other = append(other, line)
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, analysisError(ctx, err, strings.Join(other, "\n"))
	}
	return l, nil
}

func parseLUFS(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		return math.Inf(-1)
	}
	return v
}

// silence is where loudness readings are floored: quieter is all the same silence.
const silence = -70.0

// Seconds is the mean momentary loudness of every whole second, floored at -70 LUFS.
func (l *Loudness) Seconds() []float64 {
	perSecond := int(math.Round(1 / LoudnessStep))
	out := make([]float64, 0, len(l.Momentary)/perSecond+1)
	for i := 0; i < len(l.Momentary); i += perSecond {
		sum, n := 0.0, 0
		for _, v := range l.Momentary[i:min(i+perSecond, len(l.Momentary))] {
			sum += max(v, silence)
			n++
		}
		out = append(out, sum/float64(n))
	}
	return out
}

// Stretch is a span of a measured stretch, in seconds from its start, with its mean
// loudness.
type Stretch struct {
	Start, End float64
	Loudness   float64
}

// Length is the stretch's duration in seconds.
func (s Stretch) Length() float64 { return s.End - s.Start }

// What steady sound is (plan 3.1): music under credits holds a high loudness with a
// small spread over 10 s windows, while dialogue swings widely.
const (
	steadyWindow = 10
	steadyFloor  = -35.0
	steadySpread = 3.5
)

// Steady finds the stretches of steady, loud sound (music, typically): runs of
// consecutive 10 s windows whose one-second loudness averages above -35 LUFS within a
// 3.5 LU spread.
func (l *Loudness) Steady() []Stretch {
	secs := l.Seconds()
	var out []Stretch
	var cur *Stretch
	sum := 0.0
	for w := 0; w+steadyWindow <= len(secs); w += steadyWindow {
		mean, spread := meanSpread(secs[w : w+steadyWindow])
		if mean <= steadyFloor || spread >= steadySpread {
			cur = nil
			continue
		}
		if cur == nil {
			out = append(out, Stretch{Start: float64(w)})
			cur, sum = &out[len(out)-1], 0
		}
		cur.End = float64(w + steadyWindow)
		sum += mean
		cur.Loudness = sum / (cur.Length() / steadyWindow)
	}
	return out
}

func meanSpread(vals []float64) (mean, spread float64) {
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	for _, v := range vals {
		spread += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(spread / float64(len(vals)))
}

// Loudest is the start of the loudest stretch of length seconds within [from, to)
// (whole seconds), or from when the span is not longer than that. A later stretch has
// to be louder by loudestMargin on average to win, so a level piece of music keeps its
// start.
func (l *Loudness) Loudest(from, to, length float64) float64 {
	secs := l.Seconds()
	lo, hi, n := int(math.Ceil(from)), min(int(to), len(secs)), int(math.Round(length))
	if n <= 0 || hi-lo <= n {
		return from
	}
	best, bestAt, sum := math.Inf(-1), lo, 0.0
	for i := lo; i < hi; i++ {
		sum += secs[i]
		if i-lo >= n {
			sum -= secs[i-n]
		}
		if i-lo >= n-1 && sum > best+loudestMargin*float64(n) {
			best, bestAt = sum, i-n+1
		}
	}
	return float64(bestAt)
}

const loudestMargin = 0.5

// CombineLoudness is the loudness of programs measured apart, weighted by their
// durations: their energies averaged.
func CombineLoudness(lufs, weights []float64) float64 {
	energy, total := 0.0, 0.0
	for i, v := range lufs {
		energy += weights[i] * math.Pow(10, v/10)
		total += weights[i]
	}
	if total == 0 || energy == 0 {
		return silence
	}
	return 10 * math.Log10(energy/total)
}

// FrameOptions says which frames of a file Frames decodes and how small.
type FrameOptions struct {
	From, Length float64 // seconds; a zero Length reads to the end
	// FPS samples the picture this many times a second; with Keyframes only the
	// keyframes are decoded, which is far cheaper and lands on most cuts.
	FPS       float64
	Keyframes bool
	// Width and Height (even) are the size frames are scaled to.
	Width, Height int
	// Chroma also delivers the chroma planes (4:2:0), not only luma.
	Chroma bool
	// FullRange scales the planes to 0-255 whatever the file's range: luma then equals a
	// gray decode's, the scale the analyses' thresholds are tuned on, on the reference ffmpeg 7.1
	// (the Pi's; Homebrew's 8.1 differs from a gray decode by 1.9 to 6.5 levels).
	FullRange bool
}

// Frame is one decoded picture.
type Frame struct {
	Index int
	// Time is seconds from the file's start.
	Time          float64
	Width, Height int
	Y             []byte
	// U and V are the half-size chroma planes, with FrameOptions.Chroma.
	U, V []byte
}

var showinfoRe = regexp.MustCompile(`\bn:\s*\d+\s+pts:\s*-?\d+\s+pts_time:(-?[\d.e+-]+)`)

// Frames decodes the file's video as FrameOptions asks and calls fn for every frame,
// in order. A frame's planes are reused two frames later, so fn may keep the previous
// frame but nothing older.
func Frames(ctx context.Context, ffmpegPath, file string, o FrameOptions, fn func(Frame) error) error {
	var args []string
	if o.Keyframes {
		args = append(args, "-skip_frame", "nokey")
	}
	args = append(args, seekArgs(file, o.From, o.Length)...)
	format := "gray"
	if o.Chroma {
		format = "yuv420p"
	}
	scale := fmt.Sprintf("scale=%d:%d:flags=area", o.Width, o.Height)
	if o.FullRange {
		scale += ":out_range=pc"
	}
	filter := fmt.Sprintf("%s,format=%s,showinfo", scale, format)
	if o.FPS > 0 {
		filter = fmt.Sprintf("fps=%g,", o.FPS) + filter
	}
	args = append(args, "-map", "0:V:0", "-an", "-sn", "-dn", "-vf", filter, "-fps_mode", "passthrough",
		"-f", "rawvideo", "-")
	cmd := analysisCmd(ctx, ffmpegPath, append([]string{"-nostats"}, args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// showinfo logs a frame before ffmpeg writes it out, so its time is there (or on
	// its way) when the frame arrives
	times := make(chan float64, 1024)
	var other []string
	logged := make(chan struct{})
	go func() {
		defer close(logged)
		defer close(times)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if m := showinfoRe.FindStringSubmatch(sc.Text()); m != nil {
				t, _ := strconv.ParseFloat(m[1], 64)
				times <- t
			} else if len(other) < 20 && !strings.Contains(sc.Text(), "Parsed_showinfo") {
				other = append(other, sc.Text())
			}
		}
	}()

	luma := o.Width * o.Height
	size := luma
	if o.Chroma {
		size += luma / 2
	}
	bufs := [2][]byte{make([]byte, size), make([]byte, size)}
	var fnErr error
	for i := 0; ; i++ {
		buf := bufs[i%2]
		if _, err := io.ReadFull(stdout, buf); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				fnErr = err
			}
			break
		}
		t, ok := <-times
		if !ok {
			fnErr = errors.New("ffmpeg: a frame without its time")
			break
		}
		f := Frame{Index: i, Time: o.From + t, Width: o.Width, Height: o.Height, Y: buf[:luma]}
		if o.Chroma {
			f.U, f.V = buf[luma:luma+luma/4], buf[luma+luma/4:]
		}
		if fnErr = fn(f); fnErr != nil {
			break
		}
	}
	if fnErr != nil {
		// stop ffmpeg rather than read the rest
		_ = cmd.Process.Kill()
	}
	_, _ = io.Copy(io.Discard, stdout)
	for range times {
	}
	<-logged
	err = cmd.Wait()
	if fnErr != nil {
		return fnErr
	}
	if err != nil {
		return analysisError(ctx, err, strings.Join(other, "\n"))
	}
	return nil
}
