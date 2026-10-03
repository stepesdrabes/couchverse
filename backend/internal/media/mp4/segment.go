package mp4

import (
	"fmt"
)

// Fragment is the timing of one track run in a media segment's moof.
type Fragment struct {
	TrackID uint32
	// BaseDecodeTime is the tfdt value, in the track's timescale.
	BaseDecodeTime uint64
	// Duration sums the sample durations, in the track's timescale.
	Duration uint64
	Samples  int
	// FirstSync reports whether the first sample is a sync sample (an IDR for
	// video), which makes the segment independently decodable.
	FirstSync bool
	// FirstCompositionOffset is the first sample's composition time offset.
	FirstCompositionOffset int64
}

// sampleIsNonSync is sample_is_non_sync_sample in the sample flags.
const sampleIsNonSync = 0x00010000

// ParseSegment reads every track fragment of a media segment (styp/sidx/moof/mdat...).
// Defaults missing from the fragments come from the init section.
func ParseSegment(data []byte, in *Init) ([]Fragment, error) {
	boxes, err := Boxes(data)
	if err != nil {
		return nil, err
	}
	var out []Fragment
	for _, b := range boxes {
		if b.Type != "moof" {
			continue
		}
		for _, traf := range Children(b.Payload, "traf") {
			f, err := parseTraf(traf, in)
			if err != nil {
				return nil, err
			}
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("mp4: segment has no movie fragment")
	}
	return out, nil
}

func parseTraf(traf []byte, in *Init) (Fragment, error) {
	var f Fragment
	tfhd := Child(traf, "tfhd")
	if tfhd == nil {
		return f, fmt.Errorf("mp4: traf without tfhd")
	}
	r := &reader{b: tfhd}
	_, flags := r.fullBox()
	f.TrackID = r.u32()
	var defDuration, defFlags uint32
	if t := in.Track(f.TrackID); t != nil {
		defDuration, defFlags = t.DefaultSampleDuration, t.DefaultSampleFlags
	}
	if flags&0x01 != 0 {
		r.skip(8)
	}
	if flags&0x02 != 0 {
		r.skip(4)
	}
	if flags&0x08 != 0 {
		defDuration = r.u32()
	}
	if flags&0x10 != 0 {
		r.skip(4)
	}
	if flags&0x20 != 0 {
		defFlags = r.u32()
	}
	if r.err != nil {
		return f, r.err
	}

	if tfdt := Child(traf, "tfdt"); tfdt != nil {
		r := &reader{b: tfdt}
		if version, _ := r.fullBox(); version == 1 {
			f.BaseDecodeTime = r.u64()
		} else {
			f.BaseDecodeTime = uint64(r.u32())
		}
	}

	for i, trun := range Children(traf, "trun") {
		r := &reader{b: trun}
		version, flags := r.fullBox()
		count := int(r.u32())
		if flags&0x01 != 0 {
			r.skip(4)
		}
		firstFlags, hasFirst := uint32(0), flags&0x04 != 0
		if hasFirst {
			firstFlags = r.u32()
		}
		for s := 0; s < count && r.err == nil; s++ {
			duration, sampleFlags, cto := defDuration, defFlags, int64(0)
			if flags&0x100 != 0 {
				duration = r.u32()
			}
			if flags&0x200 != 0 {
				r.skip(4)
			}
			if flags&0x400 != 0 {
				sampleFlags = r.u32()
			}
			if flags&0x800 != 0 {
				raw := r.u32()
				if version == 1 {
					cto = int64(int32(raw))
				} else {
					cto = int64(raw)
				}
			}
			if i == 0 && s == 0 {
				if hasFirst {
					sampleFlags = firstFlags
				}
				f.FirstSync = sampleFlags&sampleIsNonSync == 0
				f.FirstCompositionOffset = cto
			}
			f.Duration += uint64(duration)
		}
		if r.err != nil {
			return f, r.err
		}
		f.Samples += count
	}
	return f, nil
}
