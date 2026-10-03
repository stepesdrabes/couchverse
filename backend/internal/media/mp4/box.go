// Package mp4 reads the parts of ISO base media files (MP4, fragmented MP4) that
// HLS packaging and validation need: track and codec facts from an init section,
// timing from media segments, and the keyframe index of a progressive file.
package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Box is one ISO BMFF box inside a buffer.
type Box struct {
	Type string
	// Payload is the box body after the header (and, for a full box, still
	// including version and flags).
	Payload []byte
	// Offset and Size locate the whole box (header included) in the buffer.
	Offset, Size int
}

var errTruncated = errors.New("mp4: truncated box")

// Boxes splits a buffer into its consecutive boxes.
func Boxes(data []byte) ([]Box, error) {
	var out []Box
	for off := 0; off < len(data); {
		if len(data)-off < 8 {
			return out, errTruncated
		}
		size := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		header := 8
		switch size {
		case 0:
			size = len(data) - off
		case 1:
			if len(data)-off < 16 {
				return out, errTruncated
			}
			size64 := binary.BigEndian.Uint64(data[off+8:])
			if size64 > uint64(len(data)-off) {
				return out, errTruncated
			}
			size, header = int(size64), 16
		}
		if size < header || off+size > len(data) {
			return out, fmt.Errorf("mp4: box %q overruns its parent", typ)
		}
		out = append(out, Box{Type: typ, Payload: data[off+header : off+size], Offset: off, Size: size})
		off += size
	}
	return out, nil
}

// Child returns the payload of the first child box of the given type.
func Child(data []byte, typ string) []byte {
	boxes, _ := Boxes(data)
	for _, b := range boxes {
		if b.Type == typ {
			return b.Payload
		}
	}
	return nil
}

// Path follows a chain of container box types and returns the payload of the
// last one, or nil when any step is missing.
func Path(data []byte, types ...string) []byte {
	for _, t := range types {
		if data = Child(data, t); data == nil {
			return nil
		}
	}
	return data
}

// Children returns the payloads of every child box of the given type.
func Children(data []byte, typ string) [][]byte {
	boxes, _ := Boxes(data)
	var out [][]byte
	for _, b := range boxes {
		if b.Type == typ {
			out = append(out, b.Payload)
		}
	}
	return out
}

// reader walks a box payload field by field; reads past the end yield zero
// and set err, so parsers check once at the end.
type reader struct {
	b   []byte
	off int
	err error
}

func (r *reader) take(n int) []byte {
	if r.err != nil || r.off+n > len(r.b) {
		r.err = errTruncated
		return make([]byte, n)
	}
	v := r.b[r.off : r.off+n]
	r.off += n
	return v
}

func (r *reader) skip(n int)   { r.take(n) }
func (r *reader) u8() uint8    { return r.take(1)[0] }
func (r *reader) u16() uint16  { return binary.BigEndian.Uint16(r.take(2)) }
func (r *reader) u32() uint32  { return binary.BigEndian.Uint32(r.take(4)) }
func (r *reader) u64() uint64  { return binary.BigEndian.Uint64(r.take(8)) }
func (r *reader) rest() []byte { return r.b[min(r.off, len(r.b)):] }
func (r *reader) fullBox() (uint8, uint32) {
	v := r.u32()
	return uint8(v >> 24), v & 0xffffff
}
