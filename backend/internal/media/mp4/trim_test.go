package mp4

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTrim(t *testing.T) {
	in := readInit(t, "aac")
	seg, err := os.ReadFile("testdata/aac.seg.m4s")
	if err != nil {
		t.Fatal(err)
	}
	whole, err := ParseSegment(seg, in)
	if err != nil {
		t.Fatal(err)
	}
	scale := float64(in.Tracks[0].Timescale)
	full := float64(whole[0].Duration) / scale

	cut, err := Trim(seg, in, full/2)
	if err != nil {
		t.Fatal(err)
	}
	frags, err := ParseSegment(cut, in)
	if err != nil {
		t.Fatal(err)
	}
	sample := full / float64(whole[0].Samples)
	if got := float64(frags[0].Duration) / scale; math.Abs(got-full/2) > sample/2+1e-9 {
		t.Errorf("trimmed to %.4fs, want %.4fs (+-%.4f)", got, full/2, sample/2)
	}
	if frags[0].BaseDecodeTime != whole[0].BaseDecodeTime || !frags[0].FirstSync || frags[0].Samples >= whole[0].Samples {
		t.Errorf("trimmed fragment %+v, whole %+v", frags[0], whole[0])
	}
	if len(cut) >= len(seg) {
		t.Errorf("trimmed segment is %d bytes, the whole one %d", len(cut), len(seg))
	}
	if same, err := Trim(seg, in, full+1); err != nil || !bytes.Equal(same, seg) {
		t.Errorf("a segment shorter than the cut changed (%v)", err)
	}

	// ffmpeg reads the trimmed samples back
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return
	}
	init, _ := os.ReadFile("testdata/aac.init.mp4")
	path := filepath.Join(t.TempDir(), "trimmed.mp4")
	if err := os.WriteFile(path, append(init, cut...), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-count_packets", "-select_streams", "a:0",
		"-show_entries", "stream=nb_read_packets", "-of", "csv=p=0", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != strconv.Itoa(frags[0].Samples) {
		t.Errorf("ffprobe reads %q packets (%v), want %d", out, err, frags[0].Samples)
	}
}
