package mp4

import (
	"bytes"
	"os"
	"testing"
)

func TestRetime(t *testing.T) {
	for _, name := range []string{"avc", "aac"} {
		in := readInit(t, name)
		seg, err := os.ReadFile("testdata/" + name + ".seg.m4s")
		if err != nil {
			t.Fatal(err)
		}
		whole, err := ParseSegment(seg, in)
		if err != nil {
			t.Fatal(err)
		}
		at := whole[0].BaseDecodeTime + 7*whole[0].Duration
		moved, err := Retime(seg, at, 8)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		frags, err := ParseSegment(moved, in)
		if err != nil {
			t.Fatal(err)
		}
		f := frags[0]
		if f.BaseDecodeTime != at || f.Duration != whole[0].Duration || f.Samples != whole[0].Samples || f.FirstSync != whole[0].FirstSync {
			t.Errorf("%s: retimed fragment %+v, whole %+v", name, f, whole[0])
		}
		if mfhd := Path(moved[bytes.Index(moved, []byte("moof"))+4:], "mfhd"); len(mfhd) < 8 || mfhd[7] != 8 {
			t.Errorf("%s: sequence number not rewritten: %x", name, mfhd)
		}
		// the samples are untouched, so trimming still works on the copy
		if _, err := Trim(moved, in, float64(f.Duration)/float64(in.Tracks[0].Timescale)/2); err != nil {
			t.Errorf("%s: trim a retimed segment: %v", name, err)
		}
		if bytes.Equal(moved, seg) {
			t.Errorf("%s: nothing changed", name)
		}
	}
	if _, err := Retime([]byte("\x00\x00\x00\x08free"), 0, 1); err == nil {
		t.Error("a segment without a fragment")
	}
}
