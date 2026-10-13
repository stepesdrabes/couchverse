package artwork

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeInkedLogo writes a transparent PNG of the given size with an opaque block at `ink`.
func writeInkedLogo(t *testing.T, size image.Rectangle, ink image.Rectangle) string {
	t.Helper()
	img := image.NewNRGBA(size)
	for y := ink.Min.Y; y < ink.Max.Y; y++ {
		for x := ink.Min.X; x < ink.Max.X; x++ {
			img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "logo.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestFitLogo(t *testing.T) {
	for _, c := range []struct {
		name       string
		size, ink  image.Rectangle
		wantWidth  int
		wantHeight int
	}{
		{"a padded wordmark is cropped to what shows", image.Rect(0, 0, 1400, 420), image.Rect(200, 120, 1200, 300), 1000, 180},
		{"a tight logo is left as it is", image.Rect(0, 0, 1400, 420), image.Rect(0, 0, 1400, 420), 1400, 420},
		// TMDB's are up to 12000 px wide: kept within 2560 x 1280, at their aspect
		{"an oversized logo is scaled down", image.Rect(0, 0, 6000, 1500), image.Rect(0, 0, 6000, 1500), 2560, 640},
		{"a tall one by its height", image.Rect(0, 0, 1800, 1800), image.Rect(0, 0, 1800, 1800), 1280, 1280},
		{"cropped, then scaled", image.Rect(0, 0, 9000, 3000), image.Rect(500, 400, 8500, 2400), 2560, 640},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := writeInkedLogo(t, c.size, c.ink)
			if err := fitLogo(path); err != nil {
				t.Fatal(err)
			}
			if w, h := dimensions(path); w != c.wantWidth || h != c.wantHeight {
				t.Fatalf("stored at %dx%d, want %dx%d", w, h, c.wantWidth, c.wantHeight)
			}
		})
	}
}

func TestFitLeavesAnEmptyLogo(t *testing.T) {
	path := writeInkedLogo(t, image.Rect(0, 0, 300, 100), image.Rectangle{})
	if err := fitLogo(path); err != nil {
		t.Fatal(err)
	}
	if w, h := dimensions(path); w != 300 || h != 100 {
		t.Fatalf("an empty logo changed to %dx%d", w, h)
	}
}

// A white letter's edge over transparent black keeps its white as it is scaled down, fading by
// its alpha alone: a plain average would grey it, and the letter would carry a dark fringe.
func TestShrinkWeighsColourByAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 30, 9))
	for y := range 9 {
		for x := range 10 {
			src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	got := shrink(src, 4, 3) // 7.5 source pixels to one: the second column is a third white
	if c := got.NRGBAAt(0, 1); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("the letter is %v", c)
	}
	if c := got.NRGBAAt(1, 1); c.R != 255 || c.G != 255 || c.B != 255 || c.A != 85 {
		t.Errorf("its edge is %v, want white at a third of its alpha", c)
	}
	if c := got.NRGBAAt(3, 2); c.A != 0 {
		t.Errorf("the transparent side is %v", c)
	}

	// the area each output pixel covers adds up to the source's: a fully opaque image stays so
	src = image.NewNRGBA(image.Rect(0, 0, 1366, 342))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	got = shrink(src, 1000, 250)
	for i, v := range got.Pix {
		if v != 200 {
			t.Fatalf("byte %d is %d, want 200", i, v)
		}
	}
}

func TestFitSize(t *testing.T) {
	for _, c := range []struct{ w, h, maxW, maxH, wantW, wantH int }{
		{12000, 3833, 1600, 800, 1600, 511},
		{4085, 1713, 1600, 800, 1600, 671},
		{1366, 342, 1600, 800, 1366, 342},
		{1000, 1000, 1600, 800, 800, 800},
		{3840, 2160, 1920, 0, 1920, 1080},
		{1280, 720, 1920, 0, 1280, 720},
	} {
		if w, h := fitSize(c.w, c.h, c.maxW, c.maxH); w != c.wantW || h != c.wantH {
			t.Errorf("%dx%d within %dx%d: %dx%d, want %dx%d", c.w, c.h, c.maxW, c.maxH, w, h, c.wantW, c.wantH)
		}
	}
}

// Within makes a bounded copy once and keeps it: a logo as a PNG that is still transparent
// around its letters, anything else a JPEG; what fits already is the original.
func TestWithin(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx := context.Background()
	dir := t.TempDir()
	writeLogo(t, filepath.Join(dir, "big-logo.png"), 4000, 1000)
	writeLogo(t, filepath.Join(dir, "small-logo.png"), 1366, 342)
	writeJPEG(t, filepath.Join(dir, "backdrop.jpg"), 3840, 2160)
	writeJPEG(t, filepath.Join(dir, "small-backdrop.jpg"), 1280, 720)
	svc := &Service{DataDir: dir, FFmpegPath: ffmpeg}

	big := &Artwork{ID: "big", Kind: "logo", Path: "big-logo.png", Width: 4000, Height: 1000}
	path, err := svc.Within(ctx, big, 1600, 800)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeFile(t, path)
	if b := img.Bounds(); b.Dx() != 1600 || b.Dy() != 400 || !strings.HasSuffix(path, ".png") {
		t.Fatalf("the logo's copy is %s at %v", path, b)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the transparent corner has alpha %d", a)
	}
	if r, g, b, a := img.At(800, 200).RGBA(); r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
		t.Errorf("the letters are %v %v %v %v", r, g, b, a)
	}
	info, _ := os.Stat(path)
	time.Sleep(10 * time.Millisecond)
	if again, err := svc.Within(ctx, big, 1600, 800); err != nil || again != path {
		t.Errorf("asked again: %s %v", again, err)
	}
	if after, _ := os.Stat(path); !after.ModTime().Equal(info.ModTime()) {
		t.Error("the copy was made again")
	}

	small := &Artwork{ID: "small", Kind: "logo", Path: "small-logo.png", Width: 1366, Height: 342}
	if path, err := svc.Within(ctx, small, 1600, 800); err != nil || path != filepath.Join(dir, "small-logo.png") {
		t.Errorf("a logo that fits: %s %v", path, err)
	}

	backdrop := &Artwork{ID: "backdrop", Kind: "backdrop", Path: "backdrop.jpg", Width: 3840, Height: 2160}
	path, err = svc.Within(ctx, backdrop, 1920, 0)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeFile(t, path).Bounds(); b.Dx() != 1920 || b.Dy() != 1080 || !strings.HasSuffix(path, ".jpg") {
		t.Errorf("the backdrop's copy is %s at %v", path, b)
	}
	fits := &Artwork{ID: "fits", Kind: "backdrop", Path: "small-backdrop.jpg", Width: 1280, Height: 720}
	if path, err := svc.Within(ctx, fits, 1920, 0); err != nil || path != filepath.Join(dir, "small-backdrop.jpg") {
		t.Errorf("a backdrop that fits: %s %v", path, err)
	}
	// one not measured (a WebP) is copied, never scaled up
	unmeasured := &Artwork{ID: "unmeasured", Kind: "backdrop", Path: "small-backdrop.jpg"}
	path, err = svc.Within(ctx, unmeasured, 1920, 0)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeFile(t, path).Bounds(); b.Dx() != 1280 || b.Dy() != 720 {
		t.Errorf("an unmeasured backdrop's copy is %v", b)
	}
}

func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func decodeFile(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}
