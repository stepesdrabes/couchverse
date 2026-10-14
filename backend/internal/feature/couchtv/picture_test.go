package couchtv

import (
	"math"
	"slices"
	"testing"

	"couchverse/internal/media"
)

type rgbColor = [3]byte

var (
	skinPale   = rgbColor{205, 152, 124}
	skinDark   = rgbColor{112, 72, 52}
	hairDark   = rgbColor{30, 22, 20}
	bloodRed   = rgbColor{100, 12, 8}
	dressRed   = rgbColor{200, 30, 40}
	nightBlack = rgbColor{12, 12, 16}
	roomGrey   = rgbColor{96, 100, 104}
)

// canvas is a shotW x shotH picture drawn from flat shapes.
type canvas struct{ px []byte }

func newCanvas(bg rgbColor) *canvas {
	c := &canvas{px: make([]byte, shotW*shotH*3)}
	c.rect(0, 0, shotW, shotH, bg)
	return c
}

func (c *canvas) set(x, y int, col rgbColor) {
	if x >= 0 && x < shotW && y >= 0 && y < shotH {
		copy(c.px[(y*shotW+x)*3:], col[:])
	}
}

func (c *canvas) rect(x0, y0, x1, y1 int, col rgbColor) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.set(x, y, col)
		}
	}
}

func (c *canvas) ellipse(cx, cy, rx, ry int, col rgbColor) {
	for y := cy - ry; y <= cy+ry; y++ {
		for x := cx - rx; x <= cx+rx; x++ {
			dx, dy := float64(x-cx)/float64(rx), float64(y-cy)/float64(ry)
			if dx*dx+dy*dy <= 1 {
				c.set(x, y, col)
			}
		}
	}
}

// face is a head with hairDark above it, eyes and a mouth, rx wide.
func (c *canvas) face(cx, cy, rx int, skin rgbColor) {
	ry := rx * 6 / 5
	c.ellipse(cx, cy-ry/3, rx+rx/8, ry, hairDark)
	c.ellipse(cx, cy, rx, ry, skin)
	c.ellipse(cx-rx/2, cy-ry/4, max(rx/6, 1), max(rx/10, 1), hairDark)
	c.ellipse(cx+rx/2, cy-ry/4, max(rx/6, 1), max(rx/10, 1), hairDark)
	c.ellipse(cx, cy+ry/2, max(rx/3, 1), max(rx/10, 1), rgbColor{150, 70, 76})
}

func (c *canvas) risk() float64 { return pictureRisk(c.px, shotW, shotH) }

func TestPictureRisk(t *testing.T) {
	scene := func(f func(c *canvas)) float64 {
		c := newCanvas(roomGrey)
		f(c)
		return c.risk()
	}
	tests := []struct {
		name     string
		risk     float64
		min, max float64
	}{
		{"an empty room", scene(func(c *canvas) {}), 0, 0},
		{"a dark scene", func() float64 {
			c := newCanvas(nightBlack)
			c.rect(40, 30, 60, 60, rgbColor{30, 24, 22})
			return c.risk()
		}(), 0, 0},
		{"a person far away", scene(func(c *canvas) { c.face(80, 40, 3, skinPale) }), 0, 0},
		{"two people at medium distance", scene(func(c *canvas) {
			c.face(50, 40, 8, skinPale)
			c.face(105, 42, 8, skinDark)
		}), 0, 0},
		{"a pale face close-up", scene(func(c *canvas) { c.face(80, 44, 34, skinPale) }), 0.55, closeUpRisk},
		{"a dark face close-up", scene(func(c *canvas) { c.face(80, 44, 34, skinDark) }), 0.55, closeUpRisk},
		{"a head and shoulders", scene(func(c *canvas) { c.face(80, 36, 17, skinPale) }), 0, 0.2},
		{"blood on a face close-up", scene(func(c *canvas) {
			c.face(80, 44, 34, skinPale)
			c.rect(78, 50, 84, 66, bloodRed)
		}), avoidRisk, 1},
		{"blood on a medium shot", scene(func(c *canvas) {
			c.face(80, 40, 15, skinPale)
			c.rect(77, 42, 83, 60, bloodRed)
		}), avoidRisk, 1},
		{"blood on a hand", scene(func(c *canvas) {
			c.ellipse(80, 45, 28, 30, skinPale)
			c.ellipse(76, 50, 6, 9, bloodRed)
			c.ellipse(90, 38, 4, 5, bloodRed)
		}), avoidRisk, 1},
		{"a red dress in a wide shot", scene(func(c *canvas) {
			c.rect(40, 50, 60, 90, dressRed)
			c.face(50, 42, 4, skinPale)
		}), 0, 0},
		{"a red dress at medium distance", scene(func(c *canvas) {
			c.rect(30, 52, 80, 90, dressRed)
			c.face(55, 40, 9, skinPale)
			c.rect(24, 54, 30, 70, skinPale)
		}), 0, 0.1},
		{"a red couch beside a person", scene(func(c *canvas) {
			c.rect(60, 40, 160, 90, dressRed)
			c.face(40, 40, 14, skinPale)
		}), 0, 0.1},
		{"a sunset", func() float64 {
			c := newCanvas(rgbColor{250, 205, 100})
			for y := range shotH {
				k := float64(y) / shotH
				c.rect(0, y, shotW, y+1, rgbColor{250, byte(205 - 120*k), byte(100 - 70*k)})
			}
			c.rect(0, 70, shotW, shotH, nightBlack)
			c.ellipse(120, 70, 4, 8, nightBlack)
			return c.risk()
		}(), 0, 0},
		{"a fire", func() float64 {
			c := newCanvas(rgbColor{20, 14, 12})
			c.ellipse(80, 55, 50, 35, rgbColor{230, 120, 40})
			c.ellipse(80, 55, 25, 20, rgbColor{255, 200, 90})
			return c.risk()
		}(), 0, 0},
		{"a face in an orange glow", func() float64 {
			c := newCanvas(rgbColor{210, 100, 30})
			c.face(80, 44, 34, skinPale)
			return c.risk()
		}(), 0, 0},
		{"a wall of warm paint", scene(func(c *canvas) {
			c.rect(0, 0, shotW, shotH, rgbColor{210, 160, 124})
			c.face(110, 50, 7, skinPale)
		}), 0, 0},
		{"a pale wall", scene(func(c *canvas) { c.rect(0, 0, 100, shotH, rgbColor{205, 175, 160}) }), 0, 0},
		{"red lipstick and auburn hair", scene(func(c *canvas) {
			c.face(50, 44, 16, skinPale)
			c.ellipse(50, 22, 18, 10, rgbColor{140, 52, 30})
			c.ellipse(50, 56, 6, 2, rgbColor{170, 40, 55})
		}), 0, 0.2},
		{"a frame of red", scene(func(c *canvas) { c.rect(0, 0, shotW, shotH, rgbColor{150, 16, 14}) }), 0.3, graphicRisk},
	}
	for _, tt := range tests {
		if tt.risk < tt.min-1e-9 || tt.risk > tt.max+1e-9 {
			t.Errorf("%s: risk %.2f, want %.2f to %.2f", tt.name, tt.risk, tt.min, tt.max)
		}
	}
}

// Wide and medium shots rank before close-ups, which rank before gore.
func TestPictureRiskOrder(t *testing.T) {
	wide := newCanvas(roomGrey)
	wide.face(60, 45, 5, skinPale)
	medium := newCanvas(roomGrey)
	medium.face(80, 40, 14, skinPale)
	closeUp := newCanvas(roomGrey)
	closeUp.face(80, 44, 34, skinPale)
	bloody := newCanvas(roomGrey)
	bloody.face(80, 44, 34, skinPale)
	bloody.rect(78, 50, 84, 66, bloodRed)
	risks := []float64{wide.risk(), medium.risk(), closeUp.risk(), bloody.risk()}
	for i := 1; i < len(risks); i++ {
		if risks[i-1] > risks[i] {
			t.Errorf("risks of wide, medium, close-up, bloody are %.2f", risks)
			break
		}
	}
	if risks[2] <= risks[1] || risks[3] <= risks[2] {
		t.Errorf("risks of wide, medium, close-up, bloody are %.2f", risks)
	}
}

func TestPictureRiskBadInput(t *testing.T) {
	if r := pictureRisk(nil, shotW, shotH); r != 0 {
		t.Errorf("no picture: %v", r)
	}
	if r := pictureRisk(make([]byte, 10), shotW, shotH); r != 0 {
		t.Errorf("a short picture: %v", r)
	}
	if r := pictureRisk(make([]byte, 30), 0, 0); r != 0 {
		t.Errorf("no size: %v", r)
	}
}

// frameOf encodes RGB as the full-range BT.709 4:2:0 frame media.Frames delivers with
// Chroma and FullRange, taking each chroma sample from its block's top-left pixel.
func frameOf(rgb []byte) media.Frame { return encodeFrame(rgb, false) }

// limitedFrameOf is frameOf in the limited range (luma 16 to 235, chroma 16 to 240) that
// media.Frames delivers without FullRange.
func limitedFrameOf(rgb []byte) media.Frame { return encodeFrame(rgb, true) }

func encodeFrame(rgb []byte, limited bool) media.Frame {
	f := media.Frame{Width: shotW, Height: shotH, Y: make([]byte, shotW*shotH),
		U: make([]byte, shotW*shotH/4), V: make([]byte, shotW*shotH/4)}
	scaleY, scaleC, offY := 1.0, 1.0, 0.0
	if limited {
		scaleY, scaleC, offY = 219.0/255, 224.0/255, 16
	}
	for y := range shotH {
		for x := range shotW {
			i := (y*shotW + x) * 3
			r, g, b := float64(rgb[i]), float64(rgb[i+1]), float64(rgb[i+2])
			l := 0.2126*r + 0.7152*g + 0.0722*b
			f.Y[y*shotW+x] = clamp8(l*scaleY + offY)
			if x%2 == 0 && y%2 == 0 {
				f.U[(y/2)*(shotW/2)+x/2] = clamp8((b-l)/1.8556*scaleC + 128)
				f.V[(y/2)*(shotW/2)+x/2] = clamp8((r-l)/1.5748*scaleC + 128)
			}
		}
	}
	return f
}

func TestRgbOf(t *testing.T) {
	c := newCanvas(roomGrey)
	c.rect(0, 0, 80, 44, skinPale)
	c.rect(80, 44, shotW, shotH, bloodRed)
	for name, got := range map[string][]byte{"full range": rgbOf(frameOf(c.px)), "limited range": limitedRGB(limitedFrameOf(c.px))} {
		for _, p := range [][2]int{{10, 10}, {40, 20}, {120, 60}, {150, 80}, {120, 10}} {
			i := (p[1]*shotW + p[0]) * 3
			for k := range 3 {
				if d := math.Abs(float64(got[i+k]) - float64(c.px[i+k])); d > 3 {
					t.Errorf("%s: pixel %v channel %d: %d, want %d", name, p, k, got[i+k], c.px[i+k])
				}
			}
		}
	}
}

func TestSteadyRisk(t *testing.T) {
	for _, tt := range []struct {
		name  string
		risks []float64
		want  float64
	}{
		{"no frames", nil, 0},
		{"one frame", []float64{1}, 0},
		{"a flash on one frame", []float64{0, 0.2, 1, 0.1, 0}, 0.2},
		{"two frames in a row", []float64{0, 1, 0.9, 0}, 0.9},
		{"frames apart", []float64{1, 0, 1, 0, 1}, 0},
		{"a run that eases", []float64{0.3, 0.7, 0.8, 0.5}, 0.7},
	} {
		if got := steadyRisk(tt.risks); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

// shotOf is n frames of calm with the bloody frame in each place bloodyAt names.
func shotOf(n int, bloodyAt ...int) []media.Frame {
	calm := newCanvas(roomGrey)
	calm.face(60, 45, 5, skinPale)
	bloody := newCanvas(roomGrey)
	bloody.face(80, 44, 34, skinPale)
	bloody.rect(78, 50, 84, 66, bloodRed)
	calmFrame, bloodyFrame := frameOf(calm.px), frameOf(bloody.px)
	frames := make([]media.Frame, n)
	for i := range frames {
		frames[i] = calmFrame
		if slices.Contains(bloodyAt, i) {
			frames[i] = bloodyFrame
		}
	}
	return frames
}

// A shot is rated by its worst checked picture, the last frame included; it is to be avoided
// when two frames in a row are gory, whether or not a check fell on them.
func TestShotRisk(t *testing.T) {
	tests := []struct {
		name   string
		frames []media.Frame
		rated  bool // the risk reaches avoidRisk
		avoid  bool
	}{
		{"a calm shot", shotOf(13), false, false},
		{"blood on a checked frame for 0.2 s", shotOf(13, 5, 6), true, true},
		{"blood on one frame only", shotOf(13, 5), true, false},
		{"blood on the last two frames", shotOf(13, 11, 12), true, true},
		{"blood on the last frame, which is checked", shotOf(13, 12), true, false},
		{"blood on a last frame that is checked anyway", shotOf(11, 10), true, false},
		{"blood between the checks (frames 11 to 14 of 31)", shotOf(31, 11, 12, 13, 14), false, true},
		{"blood on two frames between the checks", shotOf(31, 11, 12), false, true},
		{"blood on one frame between the checks", shotOf(31, 12), false, false},
		{"blood on frames apart", shotOf(31, 11, 13, 16, 18), false, false},
	}
	for _, tt := range tests {
		risk, avoid := shotRisk(tt.frames)
		if (risk >= avoidRisk) != tt.rated || avoid != tt.avoid {
			t.Errorf("%s: risk %.2f, avoid %v; want rated %v, avoid %v", tt.name, risk, avoid, tt.rated, tt.avoid)
		}
	}
}

// sceneOf is n limited-range frames of the calm canvas, with the bloody one in each place
// bloodyAt names.
func sceneOf(n int, calm, bloody *canvas, bloodyAt ...int) []media.Frame {
	calmFrame, bloodyFrame := limitedFrameOf(calm.px), limitedFrameOf(bloody.px)
	frames := make([]media.Frame, n)
	for i := range frames {
		frames[i] = calmFrame
		if slices.Contains(bloodyAt, i) {
			frames[i] = bloodyFrame
		}
	}
	return frames
}

// A trailer scene is refused for gore and for a frame of red, never for faces: its lines are
// dialogue, shot in close-ups, which the footage shots' rating would weigh up to 0.7.
func TestSceneViolence(t *testing.T) {
	face := func(skin rgbColor) *canvas {
		c := newCanvas(roomGrey)
		c.face(80, 44, 34, skin)
		return c
	}
	bloodyFace := func() *canvas {
		c := face(skinPale)
		c.rect(78, 50, 84, 66, bloodRed)
		return c
	}
	bloodyMedium := newCanvas(roomGrey)
	bloodyMedium.face(80, 40, 15, skinPale)
	bloodyMedium.rect(77, 42, 83, 60, bloodRed)
	redRoom := newCanvas(rgbColor{150, 16, 14})
	redDress := newCanvas(roomGrey)
	redDress.rect(30, 52, 80, 90, dressRed)
	redDress.face(55, 40, 9, skinPale)
	wide := newCanvas(roomGrey)
	wide.face(60, 45, 5, skinPale)

	tests := []struct {
		name   string
		frames []media.Frame
		refuse bool
	}{
		{"a wide shot", sceneOf(125, wide, wide), false},
		{"a pale face close-up all through", sceneOf(125, face(skinPale), face(skinPale)), false},
		{"a dark face close-up all through", sceneOf(125, face(skinDark), face(skinDark)), false},
		{"blood on a face close-up, two frames", sceneOf(125, face(skinPale), bloodyFace(), 60, 61), true},
		{"blood on a face close-up, all through", sceneOf(125, bloodyFace(), bloodyFace()), true},
		{"blood on a face close-up, one frame", sceneOf(125, face(skinPale), bloodyFace(), 60), false},
		{"blood on a medium shot, four frames", sceneOf(125, wide, bloodyMedium, 60, 61, 62, 63), true},
		{"blood on frames apart", sceneOf(125, face(skinPale), bloodyFace(), 60, 62, 64), false},
		{"a frame of red all through", sceneOf(125, redRoom, redRoom), true},
		{"a red dress", sceneOf(125, redDress, redDress), false},
	}
	for _, tt := range tests {
		r := SceneViolence(tt.frames)
		if (r >= riskLimit) != tt.refuse {
			t.Errorf("%s: risk %.2f, refused %v, want %v", tt.name, r, r >= riskLimit, tt.refuse)
		}
	}
	if r := SceneViolence(nil); r != 0 {
		t.Errorf("no frames: %v", r)
	}
}
