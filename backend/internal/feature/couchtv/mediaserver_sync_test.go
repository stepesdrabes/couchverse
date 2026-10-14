package couchtv

import (
	"context"
	"encoding/binary"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A scene's frames and its sound are cut in two separate ffmpeg runs, each seeking to the scene's
// start. They must land on the same presentation timeline, or the dialogue is out of lip sync: here
// a file whose audio starts 0.3 s after its picture (a remux's usual offset), one whose picture
// does, and one where both start late, each with a flash in the picture and a beep in the sound
// that the container places a known time apart. The cuts keep exactly that time between them.
func TestSceneCutsKeepTheContainersSync(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(ffmpeg, append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("ffmpeg cannot build the test file (%v): %s", err, out)
		}
	}
	dir := t.TempDir()
	// 12 s: a black picture with a white flash from 6.0 to 6.2 s, a silent sound with a beep then
	video, audio := filepath.Join(dir, "v.mkv"), filepath.Join(dir, "a.flac")
	run("-f", "lavfi", "-i", "color=c=black:s=160x90:r=25:d=12,drawbox=x=0:y=0:w=160:h=90:color=white:t=fill:enable='between(t,6,6.2)'",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-bf", "2", video)
	run("-f", "lavfi", "-i", `aevalsrc=if(between(t\,6\,6.2)\,0.8*sin(2*PI*1000*t)\,0):d=12:s=48000`, "-c:a", "flac", audio)

	const start = 5 * time.Second
	m := &mediaServer{ffmpeg: ffmpeg}
	for _, c := range []struct {
		name         string
		audioStart   float64 // seconds the audio's first timestamp is after zero
		pictureStart float64
	}{
		{"in step", 0, 0},
		{"audio 0.3 s late", 0.3, 0},
		{"picture 0.5 s late", 0, 0.5},
		{"both late, the audio more", 0.7, 0.4},
	} {
		src := filepath.Join(dir, c.name+".mkv")
		run("-itsoffset", strconv.FormatFloat(c.pictureStart, 'f', 3, 64), "-i", video,
			"-itsoffset", strconv.FormatFloat(c.audioStart, 'f', 3, 64), "-i", audio, "-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "flac", src)

		frames := filepath.Join(dir, c.name+"-frames")
		if err := m.cutFrames(ctx, src, "fps=25,scale='iw*sar':ih,setsar=1", start, 100, frames); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		sound := filepath.Join(dir, c.name+".flac")
		if err := m.cutAudio(ctx, src, "0:a:0", start, 4*time.Second, sound, ""); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		flash, beep := flashAt(t, frames), beepAt(t, ffmpeg, sound)
		// the container's own timeline starts at its earliest stream
		origin := math.Min(c.audioStart, c.pictureStart)
		wantFlash := 6.0 + c.pictureStart - origin - start.Seconds()
		wantBeep := 6.0 + c.audioStart - origin - start.Seconds()
		if math.Abs(flash-wantFlash) > 0.045 || math.Abs(beep-wantBeep) > 0.03 {
			t.Errorf("%s: the flash is %.3f s into the frames (want %.3f) and the beep %.3f s into the sound (want %.3f)",
				c.name, flash, wantFlash, beep, wantBeep)
		}
		if gap, want := beep-flash, c.audioStart-c.pictureStart; math.Abs(gap-want) > 0.06 {
			t.Errorf("%s: the beep is %.3f s after the flash, the file puts it %.3f s after", c.name, gap, want)
		}
	}
}

// flashAt is the time (25 fps) of the first frame in dir that is mostly white.
func flashAt(t *testing.T, dir string) float64 {
	t.Helper()
	for i := range 100 {
		f, err := os.Open(filepath.Join(dir, strconv.Itoa(i)+".jpg"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if r, _, _, _ := img.At(80, 45).RGBA(); r > 0x8000 {
			return float64(i) / FPS
		}
	}
	t.Fatal("no flash in the frames")
	return 0
}

// beepAt is the time of the first loud sample of a sound file.
func beepAt(t *testing.T, ffmpeg, path string) float64 {
	t.Helper()
	raw, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-ac", "1", "-ar", "48000", "-f", "s16le", "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(raw); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(raw[i:])); v > 8000 || v < -8000 {
			return float64(i/2) / 48000
		}
	}
	t.Fatal("no beep in the sound")
	return 0
}
