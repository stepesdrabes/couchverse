package artwork

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func fill(img *image.RGBA, c color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

func parseAccent(t *testing.T, hex string) (r, g, b int) {
	t.Helper()
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("unparseable accent %q: %v", hex, err)
	}
	return r, g, b
}

// A mostly-grey image with a smaller saturated block should accent to the
// saturated colour, not the dominant grey.
func TestVibrantAccentPrefersSaturatedOverDominant(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fill(img, color.RGBA{40, 40, 40, 255}) // dark grey background (9100 px)
	orange := color.RGBA{230, 120, 20, 255}
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			img.Set(x, y, orange) // 900 px
		}
	}
	r, g, b := parseAccent(t, vibrantAccent(img))
	if r < 150 || g < 60 || b > 120 {
		t.Fatalf("expected an orange-ish accent, got r=%d g=%d b=%d", r, g, b)
	}
}

// A flat grey image has nothing vibrant, so it falls back to the dominant
// colour rather than returning empty.
func TestVibrantAccentFallsBackToDominant(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	fill(img, color.RGBA{80, 80, 80, 255})
	hex := vibrantAccent(img)
	if hex == "" {
		t.Fatal("expected a fallback accent for a flat image")
	}
	r, g, b := parseAccent(t, hex)
	if r < 60 || r > 100 || g < 60 || g > 100 || b < 60 || b > 100 {
		t.Fatalf("expected a grey fallback near #505050, got %s", hex)
	}
}
