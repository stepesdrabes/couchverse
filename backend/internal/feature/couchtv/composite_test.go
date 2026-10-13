package couchtv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyedSpec(t *testing.T) {
	const file = "6f1c2c1e-6d55-4bb4-9a51-4f0b2f8f2d10"
	squeeze := Spec{Template: "squeeze", Handover: "ident",
		Program: &ProgramSource{Kind: "frames", Frames: FramesURLs(file, 1200*time.Second, 288)}}
	spec, _ := json.Marshal(squeeze)
	out, program, err := keyedSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if program == nil || program.fileID != file || program.start != 1200*time.Second || program.count != 288 {
		t.Fatalf("program %+v", program)
	}
	var keyed Spec
	if err := json.Unmarshal(out, &keyed); err != nil || keyed.Program.Kind != "key" || len(keyed.Program.Frames) != 0 || keyed.Handover != "ident" {
		t.Errorf("keyed spec %s (%v)", out, err)
	}

	// what the page draws itself stays as it is
	shuffled := squeeze
	frames := append([]string{}, squeeze.Program.Frames...)
	frames[3], frames[4] = frames[4], frames[3]
	shuffled.Program = &ProgramSource{Kind: "frames", Frames: frames}
	for name, s := range map[string]Spec{
		"stand-in":   {Template: "squeeze", Program: &ProgramSource{Kind: "standIn"}},
		"a promo":    {Template: "promo", Shots: []Shot{{Kind: "footage", Frames: FramesURLs(file, 0, 25)}}},
		"reordered":  shuffled,
		"cut short":  {Template: "squeeze", Program: &ProgramSource{Kind: "frames", Frames: squeeze.Program.Frames[:100]}},
		"other urls": {Template: "squeeze", Program: &ProgramSource{Kind: "frames", Frames: []string{"/x/0.jpg"}}},
	} {
		in, _ := json.Marshal(s)
		out, program, err := keyedSpec(in)
		if err != nil || program != nil || !bytes.Equal(out, in) {
			t.Errorf("%s: %s %+v %v", name, out, program, err)
		}
	}
}

func TestLookup(t *testing.T) {
	for _, c := range []struct {
		values []int
		want   string
	}{
		{[]int{5}, "5"},
		{[]int{5, 5, 5}, "5"},
		{[]int{5, 5, 7, 7, 7, 9}, "if(lt(t,0.06),5,if(lt(t,0.18),7,9))"},
		{[]int{-48, -47, 96, 96}, "if(lt(t,0.02),-48,if(lt(t,0.06),-47,96))"},
	} {
		if got := lookup(c.values); got != c.want {
			t.Errorf("lookup(%v) = %s, want %s", c.values, got, c.want)
		}
	}
}

func TestProgramBoxes(t *testing.T) {
	got := programBoxes([]*KeyRect{
		nil,
		{X: -48, Y: -27, Width: 2016, Height: 1134, Opacity: 1},
		{X: 95.4, Y: 235.9, Width: 1008.2, Height: 567.1, Opacity: 1},
		nil,
	})
	want := []programBox{{0, 0, 2, 2}, {-48, -28, 2016, 1136}, {94, 234, 1010, 570}, {94, 234, 1010, 570}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The matte's shortcut for the pixels wholly inside matches the coverage of every pixel.
func TestDrawMatte(t *testing.T) {
	m := make([]byte, stageWidth*stageHeight)
	slow := make([]byte, stageWidth*stageHeight)
	r := rand.New(rand.NewPCG(1, 2))
	rects := []*KeyRect{
		{X: 96, Y: 236, Width: 1008, Height: 567, Radius: 24.15, Opacity: 1},
		{X: -48, Y: -27, Width: 2016, Height: 1134, Radius: 48.3, Opacity: 1},
		{X: 843.2, Y: 436.8, Width: 164.9, Height: 201.5, Radius: 3.9, Opacity: 0.6},
		{X: 10.5, Y: 10.5, Width: 0.8, Height: 3, Radius: 1, Opacity: 1},
	}
	for range 20 {
		rects = append(rects, &KeyRect{X: r.Float64()*2200 - 200, Y: r.Float64()*1300 - 200, Width: r.Float64() * 1500,
			Height: r.Float64() * 900, Radius: r.Float64() * 80, Opacity: r.Float64()})
	}
	for _, rect := range rects {
		drawMatte(m, stageWidth, stageHeight, rect)
		coverage(slow, rect)
		if !bytes.Equal(m, slow) {
			for p := range m {
				if m[p] != slow[p] {
					t.Fatalf("%+v: pixel %d,%d is %d, its coverage %d", rect, p%stageWidth, p/stageWidth, m[p], slow[p])
				}
			}
		}
	}

	drawMatte(m, stageWidth, stageHeight, &KeyRect{X: 100.5, Y: 200, Width: 400, Height: 300, Radius: 20, Opacity: 1})
	at := func(x, y int) byte { return m[y*stageWidth+x] }
	if at(300, 350) != 255 || at(99, 350) != 0 || at(100, 350) != 128 || at(100, 200) != 0 || at(130, 200) != 255 || at(300, 500) != 0 {
		t.Errorf("inside %d, outside %d, half a pixel %d, the corner %d, the top edge %d, below %d",
			at(300, 350), at(99, 350), at(100, 350), at(100, 200), at(130, 200), at(300, 500))
	}
	drawMatte(m, stageWidth, stageHeight, nil)
	if bytes.Count(m, []byte{0}) != len(m) {
		t.Error("an empty frame's matte is not empty")
	}
}

// coverage is drawMatte computed for every pixel.
func coverage(m []byte, r *KeyRect) {
	clear(m)
	if r == nil || r.Width <= 0 || r.Height <= 0 || r.Opacity <= 0 {
		return
	}
	radius := min(max(r.Radius, 0), r.Width/2, r.Height/2)
	for y := range stageHeight {
		for x := range stageWidth {
			qx := math.Abs(float64(x)+0.5-(r.X+r.Width/2)) - (r.Width/2 - radius)
			qy := math.Abs(float64(y)+0.5-(r.Y+r.Height/2)) - (r.Height/2 - radius)
			d := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - radius
			m[y*stageWidth+x] = byte(math.Round(min(max(0.5-d, 0), 1) * min(r.Opacity, 1) * 255))
		}
	}
}

// TestCompositeGraph runs the composite in ffmpeg: a white program over a grey picture, at a
// rectangle, at a rectangle larger than the stage and with no panel.
func TestCompositeGraph(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	rects := []*KeyRect{
		{X: 100.5, Y: 200, Width: 400, Height: 300, Radius: 20, Opacity: 1},
		{X: -48, Y: -27, Width: 2016, Height: 1134, Radius: 48.3, Opacity: 1},
		nil,
	}
	boxes := programBoxes(rects)
	matte := filepath.Join(dir, "matte.raw")
	f, err := os.Create(matte)
	if err != nil {
		t.Fatal(err)
	}
	writeMattes(f, rects, boxes)
	w, h := boxSize(boxes)
	out := filepath.Join(dir, "out.raw")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=0x808080:s=1920x1080:r=25:d=1,format=yuvj420p",
		"-f", "lavfi", "-i", "color=c=white:s=1280x720:r=25:d=0.04",
		"-f", "rawvideo", "-pix_fmt", "gray", "-s", fmt.Sprintf("%dx%d", w, h), "-framerate", "25", "-i", matte,
		"-filter_complex", compositeGraph("format=yuv420p", 1, boxes),
		"-map", "[v]", "-frames:v", "3", "-f", "rawvideo", "-pix_fmt", "gray", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	luma, err := os.ReadFile(out)
	if err != nil || len(luma) != 3*stageWidth*stageHeight {
		t.Fatalf("%d bytes of output: %v", len(luma), err)
	}
	// read back as full-range grey: the program's white is 255, the picture's grey 128
	near := func(frame, x, y int, want float64) {
		t.Helper()
		if got := float64(luma[frame*stageWidth*stageHeight+y*stageWidth+x]); math.Abs(got-want) > 3 {
			t.Errorf("frame %d at %d,%d: %v, want %v", frame, x, y, got, want)
		}
	}
	near(0, 300, 350, 255)
	near(0, 50, 50, 128)
	near(0, 100, 350, (255+128)/2.0)
	near(0, 101, 201, 128)
	near(1, 5, 5, 255)
	near(1, 1915, 1075, 255)
	near(2, 300, 350, 128)
}
