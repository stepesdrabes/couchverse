package couchtv

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"couchverse/internal/feature/artwork"
)

// The media server hands the screens bounded artwork: a logo within 1600 x 800 that is still
// transparent around its letters, and the backdrop 1920 wide; the artwork feature's own sizes
// and the original as they were.
func TestMediaServerArtworkSizes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx := context.Background()
	dir := t.TempDir()
	// an oversized wordmark: white letters in the middle of a transparent canvas
	logoImg := image.NewNRGBA(image.Rect(0, 0, 4000, 1000))
	for y := 250; y < 750; y++ {
		for x := 1000; x < 3000; x++ {
			logoImg.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	writeImage(t, filepath.Join(dir, "logo.png"), logoImg)
	writeImage(t, filepath.Join(dir, "backdrop.jpg"), image.NewRGBA(image.Rect(0, 0, 3840, 2160)))
	m := &mediaServer{art: &artwork.Service{DataDir: dir, FFmpegPath: ffmpeg}}

	if u := ArtworkURL("a1", 7, logoSize); u != "/media/v1/artwork/a1?v=7&size=1600x800" {
		t.Errorf("logo URL %s", u)
	}
	logo := &artwork.Artwork{ID: "logo", Kind: "logo", Path: "logo.png", Width: 4000, Height: 1000}
	path, err := m.artworkFile(ctx, logo, logoSize)
	if err != nil {
		t.Fatal(err)
	}
	img := readImage(t, path)
	if b := img.Bounds(); b.Dx() != 1600 || b.Dy() != 400 {
		t.Errorf("the logo is %v, want 1600x400", b)
	}
	if _, _, _, a := img.At(10, 10).RGBA(); a != 0 {
		t.Errorf("the logo's corner has alpha %d", a)
	}
	if r, _, _, a := img.At(800, 200).RGBA(); r != 0xffff || a != 0xffff {
		t.Errorf("the logo's letters are %d at alpha %d", r, a)
	}

	backdrop := &artwork.Artwork{ID: "backdrop", Kind: "backdrop", Path: "backdrop.jpg", Width: 3840, Height: 2160}
	for _, c := range []struct {
		size       string
		width      int
		isOriginal bool
	}{{backdropSize, 1920, false}, {"w780", 780, false}, {"", 3840, true}} {
		path, err := m.artworkFile(ctx, backdrop, c.size)
		if err != nil {
			t.Fatal(err)
		}
		if b := readImage(t, path).Bounds(); b.Dx() != c.width || (path == filepath.Join(dir, "backdrop.jpg")) != c.isOriginal {
			t.Errorf("size %q: %s at %v", c.size, path, b)
		}
	}
}

func writeImage(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if filepath.Ext(path) == ".png" {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func readImage(t *testing.T, path string) image.Image {
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
