package mp4

import (
	"fmt"
	"math/bits"
	"strings"
)

// avcCodec builds avc1.PPCCLL from an avcC box.
func avcCodec(entry string, avcC []byte) string {
	if len(avcC) < 4 {
		return entry
	}
	return fmt.Sprintf("%s.%02x%02x%02x", entry, avcC[1], avcC[2], avcC[3])
}

// hevcCodec builds the ISO/IEC 14496-15 Annex E string, e.g. hvc1.2.4.L120.90.
func hevcCodec(entry string, hvcC []byte) string {
	if len(hvcC) < 13 {
		return entry
	}
	space := hvcC[1] >> 6
	tier := "L"
	if hvcC[1]&0x20 != 0 {
		tier = "H"
	}
	profile := hvcC[1] & 0x1f
	compat := bits.Reverse32(uint32(hvcC[2])<<24 | uint32(hvcC[3])<<16 | uint32(hvcC[4])<<8 | uint32(hvcC[5]))
	level := hvcC[12]

	var b strings.Builder
	b.WriteString(entry + ".")
	if space > 0 {
		b.WriteByte("ABC"[space-1])
	}
	fmt.Fprintf(&b, "%d.%X.%s%d", profile, compat, tier, level)
	constraints := hvcC[6:12]
	last := len(constraints)
	for last > 0 && constraints[last-1] == 0 {
		last--
	}
	for _, c := range constraints[:last] {
		fmt.Fprintf(&b, ".%X", c)
	}
	return b.String()
}

// doviCodec builds dvh1.PP.LL.
func doviCodec(entry string, profile, level int) string {
	return fmt.Sprintf("%s.%02d.%02d", entry, profile, level)
}

// doviBrand is the compatibility brand Apple's HLS spec appends to a
// SUPPLEMENTAL-CODECS entry: the base layer is HDR10 (db1p), SDR (db2g) or HLG (db4h).
func doviBrand(compatibility int) string {
	switch compatibility {
	case 1:
		return "/db1p"
	case 2:
		return "/db2g"
	case 4:
		return "/db4h"
	}
	return ""
}

// av1Codec builds av01.P.LLT.DD from an av1C box.
func av1Codec(av1C []byte) string {
	if len(av1C) < 3 {
		return "av01"
	}
	profile := av1C[1] >> 5
	level := av1C[1] & 0x1f
	tier := "M"
	if av1C[2]&0x80 != 0 {
		tier = "H"
	}
	depth := 8
	if av1C[2]&0x40 != 0 {
		depth = 10
		if av1C[2]&0x20 != 0 {
			depth = 12
		}
	}
	return fmt.Sprintf("av01.%d.%02d%s.%02d", profile, level, tier, depth)
}

// mp4aCodec reads the object type from an esds box: mp4a.40.2 for AAC-LC.
func mp4aCodec(esds []byte) string {
	if len(esds) < 4 {
		return "mp4a"
	}
	r := &reader{b: esds[4:]}
	oti, asc := uint8(0), []byte(nil)
	for r.err == nil && len(r.rest()) > 0 {
		tag := r.u8()
		size := descriptorSize(r)
		body := r.take(size)
		switch tag {
		case 0x03: // ES_Descriptor: ES_ID, flags, then nested descriptors
			inner := &reader{b: body}
			inner.skip(2)
			flags := inner.u8()
			if flags&0x80 != 0 {
				inner.skip(2)
			}
			if flags&0x40 != 0 {
				inner.skip(int(inner.u8()))
			}
			if flags&0x20 != 0 {
				inner.skip(2)
			}
			r = &reader{b: inner.rest()}
		case 0x04: // DecoderConfigDescriptor
			if len(body) > 0 {
				oti = body[0]
			}
			if len(body) > 13 {
				r = &reader{b: body[13:]}
			}
		case 0x05: // DecoderSpecificInfo: the AudioSpecificConfig
			asc = body
		}
	}
	if oti == 0 {
		return "mp4a"
	}
	if oti != 0x40 || len(asc) == 0 {
		return fmt.Sprintf("mp4a.%x", oti)
	}
	aot := int(asc[0] >> 3)
	if aot == 31 && len(asc) > 1 {
		aot = 32 + int(asc[0]&7)<<3 | int(asc[1]>>5)
	}
	return fmt.Sprintf("mp4a.40.%d", aot)
}

func descriptorSize(r *reader) int {
	size := 0
	for i := 0; i < 4; i++ {
		b := r.u8()
		size = size<<7 | int(b&0x7f)
		if b&0x80 == 0 {
			break
		}
	}
	return size
}

// acmodChannels is the full-bandwidth channel count of each AC-3 audio coding mode.
var acmodChannels = [8]int{2, 1, 2, 3, 3, 4, 4, 5}

func ac3Channels(dac3 []byte) int {
	if len(dac3) < 3 {
		return 0
	}
	v := uint32(dac3[0])<<16 | uint32(dac3[1])<<8 | uint32(dac3[2])
	acmod := (v >> 11) & 7
	lfe := (v >> 10) & 1
	return acmodChannels[acmod] + int(lfe)
}

// ec3Info reads the channel count of the first independent substream and
// whether the stream carries Atmos (the type A extension flag) from a dec3 box.
func ec3Info(dec3 []byte) (channels int, joc bool) {
	if len(dec3) < 5 {
		return 0, false
	}
	numInd := int(dec3[1]&7) + 1
	// first substream: fscod(2) bsid(5) reserved(1) asvc(1) bsmod(3) acmod(3) lfeon(1)
	acmod := (dec3[3] >> 1) & 7
	lfe := dec3[3] & 1
	numDep := (dec3[4] >> 1) & 0x0f
	channels = acmodChannels[acmod] + int(lfe)
	// each independent substream takes 3 bytes, or 4 with dependent substreams
	off := 2
	for i := 0; i < numInd && off+3 <= len(dec3); i++ {
		dep := (dec3[off+2] >> 1) & 0x0f
		if i == 0 {
			numDep = dep
		}
		off += 3
		if dep > 0 {
			off++
		}
	}
	if numDep > 0 && len(dec3) >= 6 {
		// dependent substreams carry the channels beyond 5.1 (chan_loc)
		chanLoc := uint16(dec3[4]&1)<<8 | uint16(dec3[5])
		for _, extra := range []int{2, 2, 1, 1, 2, 2, 2, 1, 1} {
			if chanLoc&0x100 != 0 {
				channels += extra
			}
			chanLoc <<= 1
		}
	}
	if off+2 <= len(dec3) {
		joc = dec3[off]&1 != 0
	}
	return channels, joc
}
