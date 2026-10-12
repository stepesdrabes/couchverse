package couchtv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"couchverse/internal/feature/playback"
)

// The credits squeeze's program is composited by ffmpeg (plan 6.7): the page renders the
// squeeze with its panel keyed (`program.kind = key`: transparent, the red of the cursor
// handover at full strength), ffmpeg decodes the program itself and lays it over the
// picture at the panel's rectangle, and the alpha is a matte drawn here from the same
// rectangles. Chromium then never decodes a video frame: drawing them, the squeeze rendered
// on the Pi at a third of the speed of a clip of graphics alone.

const stageWidth, stageHeight = 1920, 1080

// KeyRect is the squeeze's program panel at one frame, in stage pixels (render.ts keyRect).
type KeyRect struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Radius  float64 `json:"radius"`
	Opacity float64 `json:"opacity"`
}

// programSource is what fills a keyed panel: count frames of a media file at 25 fps from
// start, the last one held past the source's end, like the extracted frames the page would
// have drawn.
type programSource struct {
	fileID string
	start  time.Duration
	count  int
}

// keyedSpec rewrites a squeeze whose panel shows a program's frames into a keyed one and
// names the program to composite; any other spec comes back as it is, with no program.
func keyedSpec(spec []byte) ([]byte, *programSource, error) {
	var s map[string]json.RawMessage
	if err := json.Unmarshal(spec, &s); err != nil {
		return nil, nil, err
	}
	var template string
	var program struct {
		Kind   string   `json:"kind"`
		Frames []string `json:"frames"`
	}
	if json.Unmarshal(s["template"], &template) != nil || template != "squeeze" ||
		json.Unmarshal(s["program"], &program) != nil || program.Kind != "frames" {
		return spec, nil, nil
	}
	src := framesSource(program.Frames)
	if src == nil {
		return spec, nil, nil
	}
	s["program"] = json.RawMessage(`{"kind":"key"}`)
	out, err := json.Marshal(s)
	return out, src, err
}

// framesSource is the program a list of frame URLs is cut from, when it is one extraction's
// frames in order (FramesURLs), else nil.
func framesSource(urls []string) *programSource {
	var src *programSource
	for i, u := range urls {
		g := framesURL.FindStringSubmatch(u)
		if g == nil {
			return nil
		}
		start, _ := strconv.Atoi(g[2])
		count, _ := strconv.Atoi(g[3])
		n, _ := strconv.Atoi(g[4])
		if src == nil {
			src = &programSource{fileID: g[1], start: time.Duration(start) * time.Millisecond, count: count}
		}
		if g[1] != src.fileID || time.Duration(start)*time.Millisecond != src.start || count != src.count || n != i {
			return nil
		}
	}
	if src == nil || len(urls) != src.count {
		return nil
	}
	return src
}

// composite is a squeeze's program for the encoder to lay over the captured frames.
type composite struct {
	path  string // the source file
	start time.Duration
	count int
	rects []*KeyRect
	boxes []programBox
	graph string
}

// compositeOf prepares the loaded squeeze's composite: its panel's rectangle in every frame
// and the program's source.
func (r *Renderer) compositeOf(ctx context.Context, b *browser, program *programSource, frames int) (*composite, error) {
	rects, err := b.keyRects(ctx, frames)
	if err != nil {
		return nil, err
	}
	if len(rects) != frames {
		return nil, fmt.Errorf("the page placed the program in %d of %d frames", len(rects), frames)
	}
	mf, path, err := r.server.source(ctx, program.fileID)
	if err != nil {
		return nil, fmt.Errorf("the program: %w", err)
	}
	boxes := programBoxes(rects)
	sdr := playback.SDRFilter(mf, playback.DetectFeatures(r.FFmpeg))
	return &composite{path: path, start: program.start, count: program.count, rects: rects, boxes: boxes,
		graph: compositeGraph(sdr, program.count, boxes)}, nil
}

// args are the encoder's inputs after the captured frames (the program, the mattes on pipe
// 3) with the graph, and its output's.
func (c *composite) args() (inputs, output []string) {
	w, h := boxSize(c.boxes)
	// the graph holds the program's last frame, so reading stops a second after its frames
	read := time.Duration(c.count)*time.Second/FPS + time.Second
	inputs = []string{"-ss", fmt.Sprintf("%.3f", c.start.Seconds()), "-t", fmt.Sprintf("%.3f", read.Seconds()), "-i", c.path,
		"-f", "rawvideo", "-pix_fmt", "gray", "-s", fmt.Sprintf("%dx%d", w, h),
		"-framerate", strconv.Itoa(FPS), "-i", "pipe:3", "-filter_complex", c.graph}
	return inputs, []string{"-map", "[v]", "-frames:v", strconv.Itoa(len(c.rects))}
}

// programBox is where the program's picture goes in a frame: whole pixels covering the key
// rectangle (the matte cuts its exact edge), on even ones for the 4:2:0 overlay.
type programBox struct{ x, y, w, h int }

// programBoxes lay out the program for every frame; a frame without a panel keeps the last
// box (its matte is empty).
func programBoxes(rects []*KeyRect) []programBox {
	even := func(v float64) int { return 2 * int(math.Floor(v/2)) }
	out := make([]programBox, len(rects))
	last := programBox{0, 0, 2, 2}
	for i, r := range rects {
		if r != nil && r.Width > 0 && r.Height > 0 {
			x, y := even(r.X), even(r.Y)
			last = programBox{x, y, even(math.Ceil(r.X+r.Width)+1) - x, even(math.Ceil(r.Y+r.Height)+1) - y}
		}
		out[i] = last
	}
	return out
}

// boxSize is the largest box, the size of every program frame and matte in the graph.
func boxSize(boxes []programBox) (w, h int) {
	for _, b := range boxes {
		w, h = max(w, b.w), max(h, b.h)
	}
	return w, h
}

// lookup is an ffmpeg expression that is values[n] at frame n (and the last value after it)
// of a 25 fps stream renumbered from 0. It goes by the frame's time, half a frame early,
// because filters count frames differently from version to version.
func lookup(values []int) string {
	var b strings.Builder
	open := 0
	for i := 0; i < len(values); {
		j := i
		for j < len(values) && values[j] == values[i] {
			j++
		}
		if j == len(values) {
			b.WriteString(strconv.Itoa(values[i]))
			break
		}
		fmt.Fprintf(&b, "if(lt(t,%g),%d,", (float64(j)-0.5)/FPS, values[i])
		open++
		i = j
	}
	b.WriteString(strings.Repeat(")", open))
	return b.String()
}

// compositeGraph is the filter graph that lays the program (input 1, decoded by sdr into
// 8-bit SDR) over the picture (input 0, the captured JPEGs) through the mattes (input 2):
// the program covers the panel like the page's `object-fit: cover`, is scaled to each
// frame's box and padded to the largest (the overlay keeps the size its input started
// with), takes that frame's matte as its alpha and goes over the picture at the box. The
// picture's full-range BT.601 becomes the program's limited-range BT.709 first, so the
// program's pixels pass through as they air. Every input is renumbered from frame 0, and
// the program holds its last frame past count.
func compositeGraph(sdr string, count int, boxes []programBox) string {
	n := len(boxes)
	xs, ys, ws, hs := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	for i, b := range boxes {
		xs[i], ys[i], ws[i], hs[i] = b.x, b.y, b.w, b.h
	}
	w, h := boxSize(boxes)
	renumber := fmt.Sprintf("setpts=N/(%d*TB)", FPS)
	return strings.Join([]string{
		fmt.Sprintf("[1:v:0]fps=%d,scale='iw*sar':ih,setsar=1,"+
			"crop='trunc(min(iw,ih*16/9)/2)*2':'trunc(min(ih,iw*9/16)/2)*2',scale='min(%d,iw)':-2,%s,"+
			"trim=end_frame=%d,tpad=stop_mode=clone:stop=-1,%s,"+
			"scale=w='%s':h='%s':eval=frame,setsar=1,pad=%d:%d:0:0:eval=frame[program]",
			FPS, stageWidth, sdr, count, renumber, lookup(ws), lookup(hs), w, h),
		fmt.Sprintf("[2:v]%s,format=gray[matte]", renumber),
		"[program][matte]alphamerge[layer]",
		fmt.Sprintf("[0:v]%s,%s,format=yuv420p[picture]", renumber, fromJPEG),
		fmt.Sprintf("[picture][layer]overlay=x='%s':y='%s':eval=frame:format=yuv420:shortest=1[v]", lookup(xs), lookup(ys)),
	}, ";")
}

// writeMattes streams every frame's matte, the size of the largest box, drawing only those
// that change.
func writeMattes(w io.WriteCloser, rects []*KeyRect, boxes []programBox) {
	defer w.Close()
	width, height := boxSize(boxes)
	m := make([]byte, width*height)
	for i, r := range rects {
		if i == 0 || !sameRect(r, rects[i-1]) || boxes[i] != boxes[i-1] {
			var local *KeyRect
			if r != nil {
				l := *r
				l.X, l.Y = r.X-float64(boxes[i].x), r.Y-float64(boxes[i].y)
				local = &l
			}
			drawMatte(m, width, height, local)
		}
		if _, err := w.Write(m); err != nil {
			return
		}
	}
}

func sameRect(a, b *KeyRect) bool {
	return a == b || a != nil && b != nil && *a == *b
}

// drawMatte draws one frame's alpha into m (width by height, a byte a pixel): the key
// rectangle with its rounded corners, antialiased by pixel coverage like the browser draws
// the panel, times its opacity. A nil rectangle is an empty matte.
func drawMatte(m []byte, width, height int, r *KeyRect) {
	clear(m)
	if r == nil || r.Width <= 0 || r.Height <= 0 || r.Opacity <= 0 {
		return
	}
	radius := min(max(r.Radius, 0), r.Width/2, r.Height/2)
	opacity := min(r.Opacity, 1)
	// the distance from a pixel's centre to the rounded rectangle's edge (negative inside)
	cx, cy := r.X+r.Width/2, r.Y+r.Height/2
	hx, hy := r.Width/2-radius, r.Height/2-radius
	alpha := func(px, py int) byte {
		qx := math.Abs(float64(px)+0.5-cx) - hx
		qy := math.Abs(float64(py)+0.5-cy) - hy
		d := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - radius
		return byte(math.Round(min(max(0.5-d, 0), 1) * opacity * 255))
	}
	full := byte(math.Round(opacity * 255))
	x0, x1 := max(0, int(math.Floor(r.X))), min(width, int(math.Ceil(r.X+r.Width)))
	y0, y1 := max(0, int(math.Floor(r.Y))), min(height, int(math.Ceil(r.Y+r.Height)))
	for y := y0; y < y1; y++ {
		row := m[y*width : (y+1)*width]
		// the pixels wholly inside, between the edges or the corners, need no distance
		a, b := x0, x0
		top, bottom := float64(y), float64(y+1)
		if top >= r.Y && bottom <= r.Y+r.Height {
			inset := radius
			if top >= r.Y+radius && bottom <= r.Y+r.Height-radius {
				inset = 0
			}
			a = min(max(int(math.Ceil(r.X+inset)), x0), x1)
			b = max(min(int(math.Floor(r.X+r.Width-inset)), x1), a)
		}
		for x := x0; x < a; x++ {
			row[x] = alpha(x, y)
		}
		for x := a; x < b; x++ {
			row[x] = full
		}
		for x := b; x < x1; x++ {
			row[x] = alpha(x, y)
		}
	}
}
