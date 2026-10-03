// Package hls models HLS playlists (RFC 8216 and Apple's HLS authoring spec):
// it writes the multivariant and media playlists the server generates, parses
// them back, and validates a whole presentation the way AVPlayer will judge it.
package hls

import (
	"fmt"
	"strconv"
	"strings"
)

// Multivariant is a multivariant (master) playlist.
type Multivariant struct {
	Version             int
	IndependentSegments bool
	Renditions          []Rendition
	Variants            []Variant
	IFrameVariants      []IFrameVariant
}

// Rendition is an EXT-X-MEDIA tag.
type Rendition struct {
	Type       string // AUDIO, SUBTITLES, CLOSED-CAPTIONS
	GroupID    string
	Name       string
	Language   string
	Default    bool
	Autoselect bool
	Forced     bool
	// Channels is the CHANNELS attribute of an audio rendition, e.g. "6" or "16/JOC".
	Channels string
	URI      string
}

// Variant is an EXT-X-STREAM-INF tag and its URI.
type Variant struct {
	Bandwidth          int64
	AverageBandwidth   int64
	Codecs             []string
	SupplementalCodecs string
	Width, Height      int
	FrameRate          float64
	VideoRange         string // SDR, PQ, HLG
	Audio              string
	Subtitles          string
	// ClosedCaptions is the CLOSED-CAPTIONS group, or "NONE".
	ClosedCaptions string
	URI            string
}

// IFrameVariant is an EXT-X-I-FRAME-STREAM-INF tag.
type IFrameVariant struct {
	Bandwidth        int64
	AverageBandwidth int64
	Codecs           []string
	Width, Height    int
	VideoRange       string
	URI              string
}

// Media is a media playlist.
type Media struct {
	Version             int
	TargetDuration      int
	MediaSequence       int
	PlaylistType        string // VOD, EVENT or empty
	IndependentSegments bool
	IFramesOnly         bool
	Segments            []Segment
	EndList             bool
}

// Segment is one EXTINF entry.
type Segment struct {
	Duration float64
	URI      string
	// ByteRange limits the segment to part of its resource; zero length means the whole resource.
	ByteRange ByteRange
	// Map is the init section in effect for this segment (EXT-X-MAP), if any.
	Map *Map
	// Discontinuity marks an EXT-X-DISCONTINUITY before the segment.
	Discontinuity bool
}

// Map is an EXT-X-MAP tag.
type Map struct {
	URI       string
	ByteRange ByteRange
}

// ByteRange is an EXT-X-BYTERANGE (or BYTERANGE attribute) value.
type ByteRange struct {
	Length, Offset int64
}

func (b ByteRange) String() string {
	return fmt.Sprintf("%d@%d", b.Length, b.Offset)
}

// String renders the playlist.
func (m *Multivariant) String() string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", max(m.Version, 1))
	if m.IndependentSegments {
		b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}
	for _, r := range m.Renditions {
		attrs := attrList{}
		attrs.enum("TYPE", r.Type)
		attrs.quoted("GROUP-ID", r.GroupID)
		attrs.quoted("NAME", r.Name)
		if r.Language != "" {
			attrs.quoted("LANGUAGE", r.Language)
		}
		attrs.enum("DEFAULT", yesNo(r.Default))
		attrs.enum("AUTOSELECT", yesNo(r.Autoselect))
		if r.Type == "SUBTITLES" {
			attrs.enum("FORCED", yesNo(r.Forced))
		}
		if r.Channels != "" {
			attrs.quoted("CHANNELS", r.Channels)
		}
		if r.URI != "" {
			attrs.quoted("URI", r.URI)
		}
		b.WriteString("#EXT-X-MEDIA:" + attrs.String() + "\n")
	}
	for _, v := range m.Variants {
		attrs := attrList{}
		attrs.int("BANDWIDTH", v.Bandwidth)
		if v.AverageBandwidth > 0 {
			attrs.int("AVERAGE-BANDWIDTH", v.AverageBandwidth)
		}
		attrs.quoted("CODECS", strings.Join(v.Codecs, ","))
		if v.SupplementalCodecs != "" {
			attrs.quoted("SUPPLEMENTAL-CODECS", v.SupplementalCodecs)
		}
		if v.Width > 0 {
			attrs.enum("RESOLUTION", fmt.Sprintf("%dx%d", v.Width, v.Height))
		}
		if v.FrameRate > 0 {
			attrs.enum("FRAME-RATE", strconv.FormatFloat(v.FrameRate, 'f', 3, 64))
		}
		if v.VideoRange != "" {
			attrs.enum("VIDEO-RANGE", v.VideoRange)
		}
		if v.Audio != "" {
			attrs.quoted("AUDIO", v.Audio)
		}
		if v.Subtitles != "" {
			attrs.quoted("SUBTITLES", v.Subtitles)
		}
		if v.ClosedCaptions == "NONE" {
			attrs.enum("CLOSED-CAPTIONS", "NONE")
		} else if v.ClosedCaptions != "" {
			attrs.quoted("CLOSED-CAPTIONS", v.ClosedCaptions)
		}
		b.WriteString("#EXT-X-STREAM-INF:" + attrs.String() + "\n" + v.URI + "\n")
	}
	for _, v := range m.IFrameVariants {
		attrs := attrList{}
		attrs.int("BANDWIDTH", v.Bandwidth)
		if v.AverageBandwidth > 0 {
			attrs.int("AVERAGE-BANDWIDTH", v.AverageBandwidth)
		}
		attrs.quoted("CODECS", strings.Join(v.Codecs, ","))
		attrs.enum("RESOLUTION", fmt.Sprintf("%dx%d", v.Width, v.Height))
		if v.VideoRange != "" {
			attrs.enum("VIDEO-RANGE", v.VideoRange)
		}
		attrs.quoted("URI", v.URI)
		b.WriteString("#EXT-X-I-FRAME-STREAM-INF:" + attrs.String() + "\n")
	}
	return b.String()
}

// String renders the playlist.
func (m *Media) String() string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", max(m.Version, 1))
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", m.TargetDuration)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", m.MediaSequence)
	if m.PlaylistType != "" {
		b.WriteString("#EXT-X-PLAYLIST-TYPE:" + m.PlaylistType + "\n")
	}
	if m.IndependentSegments {
		b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}
	if m.IFramesOnly {
		b.WriteString("#EXT-X-I-FRAMES-ONLY\n")
	}
	var current *Map
	for _, s := range m.Segments {
		if s.Discontinuity {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if s.Map != nil && (current == nil || *current != *s.Map) {
			attrs := attrList{}
			attrs.quoted("URI", s.Map.URI)
			if s.Map.ByteRange.Length > 0 {
				attrs.quoted("BYTERANGE", s.Map.ByteRange.String())
			}
			b.WriteString("#EXT-X-MAP:" + attrs.String() + "\n")
			current = s.Map
		}
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", s.Duration)
		if s.ByteRange.Length > 0 {
			b.WriteString("#EXT-X-BYTERANGE:" + s.ByteRange.String() + "\n")
		}
		b.WriteString(s.URI + "\n")
	}
	if m.EndList {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	return b.String()
}

// TargetDuration is the EXT-X-TARGETDURATION covering the given segment
// durations: the longest one rounded to the nearest integer (RFC 8216 4.3.3.1).
func TargetDuration(durations ...float64) int {
	target := 1
	for _, d := range durations {
		target = max(target, int(d+0.5))
	}
	return target
}

func yesNo(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}

type attrList []string

func (a *attrList) quoted(name, value string) { *a = append(*a, name+`="`+value+`"`) }
func (a *attrList) enum(name, value string)   { *a = append(*a, name+"="+value) }
func (a *attrList) int(name string, v int64)  { *a = append(*a, name+"="+strconv.FormatInt(v, 10)) }
func (a attrList) String() string             { return strings.Join(a, ",") }
