package artwork

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
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

func TestTrimTransparentMargins(t *testing.T) {
	for _, c := range []struct {
		name       string
		ink        image.Rectangle
		wantWidth  int
		wantHeight int
	}{
		{"a padded wordmark is cropped to what shows", image.Rect(200, 120, 1200, 300), 1000, 180},
		{"a tight logo is left as it is", image.Rect(0, 0, 1400, 420), 1400, 420},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := writeInkedLogo(t, image.Rect(0, 0, 1400, 420), c.ink)
			if err := trimTransparentMargins(path); err != nil {
				t.Fatal(err)
			}
			if w, h := dimensions(path); w != c.wantWidth || h != c.wantHeight {
				t.Fatalf("trimmed to %dx%d, want %dx%d", w, h, c.wantWidth, c.wantHeight)
			}
		})
	}
}

func TestTrimLeavesAnEmptyLogo(t *testing.T) {
	path := writeInkedLogo(t, image.Rect(0, 0, 300, 100), image.Rectangle{})
	if err := trimTransparentMargins(path); err != nil {
		t.Fatal(err)
	}
	if w, h := dimensions(path); w != 300 || h != 100 {
		t.Fatalf("an empty logo changed to %dx%d", w, h)
	}
}
