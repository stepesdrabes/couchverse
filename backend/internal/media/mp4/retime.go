package mp4

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Retime moves a media segment to another decode time: every track fragment's
// tfdt becomes decodeTime (in the track's timescale) and the fragment sequence
// number seq. Played one after another, copies of one segment retimed n times
// its duration form a continuous stream (the Couch TV slate loops this way).
// A sidx would index the old times, so it is dropped.
func Retime(data []byte, decodeTime uint64, seq uint32) ([]byte, error) {
	boxes, err := Boxes(data)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(data))
	moofs := 0
	for _, b := range boxes {
		if b.Type == "sidx" {
			continue
		}
		start := len(out)
		out = append(out, data[b.Offset:b.Offset+b.Size]...)
		if b.Type != "moof" {
			continue
		}
		moofs++
		moof := out[start : start+b.Size]
		header := b.Size - len(b.Payload)
		children, err := Boxes(moof[header:])
		if err != nil {
			return nil, err
		}
		for _, c := range children {
			body := moof[header+c.Offset+c.Size-len(c.Payload) : header+c.Offset+c.Size]
			switch c.Type {
			case "mfhd":
				if len(body) < 8 {
					return nil, errTruncated
				}
				binary.BigEndian.PutUint32(body[4:], seq)
			case "traf":
				if err := setTfdt(body, decodeTime); err != nil {
					return nil, err
				}
			}
		}
	}
	if moofs == 0 {
		return nil, fmt.Errorf("mp4: segment has no movie fragment")
	}
	return out, nil
}

// setTfdt rewrites the base media decode time of a traf payload in place.
func setTfdt(traf []byte, decodeTime uint64) error {
	boxes, err := Boxes(traf)
	if err != nil {
		return err
	}
	for _, b := range boxes {
		if b.Type != "tfdt" {
			continue
		}
		body := traf[b.Offset+b.Size-len(b.Payload) : b.Offset+b.Size]
		switch {
		case len(body) >= 12 && body[0] == 1:
			binary.BigEndian.PutUint64(body[4:], decodeTime)
		case len(body) >= 8 && body[0] == 0 && decodeTime <= math.MaxUint32:
			binary.BigEndian.PutUint32(body[4:], uint32(decodeTime))
		default:
			return fmt.Errorf("mp4: tfdt cannot hold %d", decodeTime)
		}
		return nil
	}
	return fmt.Errorf("mp4: traf without tfdt")
}
