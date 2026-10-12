package couchtv

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/media/mp4"
)

// loop plays a rendition directory over and over to fill a stretch of any
// length as one item of whole segments: copy n is segment n mod S of pass n / S,
// retimed by whole passes (mp4.Retime). The standby card loops this way, and so
// does the slate, a single segment.
type loop struct {
	// durations are the segments' lengths (EXTINF).
	durations []float64
	// info is what the multivariant playlist declares for it.
	info playback.RenditionInfo

	dir   string
	names []string
	init  []byte
	in    *mp4.Init
	// base is each segment's decode time and period one pass's length, in the
	// track's timescale
	base   []uint64
	period uint64
}

// openLoop reads a finished rendition directory (index.m3u8, init.mp4, the
// segments) to loop it.
func openLoop(dir string, info playback.RenditionInfo) (*loop, error) {
	text, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		return nil, err
	}
	pl, err := hls.Parse(string(text))
	if err != nil || pl.Media == nil || len(pl.Media.Segments) == 0 {
		return nil, fmt.Errorf("couchtv: %s: not a media playlist", dir)
	}
	l := &loop{info: info, dir: dir}
	if l.init, err = os.ReadFile(filepath.Join(dir, "init.mp4")); err != nil {
		return nil, err
	}
	if l.in, err = mp4.ParseInit(l.init); err != nil {
		return nil, err
	}
	// lengths come from the samples, not EXTINF: copies are placed by them
	scale := float64(l.in.Tracks[0].Timescale)
	var end uint64
	for _, s := range pl.Media.Segments {
		data, err := os.ReadFile(filepath.Join(dir, s.URI))
		if err != nil {
			return nil, err
		}
		frags, err := mp4.ParseSegment(data, l.in)
		if err != nil {
			return nil, err
		}
		last := frags[len(frags)-1]
		l.names = append(l.names, s.URI)
		l.base = append(l.base, frags[0].BaseDecodeTime)
		l.durations = append(l.durations, float64(last.BaseDecodeTime+last.Duration-frags[0].BaseDecodeTime)/scale)
		end = last.BaseDecodeTime + last.Duration
	}
	l.period = end - l.base[0]
	return l, nil
}

// segments lists the copies that fill d, under prefix, numbered from 0.
func (l *loop) segments(prefix string, d time.Duration) []hls.Segment {
	init := &hls.Map{URI: prefix + "init.mp4"}
	var out []hls.Segment
	var t float64
	for n := 0; n == 0 || t < d.Seconds()-1e-9; n++ {
		length := l.durations[n%len(l.durations)]
		out = append(out, hls.Segment{Duration: length, URI: fmt.Sprintf("%s%d.m4s", prefix, n), Map: init})
		t += length
	}
	return out
}

// copy is copy n, cut to keep seconds when keep > 0.
func (l *loop) copy(n int, keep float64) ([]byte, error) {
	j, pass := n%len(l.names), n/len(l.names)
	data, err := os.ReadFile(filepath.Join(l.dir, l.names[j]))
	if err != nil {
		return nil, err
	}
	if data, err = mp4.Retime(data, l.base[j]+uint64(pass)*l.period, uint32(n+1)); err != nil || keep <= 0 {
		return data, err
	}
	return mp4.Trim(data, l.in, keep)
}

// seconds is one pass's length.
func (l *loop) seconds() float64 {
	if len(l.base) == 0 {
		var total float64
		for _, d := range l.durations {
			total += d
		}
		return total
	}
	return float64(l.period) / float64(l.in.Tracks[0].Timescale)
}

// bitrate measures a rendition's segments for the multivariant playlist.
func (l *loop) bitrate() int64 {
	var bits float64
	for _, name := range l.names {
		if st, err := os.Stat(filepath.Join(l.dir, name)); err == nil {
			bits += float64(st.Size() * 8)
		}
	}
	return int64(math.Ceil(bits / l.seconds()))
}

// fill is a filler's video and audio: the slate, or a standby card.
type fill struct {
	Video, Audio *loop
}
