package couchtv

import (
	"math"

	"couchverse/internal/media"
)

// pictureRisk rates how badly a picture suits the channel's footage: 0 is safe, 1 is to
// avoid. rgb is a w x h picture of packed RGB24, meant for a 160 x 90 thumbnail (the
// cells and shares below are tuned on that); a picture that is not w x h pixels is safe.
//
// It reads three things with no face detector, because a thumbnail and a few passes over
// it are all the Pi can spare for every candidate:
//   - gore: blood-red pixels in or beside a big skin-toned blob (a face or a hand
//     bleeding). A red dress in a wide shot has no such blob beside it, and a sunset or a
//     fire is lit orange: not blood, and not skin;
//   - a close-up: one big, compact skin-toned blob, a face or a body filling the frame,
//     where a person at medium distance is a small one. A blob spanning the frame is a
//     wall or sand, not a face. It weighs up to closeUpRisk, never "avoid": a promo may
//     show a face, only after the wide and medium shots of its stretch;
//   - a graphic red frame: a third or more of it dark, saturated red.
//
// Colour cannot tell skin from a pale warm wall, so the readings are tuned to miss
// rather than to cry wolf: faces in shadow and cartoon skin mostly read as no risk.
func pictureRisk(rgb []byte, w, h int) float64 {
	if w <= 0 || h <= 0 || len(rgb) < w*h*3 {
		return 0
	}
	return readPicture(rgb, w, h).risk()
}

// rgbOf converts a frame decoded with media.FrameOptions Chroma and FullRange (4:2:0) to
// packed RGB24. The BT.709 matrix is a guess for files that do not say: skin and blood
// are far enough from their thresholds that BT.601 would read the same.
func rgbOf(f media.Frame) []byte { return toRGB(f, false) }

// limitedRGB is rgbOf for a frame decoded without FullRange, which keeps the file's own range:
// 16 to 235 for nearly every video. A file that is full range reads a little more contrasty and
// saturated, still far from the thresholds.
func limitedRGB(f media.Frame) []byte { return toRGB(f, true) }

func toRGB(f media.Frame, limited bool) []byte {
	out := make([]byte, 0, f.Width*f.Height*3)
	cw := f.Width / 2
	for y := range f.Height {
		for x := range f.Width {
			l := float64(f.Y[y*f.Width+x])
			u := float64(f.U[(y/2)*cw+x/2]) - 128
			v := float64(f.V[(y/2)*cw+x/2]) - 128
			if limited {
				l, u, v = (l-16)*255/219, u*255/224, v*255/224
			}
			out = append(out, clamp8(l+1.5748*v), clamp8(l-0.1873*u-0.4681*v), clamp8(l+1.8556*u))
		}
	}
	return out
}

// steadyRisk is the highest risk that holds for two frames in a row: a flash on one frame is no
// wound, which stays on the screen.
func steadyRisk(risks []float64) float64 {
	steady := 0.0
	for i := 1; i < len(risks); i++ {
		steady = max(steady, min(risks[i-1], risks[i]))
	}
	return steady
}

func clamp8(v float64) byte {
	return byte(min(max(math.Round(v), 0), 255))
}

// What each reading is made of: where it starts to count and where it counts in full
// (a ramp), and the most each weighs.
const (
	// blobCell is the side in pixels of the cells blobs are found on: coarse enough that
	// eyes, lips and shadows do not split a face. A cell is skin from cellSkin of its
	// pixels, which keeps the noise of a pale wall out.
	blobCell = 4
	cellSkin = 0.55

	closeUpFrom, closeUpFull = 0.12, 0.32 // the largest skin blob, share of the frame
	fillFrom, fillFull       = 0.45, 0.62 // how much of its bounding box it fills
	spanFrom, spanFull       = 0.7, 0.95  // its width, share of the frame: a backdrop
	nudeFrom, nudeFull       = 0.30, 0.55 // all skin, share of the frame
	goreFrom, goreFull       = 0.002, 0.007
	faceFrom, faceFull       = 0.02, 0.05 // the blob blood must be beside to count
	redLitFrom, redLitFull   = 0.15, 0.4  // red all over is lighting, not wounds
	dressFrom, dressFull     = 0.5, 1.5   // red bigger than the skin it is beside is a dress or a wall
	graphicFrom, graphicFull = 0.35, 0.65
	hotFrom, hotFull         = 0.01, 0.05 // bright yellow and orange: fire or the sun

	closeUpRisk = 0.7
	nudeRisk    = 0.6
	graphicRisk = 0.6
)

// picture is what pictureRisk reads of a picture, as shares of it.
type picture struct {
	skin  float64 // skin-toned pixels
	face  float64 // the largest skin-toned blob
	faceW float64 // its width
	fill  float64 // its share of its bounding box
	blood float64 // blood-red pixels in or beside the blob
	red   float64 // blood-red pixels anywhere
	hot   float64 // bright yellow and orange: fire, a sunset, a lamp
}

// parts are the readings of a picture's risk, each 0 to 1 before its weight.
func (p picture) parts() (closeUp, nude, gore, graphic float64) {
	// fire and the sun light a frame orange: what shows then is not skin
	skinlike := 1 - ramp(p.hot, hotFrom, hotFull)
	closeUp = ramp(p.face, closeUpFrom, closeUpFull) * ramp(p.fill, fillFrom, fillFull) *
		(1 - ramp(p.faceW, spanFrom, spanFull)) * skinlike
	nude = ramp(p.skin, nudeFrom, nudeFull) * (1 - ramp(p.faceW, 0.85, 1)) * skinlike
	gore = ramp(p.blood, goreFrom, goreFull) * ramp(p.face, faceFrom, faceFull) *
		(1 - ramp(p.red, redLitFrom, redLitFull)) * (1 - ramp(p.blood/max(p.face, faceFrom), dressFrom, dressFull)) * skinlike
	graphic = ramp(p.red, graphicFrom, graphicFull)
	return closeUp, nude, gore, graphic
}

// SceneViolence rates a trailer scene's frames (160 x 90 with chroma, as the scene picker
// decodes them: limited range) for what the channel will not air, 0 to 1: gore, and a frame of
// graphic red. It is a SceneRisk. A face close-up and bare skin are left out, because a
// trailer is made of dialogue, shot in close-ups. Every frame is read, and a reading counts
// when two frames in a row reach it (steadyRisk).
func SceneViolence(frames []media.Frame) float64 {
	each := make([]float64, len(frames))
	for i, f := range frames {
		_, _, gore, graphic := readPicture(limitedRGB(f), f.Width, f.Height).parts()
		each[i] = max(gore, graphicRisk*graphic)
	}
	return steadyRisk(each)
}

func (p picture) risk() float64 {
	closeUp, nude, gore, graphic := p.parts()
	return max(closeUpRisk*closeUp, nudeRisk*nude, gore, graphicRisk*graphic)
}

// ramp is 0 up to from, 1 from full on, and linear between.
func ramp(v, from, full float64) float64 {
	return min(max((v-from)/(full-from), 0), 1)
}

// isSkin is a reddish-orange hue (2 to 24 degrees, red above green above blue) of some
// but not much saturation (28 to 68 %), whatever the brightness: the hue holds across
// skin tones, where a box of Cb and Cr loses the dark ones. It leaves out tan walls and
// sand (hue above 24, or pale), wood, pink, fire (saturated) and the red of lips and
// blood.
func isSkin(r, g, b int) bool {
	if r < 40 || r <= g || g < b {
		return false
	}
	span := r - b
	return span*100 >= 28*r && span*100 <= 68*r && (g-b)*30 >= span && (g-b)*5 <= 2*span
}

// isBlood is a dark or mid, saturated red: green and blue far below red. Lipstick and
// auburn hair are lighter or less saturated.
func isBlood(r, g, b int) bool {
	return r >= 60 && r <= 230 && g*100 <= 24*r && b*100 <= 30*r
}

// isHot is a bright yellow or orange (red and green high, blue well below green), or an
// orange more saturated than skin gets: fire, the sun, a lamp.
func isHot(r, g, b int) bool {
	if r >= 190 && g >= 130 && b*100 <= 60*g {
		return true
	}
	span := r - b
	return r >= 150 && g > b && span*100 > 70*r && (g-b)*100 <= 60*span
}

func readPicture(rgb []byte, w, h int) picture {
	gw, gh := (w+blobCell-1)/blobCell, (h+blobCell-1)/blobCell
	cellN, skinN, bloodN := make([]int, gw*gh), make([]int, gw*gh), make([]int, gw*gh)
	var skin, red, hot int
	for y := range h {
		for x := range w {
			i := (y*w + x) * 3
			r, g, b := int(rgb[i]), int(rgb[i+1]), int(rgb[i+2])
			c := (y/blobCell)*gw + x/blobCell
			cellN[c]++
			switch {
			case isSkin(r, g, b):
				skin++
				skinN[c]++
			case isBlood(r, g, b):
				red++
				bloodN[c]++
			case isHot(r, g, b):
				hot++
			}
		}
	}
	total := float64(w * h)
	p := picture{skin: float64(skin) / total, red: float64(red) / total, hot: float64(hot) / total}

	mask := make([]bool, gw*gh)
	for c := range mask {
		mask[c] = float64(skinN[c]) >= cellSkin*float64(cellN[c])
	}
	blob, box, cells := largestBlob(fillCells(mask, gw, gh), gw, gh)
	if blob > 0 {
		bw, bh := box.x1-box.x0+1, box.y1-box.y0+1
		p.face, p.faceW = float64(blob)/float64(gw*gh), float64(bw)/float64(gw)
		p.fill = float64(blob) / float64(bw*bh)
	}
	near := 0
	for c, n := range bloodN {
		if n > 0 && inOrBeside(cells, gw, gh, c) {
			near += n
		}
	}
	p.blood = float64(near) / total
	return p
}

var neighbours = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// fillCells closes holes in a skin mask: a cell with five or more skin neighbours is skin.
func fillCells(mask []bool, gw, gh int) []bool {
	out := append([]bool(nil), mask...)
	for c := range mask {
		if mask[c] {
			continue
		}
		n := 0
		for _, d := range neighbours {
			if x, y := c%gw+d[0], c/gw+d[1]; x >= 0 && x < gw && y >= 0 && y < gh && mask[y*gw+x] {
				n++
			}
		}
		out[c] = n >= 5
	}
	return out
}

// inOrBeside reports whether cell c, or a neighbour of it, is set in mask.
func inOrBeside(mask []bool, gw, gh, c int) bool {
	if mask[c] {
		return true
	}
	for _, d := range neighbours {
		if x, y := c%gw+d[0], c/gw+d[1]; x >= 0 && x < gw && y >= 0 && y < gh && mask[y*gw+x] {
			return true
		}
	}
	return false
}

type cellBox struct{ x0, y0, x1, y1 int }

// largestBlob is the cell count, bounding box and cells of the biggest 8-connected run of
// set cells in a mask.
func largestBlob(mask []bool, gw, gh int) (int, cellBox, []bool) {
	seen := make([]bool, len(mask))
	best, bestBox := 0, cellBox{}
	var stack, run, bestRun []int
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		box := cellBox{x0: gw, y0: gh}
		stack, run = append(stack[:0], start), run[:0]
		seen[start] = true
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := c%gw, c/gw
			run = append(run, c)
			box = cellBox{min(box.x0, x), min(box.y0, y), max(box.x1, x), max(box.y1, y)}
			for _, d := range neighbours {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= gw || ny < 0 || ny >= gh || !mask[ny*gw+nx] || seen[ny*gw+nx] {
					continue
				}
				seen[ny*gw+nx] = true
				stack = append(stack, ny*gw+nx)
			}
		}
		if len(run) > best {
			best, bestBox = len(run), box
			bestRun = append(bestRun[:0], run...)
		}
	}
	cells := make([]bool, len(mask))
	for _, c := range bestRun {
		cells[c] = true
	}
	return best, bestBox, cells
}
