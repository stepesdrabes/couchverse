package mp4

import (
	"os"
	"testing"
)

func readInit(t *testing.T, name string) *Init {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name + ".init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	in, err := ParseInit(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return in
}

func TestParseInit(t *testing.T) {
	cases := []struct {
		name string
		want Track
	}{
		{name: "avc", want: Track{Handler: "vide", SampleEntry: "avc1", Codec: "avc1.42c00a", Width: 128, Height: 72}},
		{name: "hevc10", want: Track{Handler: "vide", SampleEntry: "hvc1", Codec: "hvc1.2.4.L30.90", Width: 128, Height: 72}},
		{name: "av1", want: Track{Handler: "vide", SampleEntry: "av01", Codec: "av01.0.00M.08", Width: 128, Height: 72}},
		{name: "aac", want: Track{Handler: "soun", SampleEntry: "mp4a", Codec: "mp4a.40.2", Channels: 2}},
		{name: "ac3", want: Track{Handler: "soun", SampleEntry: "ac-3", Codec: "ac-3", Channels: 6}},
		{name: "eac3", want: Track{Handler: "soun", SampleEntry: "ec-3", Codec: "ec-3", Channels: 6}},
		// profile 8.1 stays hvc1 so HDR10 players decode it; the DV layer is supplemental
		{name: "dv81", want: Track{Handler: "vide", SampleEntry: "hvc1", Codec: "hvc1.2.4.L120.90",
			SupplementalCodec: "dvh1.08.03/db1p", Width: 1920, Height: 1080, Transfer: 16, DoviProfile: 8, DoviLevel: 3, DoviCompatibility: 1}},
		{name: "dv5", want: Track{Handler: "vide", SampleEntry: "dvh1", Codec: "dvh1.05.03",
			Width: 1920, Height: 1080, Transfer: 16, DoviProfile: 5, DoviLevel: 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := readInit(t, c.name)
			if len(in.Tracks) != 1 {
				t.Fatalf("tracks = %d, want 1", len(in.Tracks))
			}
			got := in.Tracks[0]
			if got.Timescale == 0 || got.ID == 0 {
				t.Errorf("timescale %d, id %d: want both set", got.Timescale, got.ID)
			}
			got.ID, got.Timescale, got.DefaultSampleDuration, got.DefaultSampleFlags = 0, 0, 0, 0
			if got != c.want {
				t.Errorf("got  %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestParseSegment(t *testing.T) {
	for _, c := range []struct {
		name     string
		duration float64 // seconds
	}{{"avc", 1}, {"hevc10", 1}, {"aac", 1.024}, {"eac3", 1.024}} {
		t.Run(c.name, func(t *testing.T) {
			in := readInit(t, c.name)
			data, err := os.ReadFile("testdata/" + c.name + ".seg.m4s")
			if err != nil {
				t.Fatal(err)
			}
			frags, err := ParseSegment(data, in)
			if err != nil {
				t.Fatal(err)
			}
			f := frags[0]
			if !f.FirstSync {
				t.Error("first sample is not a sync sample")
			}
			if f.BaseDecodeTime != 0 {
				t.Errorf("tfdt = %d, want 0 for the first segment", f.BaseDecodeTime)
			}
			got := float64(f.Duration) / float64(in.Track(f.TrackID).Timescale)
			if got < c.duration-0.05 || got > c.duration+0.05 {
				t.Errorf("duration %.3f, want about %.3f", got, c.duration)
			}
		})
	}
}

func TestHEVCCodecString(t *testing.T) {
	// Main 10, high tier, level 5.1 with progressive/frame-only constraints
	hvcC := []byte{1, 0x22, 0x20, 0, 0, 0, 0xb0, 0, 0, 0, 0, 0, 153}
	if got := hevcCodec("hvc1", hvcC); got != "hvc1.2.4.H153.B0" {
		t.Errorf("got %s", got)
	}
}

func TestEC3Atmos(t *testing.T) {
	// one independent 5.1 substream followed by the type A extension (JOC)
	dec3 := []byte{0x02, 0x80, 0x20, 0x0f, 0x00, 0x01, 0x10}
	ch, joc := ec3Info(dec3)
	if ch != 6 || !joc {
		t.Errorf("channels %d joc %v, want 6 true", ch, joc)
	}
}
