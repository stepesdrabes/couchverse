package couchtv

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

// A stream of a file has a URL of its own, cached beside the default cuts (the cleanup judges
// files, not directories) and apart from them; the default form is unchanged.
func TestStreamURL(t *testing.T) {
	const id = "0f5c1a52-8e6d-4c1a-9a4e-3d2f4c7b9e11"
	u := StreamURL(id, 2, 12500*time.Millisecond, 4200*time.Millisecond)
	if want := "/media/v1/audio/" + id + "/2/12500_4200.flac"; u != want {
		t.Fatalf("StreamURL = %s, want %s", u, want)
	}
	if d := AudioURL(id, 12500*time.Millisecond, 4200*time.Millisecond); d != "/media/v1/audio/"+id+"/12500_4200.flac" {
		t.Errorf("the default form changed: %s", d)
	}
	m := &mediaServer{dataDir: "/data"}
	path, ok := m.local(u)
	if want := "/data/couchtv/audio/" + id + "/s2_12500_4200.flac"; !ok || path != want {
		t.Errorf("local(%s) = %s %v, want %s", u, path, ok, want)
	}
	if def, _ := m.local(AudioURL(id, 12500*time.Millisecond, 4200*time.Millisecond)); def == path {
		t.Error("the default cut and the stream's share a file")
	}
	if streamURL.MatchString(AudioURL(id, 0, time.Second)) || audioURL.MatchString(u) {
		t.Error("the two forms match each other's URLs")
	}
	spec := `{"sound":{"url":"` + u + `","in":0,"out":4.2},"bed":{"url":"/media/v1/audio/` + id + `/0_1000.flac"}}`
	if got := mediaURLs([]byte(spec)); len(got) != 2 || got[0] != u {
		t.Errorf("mediaURLs finds %v", got)
	}
}

func TestDialogueFilter(t *testing.T) {
	for channels, want := range map[int]string{1: "", 2: "", 3: "", 6: "pan=stereo|c0=0.46*c2+0.34*c0+0.20*c4|c1=0.46*c2+0.34*c1+0.20*c5", 8: "pan=stereo|c0=0.44*c2"} {
		if got := dialogueFilter(channels); !strings.HasPrefix(got, want) {
			t.Errorf("%d channels: %q, want %q", channels, got, want)
		}
	}
	for _, channels := range []int{6, 8} {
		// the weights of an output channel add up to 1: a loud scene must not clip
		var sum float64
		for _, term := range strings.Split(strings.Split(strings.Split(dialogueFilter(channels), "|")[1], "=")[1], "+") {
			w, _ := strconv.ParseFloat(strings.Split(term, "*")[0], 64)
			sum += w
		}
		if math.Abs(sum-1) > 1e-9 {
			t.Errorf("%d channels: weights add up to %g", channels, sum)
		}
	}
}

// A 5.1 stream's dialogue (the centre channel) comes out louder against its front channels
// than ffmpeg's standard downmix puts it, and a filter ffmpeg refuses still leaves a cut.
func TestCutAudioKeepsDialogueUp(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "surround.mka")
	// the left channel carries 500 Hz, the centre 1 kHz, both at the same level
	if out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=f=500:sample_rate=48000:duration=2", "-f", "lavfi", "-i", "sine=f=1000:sample_rate=48000:duration=2",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono:d=2",
		"-filter_complex", "[0][2][1][2][2][2]amerge=inputs=6[a]", "-map", "[a]", "-c:a", "flac", src).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	m := &mediaServer{ffmpeg: ffmpeg}
	ctx := context.Background()
	// the left channel's 1 kHz (the centre's) and 500 Hz (the front's) amplitudes
	levels := func(out string) (centre, front float64) {
		t.Helper()
		raw, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-i", out, "-f", "f32le", "-ac", "2", "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		var l []float64
		for i := 0; i+8 <= len(raw); i += 8 {
			l = append(l, float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i:]))))
		}
		return goertzel(l, 1000, 48000), goertzel(l, 500, 48000)
	}

	plain := filepath.Join(dir, "plain.flac")
	if err := m.cutAudio(ctx, src, "0:0", 0, time.Second, plain, ""); err != nil {
		t.Fatal(err)
	}
	c, f := levels(plain)
	standard := c / f
	forward := filepath.Join(dir, "forward.flac")
	if err := m.cutAudio(ctx, src, "0:0", 0, time.Second, forward, dialogueFilter(6)); err != nil {
		t.Fatal(err)
	}
	c, f = levels(forward)
	if want := 0.46 / 0.34; math.Abs(c/f-want) > 0.05*want || c/f < 1.5*standard {
		t.Errorf("the dialogue is %.2f times the front channel, the standard downmix gives %.2f", c/f, standard)
	}

	odd := filepath.Join(dir, "odd.flac")
	if err := m.cutAudio(ctx, src, "0:0", 0, time.Second, odd, "pan=stereo|c0=c9"); err != nil {
		t.Fatalf("no fallback: %v", err)
	}
	if c, f := levels(odd); math.Abs(c/f-standard) > 0.05*standard {
		t.Errorf("the fallback is not the plain downmix: %.2f against %.2f", c/f, standard)
	}
}

// goertzel is the amplitude of one frequency in the samples.
func goertzel(x []float64, freq, rate float64) float64 {
	k := 2 * math.Cos(2*math.Pi*freq/rate)
	var s1, s2 float64
	for _, v := range x {
		s1, s2 = v+k*s1-s2, s1
	}
	return math.Sqrt(s1*s1+s2*s2-k*s1*s2) * 2 / float64(len(x))
}
