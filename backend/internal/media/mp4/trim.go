package mp4

import (
	"encoding/binary"
	"fmt"
)

// Trim cuts a single-track media segment short: it keeps the samples whose
// middle falls in the first seconds of the segment and drops the rest, so a
// rendition can end where another one (the video of the same item) does.
// It is meant for streams of sync samples, audio above all: a video segment
// cut this way may lose frames that kept ones are displayed after.
func Trim(data []byte, in *Init, seconds float64) ([]byte, error) {
	boxes, err := Boxes(data)
	if err != nil {
		return nil, err
	}
	var (
		out     []byte
		elapsed float64
		done    bool
	)
	for i := 0; i < len(boxes); i++ {
		b := boxes[i]
		switch {
		case b.Type == "sidx":
			continue // it would index the samples that go
		case b.Type != "moof":
			out = append(out, data[b.Offset:b.Offset+b.Size]...)
			continue
		}
		if i+1 >= len(boxes) || boxes[i+1].Type != "mdat" {
			return nil, fmt.Errorf("mp4: moof without its mdat")
		}
		mdat := boxes[i+1]
		i++
		if done {
			continue
		}
		f, err := trimFragment(data[b.Offset:b.Offset+b.Size], mdat.Payload, in, seconds-elapsed)
		if err != nil {
			return nil, err
		}
		elapsed += f.kept
		done = f.cut
		switch {
		case !f.cut:
			out = append(out, data[b.Offset:mdat.Offset+mdat.Size]...)
		case f.moof != nil:
			out = append(out, f.moof...)
			out = append(out, box("mdat", f.mdat)...)
		}
	}
	if !done {
		return data, nil
	}
	return out, nil
}

func box(typ string, payload []byte) []byte {
	out := make([]byte, 8, 8+len(payload))
	binary.BigEndian.PutUint32(out, uint32(8+len(payload)))
	copy(out[4:], typ)
	return append(out, payload...)
}

// trimmed is one fragment cut short: its new moof and mdat payload (nil when
// no sample is left), the seconds it keeps and whether samples were dropped.
type trimmed struct {
	moof, mdat []byte
	kept       float64
	cut        bool
}

// trimFragment rewrites one moof (whole box) and its mdat payload to the
// samples whose middle comes before seconds.
func trimFragment(moof, mdat []byte, in *Init, seconds float64) (trimmed, error) {
	var out trimmed
	children, err := Boxes(moof[8:])
	if err != nil {
		return out, err
	}
	var traf *Box
	for i, c := range children {
		if c.Type == "traf" {
			if traf != nil {
				return out, fmt.Errorf("mp4: trimming needs one track per fragment")
			}
			traf = &children[i]
		}
	}
	if traf == nil {
		return out, fmt.Errorf("mp4: moof without traf")
	}
	tfhd := Child(traf.Payload, "tfhd")
	if tfhd == nil {
		return out, fmt.Errorf("mp4: traf without tfhd")
	}
	r := &reader{b: tfhd}
	_, tfFlags := r.fullBox()
	t := in.Track(r.u32())
	if t == nil || t.Timescale == 0 {
		return out, fmt.Errorf("mp4: fragment of a track the init section lacks")
	}
	if tfFlags&0x01 != 0 {
		return out, fmt.Errorf("mp4: trimming does not follow an explicit base data offset")
	}
	defDuration, defSize := t.DefaultSampleDuration, uint32(0)
	if tfFlags&0x02 != 0 {
		r.skip(4)
	}
	if tfFlags&0x08 != 0 {
		defDuration = r.u32()
	}
	if tfFlags&0x10 != 0 {
		defSize = r.u32()
	}
	if r.err != nil {
		return out, r.err
	}
	cut := seconds * float64(t.Timescale)

	var (
		newTraf    []byte
		dataOffset int64
		keptBytes  int64
		runs       int
		kept       int
		ticks      float64
	)
	for _, c := range mustBoxes(traf.Payload) {
		switch c.Type {
		case "trun":
		case "sbgp", "saiz", "saio", "senc", "sdtp":
			return out, fmt.Errorf("mp4: trimming does not rewrite %s", c.Type)
		default:
			newTraf = append(newTraf, traf.Payload[c.Offset:c.Offset+c.Size]...)
			continue
		}
		if runs++; runs > 1 {
			return out, fmt.Errorf("mp4: trimming needs one track run per fragment")
		}
		rr := &reader{b: c.Payload}
		_, flags := rr.fullBox()
		count := int(rr.u32())
		if flags&0x01 == 0 {
			return out, fmt.Errorf("mp4: track run without a data offset")
		}
		dataOffset = int64(int32(rr.u32()))
		header := 12
		if flags&0x04 != 0 {
			header += 4
		}
		entry := 0
		for _, bit := range []uint32{0x100, 0x200, 0x400, 0x800} {
			if flags&bit != 0 {
				entry += 4
			}
		}
		if flags&0x200 == 0 && defSize == 0 {
			return out, fmt.Errorf("mp4: sample sizes unknown")
		}
		for s := 0; s < count; s++ {
			e := &reader{b: c.Payload[min(header+s*entry, len(c.Payload)):]}
			duration, size := defDuration, defSize
			if flags&0x100 != 0 {
				duration = e.u32()
			}
			if flags&0x200 != 0 {
				size = e.u32()
			}
			if e.err != nil {
				return out, e.err
			}
			if ticks+float64(duration)/2 >= cut {
				out.cut = true
				break
			}
			ticks += float64(duration)
			keptBytes += int64(size)
			kept++
		}
		run := append([]byte{}, c.Payload[:header+kept*entry]...)
		binary.BigEndian.PutUint32(run[4:], uint32(kept))
		newTraf = append(newTraf, box("trun", run)...)
	}
	out.kept = ticks / float64(t.Timescale)
	if kept == 0 {
		return out, nil
	}
	var newMoof []byte
	for _, c := range children {
		if c.Type == "traf" {
			newMoof = append(newMoof, box("traf", newTraf)...)
		} else {
			newMoof = append(newMoof, moof[8+c.Offset:8+c.Offset+c.Size]...)
		}
	}
	newMoof = box("moof", newMoof)
	// the samples sit at the run's data offset from the moof's first byte
	from := dataOffset - int64(len(moof)) - 8
	if from < 0 || from+keptBytes > int64(len(mdat)) {
		return out, fmt.Errorf("mp4: samples outside the mdat")
	}
	binary.BigEndian.PutUint32(newMoof[trunDataOffset(newMoof):], uint32(int32(dataOffset-int64(len(moof))+int64(len(newMoof)))))
	out.moof, out.mdat = newMoof, mdat[:from+keptBytes]
	return out, nil
}

func mustBoxes(data []byte) []Box {
	boxes, _ := Boxes(data)
	return boxes
}

// trunDataOffset locates the data_offset field of the track run in a moof
// written by box (plain 8-byte headers).
func trunDataOffset(moof []byte) int {
	for _, traf := range mustBoxes(moof[8:]) {
		if traf.Type != "traf" {
			continue
		}
		for _, c := range mustBoxes(traf.Payload) {
			if c.Type == "trun" {
				// moof and traf headers, then the trun's header, version and flags, sample count
				return 8 + traf.Offset + 8 + c.Offset + 8 + 4 + 4
			}
		}
	}
	return 0
}
