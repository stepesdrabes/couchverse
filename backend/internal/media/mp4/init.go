package mp4

import (
	"errors"
	"fmt"
)

// Track is what an init section (ftyp + moov) says about one track.
type Track struct {
	ID        uint32
	Handler   string // vide, soun, text, subt
	Timescale uint32
	// SampleEntry is the stsd entry type: avc1, hvc1, hev1, dvh1, dvhe, av01,
	// mp4a, ac-3, ec-3, Opus, fLaC...
	SampleEntry string
	// Codec is the RFC 6381 codec string for an HLS CODECS attribute.
	Codec string
	// SupplementalCodec is the Dolby Vision layer of a backward-compatible
	// (profile 8) stream, for SUPPLEMENTAL-CODECS; empty otherwise.
	SupplementalCodec string
	Width, Height     int
	// Transfer is the colr box's transfer characteristics (ITU-T H.273):
	// 16 is PQ, 18 is HLG; 0 when the init does not say.
	Transfer int
	Channels int
	// JOC marks E-AC-3 with joint object coding (Dolby Atmos).
	JOC bool
	// Dolby Vision configuration, zero when the track has none.
	DoviProfile, DoviLevel, DoviCompatibility int
	// DefaultSampleDuration and DefaultSampleFlags come from mvex/trex, the
	// fallback for fragments that do not repeat them.
	DefaultSampleDuration uint32
	DefaultSampleFlags    uint32
}

// Init is a parsed init section.
type Init struct {
	Tracks []Track
}

// Track returns the track with the given id.
func (in *Init) Track(id uint32) *Track {
	for i := range in.Tracks {
		if in.Tracks[i].ID == id {
			return &in.Tracks[i]
		}
	}
	return nil
}

// ErrNoMovie is returned for data without a moov box.
var ErrNoMovie = errors.New("mp4: no moov box")

// ParseInit reads an init section: a ftyp and a moov with at least one track.
func ParseInit(data []byte) (*Init, error) {
	moov := Child(data, "moov")
	if moov == nil {
		return nil, ErrNoMovie
	}
	in := &Init{}
	for _, trak := range Children(moov, "trak") {
		t, err := parseTrack(trak)
		if err != nil {
			return nil, err
		}
		in.Tracks = append(in.Tracks, t)
	}
	if len(in.Tracks) == 0 {
		return nil, fmt.Errorf("mp4: moov has no tracks")
	}
	for _, trex := range Children(Child(moov, "mvex"), "trex") {
		r := &reader{b: trex}
		r.fullBox()
		id := r.u32()
		r.skip(4) // default_sample_description_index
		duration := r.u32()
		r.skip(4) // default_sample_size
		flags := r.u32()
		if t := in.Track(id); t != nil && r.err == nil {
			t.DefaultSampleDuration, t.DefaultSampleFlags = duration, flags
		}
	}
	return in, nil
}

func parseTrack(trak []byte) (Track, error) {
	var t Track
	if tkhd := Child(trak, "tkhd"); tkhd != nil {
		r := &reader{b: tkhd}
		version, _ := r.fullBox()
		if version == 1 {
			r.skip(16)
		} else {
			r.skip(8)
		}
		t.ID = r.u32()
		// width and height close the box as 16.16 fixed point
		if len(tkhd) >= 8 {
			end := &reader{b: tkhd[len(tkhd)-8:]}
			t.Width, t.Height = int(end.u32()>>16), int(end.u32()>>16)
		}
	}
	mdia := Child(trak, "mdia")
	if mdhd := Child(mdia, "mdhd"); mdhd != nil {
		r := &reader{b: mdhd}
		version, _ := r.fullBox()
		if version == 1 {
			r.skip(16)
		} else {
			r.skip(8)
		}
		t.Timescale = r.u32()
	}
	if hdlr := Child(mdia, "hdlr"); len(hdlr) >= 12 {
		t.Handler = string(hdlr[8:12])
	}
	stsd := Path(mdia, "minf", "stbl", "stsd")
	if len(stsd) < 8 {
		return t, fmt.Errorf("mp4: track %d has no sample description", t.ID)
	}
	entries, err := Boxes(stsd[8:])
	if err != nil || len(entries) == 0 {
		return t, fmt.Errorf("mp4: track %d has a malformed sample description", t.ID)
	}
	entry := entries[0]
	t.SampleEntry = entry.Type
	switch t.Handler {
	case "vide":
		parseVisualEntry(&t, entry.Payload)
	case "soun":
		parseAudioEntry(&t, entry.Payload)
	default:
		t.Codec = entry.Type
	}
	return t, nil
}

// visualEntryHeader is the fixed part of a VisualSampleEntry before its child boxes.
const visualEntryHeader = 78

func parseVisualEntry(t *Track, entry []byte) {
	if len(entry) < visualEntryHeader {
		return
	}
	r := &reader{b: entry}
	r.skip(24)
	if w, h := int(r.u16()), int(r.u16()); w > 0 && h > 0 {
		t.Width, t.Height = w, h
	}
	children := entry[visualEntryHeader:]
	if colr := Child(children, "colr"); len(colr) >= 8 && string(colr[:4]) == "nclx" {
		t.Transfer = int(colr[6])<<8 | int(colr[7])
	}
	dovi := Child(children, "dvcC")
	if dovi == nil {
		dovi = Child(children, "dvvC")
	}
	if len(dovi) >= 4 {
		t.DoviProfile = int(dovi[2] >> 1)
		t.DoviLevel = int((dovi[2]&1)<<5 | dovi[3]>>3)
		if len(dovi) >= 5 {
			t.DoviCompatibility = int(dovi[4] >> 4)
		}
	}
	switch t.SampleEntry {
	case "avc1", "avc3":
		t.Codec = avcCodec(t.SampleEntry, Child(children, "avcC"))
	case "hvc1", "hev1":
		t.Codec = hevcCodec(t.SampleEntry, Child(children, "hvcC"))
		if t.DoviProfile > 0 {
			t.SupplementalCodec = doviCodec("dvh1", t.DoviProfile, t.DoviLevel) + doviBrand(t.DoviCompatibility)
		}
	case "dvh1", "dvhe":
		t.Codec = doviCodec(t.SampleEntry, t.DoviProfile, t.DoviLevel)
	case "av01":
		t.Codec = av1Codec(Child(children, "av1C"))
	default:
		t.Codec = t.SampleEntry
	}
}

// audioEntryHeader is the fixed part of a version 0 AudioSampleEntry.
const audioEntryHeader = 28

func parseAudioEntry(t *Track, entry []byte) {
	if len(entry) < audioEntryHeader {
		return
	}
	r := &reader{b: entry}
	r.skip(16)
	t.Channels = int(r.u16())
	children := entry[audioEntryHeader:]
	switch t.SampleEntry {
	case "mp4a":
		t.Codec = mp4aCodec(Child(children, "esds"))
	case "ac-3":
		t.Codec = "ac-3"
		if ch := ac3Channels(Child(children, "dac3")); ch > 0 {
			t.Channels = ch
		}
	case "ec-3":
		t.Codec = "ec-3"
		ch, joc := ec3Info(Child(children, "dec3"))
		if ch > 0 {
			t.Channels = ch
		}
		t.JOC = joc
	case "Opus":
		t.Codec = "opus"
	case "fLaC":
		t.Codec = "fLaC"
	default:
		t.Codec = t.SampleEntry
	}
}
