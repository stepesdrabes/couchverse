package hls

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A live channel plays a schedule of items (programs, breaks) back to back on
// the wall clock as one live presentation. Each item keeps its own segments in
// every rendition, cut on its own timeline, so the playlists put an
// EXT-X-DISCONTINUITY between items (players reset their timestamp mapping
// there) and an EXT-X-PROGRAM-DATE-TIME on every item start, which is how
// players line renditions up and how viewers agree on what is on.

// LiveItem is one entry of a live schedule.
type LiveItem struct {
	// Duration is the item's time on air. Every rendition is fitted to it, so
	// all of them change item at the same instant: segments that would start
	// in its last tenth of a second are left out and the last EXTINF ends
	// exactly on it (LiveSchedule.Trim cuts media that runs past it; players
	// step over a short gap).
	Duration time.Duration
	// Renditions maps a rendition key ("video", "audio-cs"...) to the item's
	// segments in it, from its first frame, each with its EXT-X-MAP.
	Renditions map[string][]Segment
}

// LiveOrigin is where the numbering of a live schedule stands at an item: the
// media sequence number of the item's first segment in every rendition and the
// item's discontinuity sequence number.
type LiveOrigin struct {
	MediaSequence         map[string]int
	DiscontinuitySequence int
}

// After returns the numbering of the item that follows item. Chained from a
// fixed origin over a schedule whose aired part never changes, it gives every
// request and every server restart the same numbers.
func (o LiveOrigin) After(item LiveItem) LiveOrigin {
	next := LiveOrigin{MediaSequence: map[string]int{}, DiscontinuitySequence: o.DiscontinuitySequence + 1}
	for key, segments := range item.Renditions {
		next.MediaSequence[key] = o.MediaSequence[key] + len(fit(segments, item.Duration.Seconds()))
	}
	return next
}

// LiveSchedule is a live presentation computed from a schedule and the clock.
type LiveSchedule struct {
	// Start is when Items[0] goes on air and Origin its numbering.
	Start  time.Time
	Origin LiveOrigin
	Items  []LiveItem
	// TargetDuration is the channel's EXT-X-TARGETDURATION. RFC 8216 forbids
	// changing it, so it should be a constant that covers the longest segment
	// the channel airs; a longer segment still raises it (EXTINF may never
	// exceed it), see MaxSegment.
	TargetDuration int
	// Window is how much of the past a playlist lists; it is at least four
	// target durations, so a playlist always holds the three RFC 8216 asks for.
	Window time.Duration
	// Trim names a segment cut to its first keep seconds. It replaces a
	// segment that runs past the end of its item (a program's audio, cut
	// where its video is): hls.js overwrites the overrun with the next item,
	// but AVPlayer plays it, running late. Nil lists the whole segment.
	Trim func(uri string, keep float64) string
}

// ErrOffAir means the schedule has nothing on air at the requested time.
var ErrOffAir = errors.New("hls: nothing on air")

// minTail is the shortest piece of a segment a fitted rendition keeps at the
// end of an item: shorter tails become a gap players step over.
const minTail = 0.1

// trimSlack is how far a segment may run past its item before Trim cuts it:
// about two audio frames.
const trimSlack = 0.05

// fit cuts a rendition's segments to an item's time on air.
func fit(segments []Segment, onAir float64) []Segment {
	out := make([]Segment, 0, len(segments))
	var t float64
	for _, s := range segments {
		if len(out) > 0 && t > onAir-minTail {
			break
		}
		s.Discontinuity, s.ProgramDateTime = false, time.Time{}
		out = append(out, s)
		t += s.Duration
	}
	if n := len(out); n > 0 {
		out[n-1].Duration += onAir - t
	}
	return out
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// MaxSegment is the longest segment a rendition of the item airs, in seconds.
func (item LiveItem) MaxSegment() float64 {
	var longest float64
	for _, segments := range item.Renditions {
		for _, s := range fit(segments, item.Duration.Seconds()) {
			longest = max(longest, s.Duration)
		}
	}
	return longest
}

// Cut is the segment boundary nearest t (seconds from the first segment's
// start), never before the first segment ends: where a program can leave the
// air without a partial video segment.
func Cut(segments []Segment, t float64) float64 {
	var end, best float64
	for i, s := range segments {
		end += s.Duration
		if i == 0 || math.Abs(end-t) < math.Abs(best-t) {
			best = end
		}
	}
	return best
}

// Playlist is the media playlist of rendition key at now: every segment that
// started airing in the last Window. A segment is listed as soon as it goes on
// air, so the playlist ends up to a segment ahead of the clock.
func (s *LiveSchedule) Playlist(key string, now time.Time) (*Media, error) {
	// every rendition gets the same target duration, and so the same window
	target := s.TargetDuration
	fitted := make([][]Segment, len(s.Items))
	for i, item := range s.Items {
		segments, ok := item.Renditions[key]
		if !ok || len(segments) == 0 {
			return nil, fmt.Errorf("hls: item %d has no %s rendition", i, key)
		}
		fitted[i] = fit(segments, item.Duration.Seconds())
		target = max(target, TargetDuration(item.MaxSegment()))
	}
	window := max(s.Window, time.Duration(4*target)*time.Second)
	from := now.Add(-window)

	m := &Media{Version: 7, TargetDuration: target}
	seq, start := s.Origin.MediaSequence[key], s.Start
	for i, segments := range fitted {
		end := start.Add(s.Items[i].Duration)
		if start.After(now) {
			break
		}
		var offset float64
		for j, seg := range segments {
			at := start.Add(seconds(offset))
			offset += seg.Duration
			if at.After(now) {
				break
			}
			if !at.Add(seconds(seg.Duration)).After(from) {
				continue
			}
			switch {
			case len(m.Segments) == 0:
				m.MediaSequence, m.DiscontinuitySequence = seq+j, s.Origin.DiscontinuitySequence+i
				seg.ProgramDateTime = at
			case j == 0:
				seg.Discontinuity, seg.ProgramDateTime = true, at
			}
			if whole := s.Items[i].Renditions[key][j].Duration; s.Trim != nil && whole-seg.Duration > trimSlack {
				seg.URI = s.Trim(seg.URI, seg.Duration)
			}
			m.Segments = append(m.Segments, seg)
		}
		seq += len(segments)
		start = end
	}
	if len(m.Segments) == 0 {
		return nil, ErrOffAir
	}
	return m, nil
}

// LiveMultivariant is the multivariant playlist of a live channel: one video
// variant, an audio group ("audio") and a subtitle group ("subs") whose
// renditions every item fills. The audio rendition in language lang is the
// default (else the first); subtitles stay off until the viewer picks one.
// The variant's CODECS, RESOLUTION, FRAME-RATE and bandwidths must cover every
// item (CoverCodecs merges the items' codec strings).
func LiveMultivariant(video Variant, audio, subtitles []Rendition, lang string) *Multivariant {
	pl := &Multivariant{Version: 7, IndependentSegments: true}
	def := slices.IndexFunc(audio, func(r Rendition) bool { return r.Language == lang })
	for i, r := range audio {
		r.Type, r.GroupID, r.Autoselect = "AUDIO", "audio", true
		r.Default = i == max(def, 0)
		pl.Renditions = append(pl.Renditions, r)
	}
	for _, r := range subtitles {
		r.Type, r.GroupID, r.Autoselect, r.Default = "SUBTITLES", "subs", true, false
		pl.Renditions = append(pl.Renditions, r)
	}
	video.Audio, video.Subtitles, video.ClosedCaptions = "", "", "NONE"
	if len(audio) > 0 {
		video.Audio = "audio"
	}
	if len(subtitles) > 0 {
		video.Subtitles = "subs"
	}
	pl.Variants = []Variant{video}
	return pl
}

// avc is an H.264 codec string (avc1.PPCCLL): profile, constraint flags, level.
type avc struct {
	entry                       string
	profile, constraints, level int
}

func parseAVC(codec string) (avc, bool) {
	entry, hex, ok := strings.Cut(codec, ".")
	if !ok || (entry != "avc1" && entry != "avc3") || len(hex) != 6 {
		return avc{}, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return avc{}, false
	}
	return avc{entry: entry, profile: int(v >> 16), constraints: int(v >> 8 & 0xff), level: int(v & 0xff)}, true
}

func (a avc) String() string {
	return fmt.Sprintf("%s.%02x%02x%02x", a.entry, a.profile, a.constraints, a.level)
}

// avcRank orders the H.264 profiles by what a decoder of one also decodes.
func avcRank(profile int) int {
	return slices.Index([]int{66, 77, 88, 100, 110, 122, 244}, profile)
}

// covers reports whether a decoder set up for a also decodes b.
func (a avc) covers(b avc) bool {
	if a.entry != b.entry || avcRank(a.profile) < avcRank(b.profile) || a.level < b.level {
		return false
	}
	// a constraint flag the declared stream sets must hold for the media too
	return a.profile != b.profile || a.constraints&^b.constraints == 0
}

// CoverCodecs merges the codec strings of several items into the CODECS of one
// variant: the H.264 ones become a single string covering the highest profile
// and level, the rest are listed once each.
func CoverCodecs(codecs []string) []string {
	var out []string
	merged := -1
	var top avc
	for _, c := range codecs {
		a, ok := parseAVC(c)
		if !ok {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
			continue
		}
		if merged < 0 {
			merged, top = len(out), a
			out = append(out, "")
			continue
		}
		switch {
		case avcRank(a.profile) > avcRank(top.profile):
			top.profile, top.constraints = a.profile, a.constraints
		case a.profile == top.profile:
			top.constraints &= a.constraints
		}
		top.level = max(top.level, a.level)
	}
	if merged >= 0 {
		out[merged] = top.String()
	}
	return out
}

// codecCovers reports whether the declared codec string covers the one the
// media carries: the same string, or an H.264 one of at least its profile and
// level (a live variant declares one string for items of several).
func codecCovers(declared, actual string) bool {
	if declared == actual {
		return true
	}
	d, ok1 := parseAVC(declared)
	a, ok2 := parseAVC(actual)
	return ok1 && ok2 && d.covers(a)
}
