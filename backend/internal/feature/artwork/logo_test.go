package artwork

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func logo(id string, lang string) Artwork {
	a := Artwork{ID: id, Kind: "logo"}
	if lang != "" {
		a.Lang = &lang
	}
	return a
}

func TestPickLogo(t *testing.T) {
	poster := Artwork{ID: "poster", Kind: "poster"}
	en, cs, neutral, de := logo("en", "en"), logo("cs", "cs"), logo("neutral", ""), logo("de", "de")
	for _, c := range []struct {
		name  string
		items []Artwork
		langs []string
		want  string
	}{
		{"display language", []Artwork{poster, en, cs}, []string{"cs", "en"}, "cs"},
		{"base language", []Artwork{poster, en, cs}, []string{"fr", "en"}, "en"},
		{"no display language", []Artwork{en, cs}, []string{"", "cs"}, "cs"},
		{"neutral before others", []Artwork{de, neutral}, []string{"cs", "en"}, "neutral"},
		{"any other, by language", []Artwork{en, de}, []string{"cs", "fr"}, "de"},
		{"none", []Artwork{poster}, []string{"en"}, ""},
	} {
		got := PickLogo(c.items, c.langs...)
		id := ""
		if got != nil {
			id = got.ID
		}
		if id != c.want {
			t.Errorf("%s: picked %q, want %q", c.name, id, c.want)
		}
	}
}

// a white wordmark in the middle of a transparent canvas
func writeLogo(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := h / 4; y < h*3/4; y++ {
		for x := w / 4; x < w*3/4; x++ {
			img.Set(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logo.png")
	writeLogo(t, path, 400, 150)
	if w, h := dimensions(path); w != 400 || h != 150 {
		t.Errorf("dimensions = %dx%d, want 400x150", w, h)
	}
	if w, h := dimensions(filepath.Join(t.TempDir(), "missing.png")); w != 0 || h != 0 {
		t.Errorf("a missing file measured %dx%d", w, h)
	}
}

func TestResolveKeepsALogoTransparent(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	writeLogo(t, filepath.Join(dir, "logo.png"), 1000, 400)
	svc := &Service{DataDir: dir, FFmpegPath: ffmpeg}

	path, err := svc.Resolve(context.Background(), &Artwork{ID: "logo", Kind: "logo", Path: "logo.png"}, "w342")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Fatalf("a logo resized to %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if w := img.Bounds().Dx(); w != 342 {
		t.Errorf("resized to width %d, want 342", w)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the transparent corner has alpha %d", a)
	}

	poster, err := svc.Resolve(context.Background(), &Artwork{ID: "poster", Kind: "poster", Path: "logo.png"}, "w342")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(poster, ".jpg") {
		t.Errorf("a poster resized to %s", poster)
	}
}
