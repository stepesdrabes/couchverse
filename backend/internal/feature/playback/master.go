package playback

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"couchverse/internal/feature/library"
	"couchverse/internal/hls"
	"couchverse/internal/media"
)

// masterURL is the multivariant playlist a decision plays.
func masterURL(g string, m Master) string {
	if m.Video == "legacy" {
		return MediaPath(g, "hls/master.m3u8?video=legacy")
	}
	q := url.Values{"video": {m.Video}}
	if len(m.Surround) > 0 {
		q.Set("surround", strings.Join(m.Surround, ","))
	}
	return MediaPath(g, "hls/master.m3u8?"+q.Encode())
}

// rendition is a ready rendition directory and what rendition.json says about it.
type rendition struct {
	dir  string
	info renditionInfo
}

// audioTrack is one source audio track with its prepared renditions.
type audioTrack struct {
	stereo   rendition
	surround *rendition
}

// fileRenditions are the ready fMP4 renditions of a media file.
type fileRenditions struct {
	source    *rendition
	ladder    []rendition
	audio     []audioTrack
	trickplay *rendition
}

func (h *Stream) renditions(mediaFileID string, variants []library.TranscodeVariant) fileRenditions {
	dir := filepath.Join(h.dataDir, "cache", "hls", mediaFileID)
	read := func(name string) *rendition {
		info, err := readRendition(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		return &rendition{dir: name, info: info}
	}
	var out fileRenditions
	for _, v := range variants {
		if v.Status != "ready" || v.Format != "fmp4" {
			continue
		}
		switch v.Name {
		case media.VariantSource:
			out.source = read(media.VariantSource)
		case media.VariantTrickplay:
			out.trickplay = read(media.VariantTrickplay)
		case media.VariantAudio:
			out.audio = readAudioTracks(dir)
		default:
			if r := read(v.Name); r != nil {
				out.ladder = append(out.ladder, *r)
			}
		}
	}
	return out
}

func readAudioTracks(dir string) []audioTrack {
	entries, _ := os.ReadDir(dir)
	byStream := map[int]*audioTrack{}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "audio-") {
			continue
		}
		info, err := readRendition(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		t := byStream[info.StreamIndex]
		if t == nil {
			t = &audioTrack{}
			byStream[info.StreamIndex] = t
		}
		r := rendition{dir: e.Name(), info: info}
		if strings.HasPrefix(info.Codec, "mp4a") {
			t.stereo = r
		} else {
			t.surround = &r
		}
	}
	var out []audioTrack
	for _, t := range byStream {
		if t.stereo.dir != "" {
			out = append(out, *t)
		}
	}
	slices.SortFunc(out, func(a, b audioTrack) int { return cmp.Compare(a.stereo.info.StreamIndex, b.stereo.info.StreamIndex) })
	return out
}

// surroundCodecNames maps profile codec names to the codec strings of fMP4.
var surroundCodecNames = map[string]string{"eac3": "ec-3", "ac3": "ac-3"}

// buildMaster writes the multivariant playlist of a file's ready fMP4
// renditions: the copied source (video=original) or the transcoded ladder,
// each paired with every audio group, plus subtitle renditions and the
// I-frame playlist.
func buildMaster(r fileRenditions, subs []media.Subtitle, m Master) (*hls.Multivariant, error) {
	var videos []rendition
	if m.Video == "original" {
		if r.source == nil {
			return nil, fmt.Errorf("the original rendition is not ready")
		}
		videos = []rendition{*r.source}
	} else {
		videos = orderLadder(r.ladder)
	}
	if len(videos) == 0 {
		return nil, fmt.Errorf("no video rendition is ready")
	}

	pl := &hls.Multivariant{Version: 7, IndependentSegments: true}
	groups := audioGroups(r.audio, m.Surround)
	for _, g := range groups {
		pl.Renditions = append(pl.Renditions, g.renditions...)
	}
	subtitles := ""
	if len(subs) > 0 {
		subtitles = "subs"
		names := uniqueNames{}
		for _, s := range subs {
			label := s.Label
			if label == "" {
				label = audioLabel(s.Lang)
			}
			pl.Renditions = append(pl.Renditions, hls.Rendition{
				Type: "SUBTITLES", GroupID: subtitles, Name: names.take(label), Language: media.BCP47(s.Lang),
				Autoselect: true, Forced: s.Forced, URI: "subtitles/" + s.ID + "/index.m3u8",
			})
		}
	}

	for _, v := range videos {
		codecs := []string{v.info.Codec}
		if len(groups) == 0 {
			pl.Variants = append(pl.Variants, videoVariant(v, codecs, v.info.Peak, v.info.Average, "", subtitles))
			continue
		}
		for _, g := range groups {
			pl.Variants = append(pl.Variants, videoVariant(v, append(slices.Clone(codecs), g.codecs...),
				v.info.Peak+g.peak, v.info.Average+g.average, g.id, subtitles))
		}
	}
	if t := r.trickplay; t != nil {
		pl.IFrameVariants = append(pl.IFrameVariants, hls.IFrameVariant{
			Bandwidth: t.info.Peak, AverageBandwidth: t.info.Average, Codecs: []string{t.info.Codec},
			Width: t.info.Width, Height: t.info.Height, VideoRange: "SDR", URI: t.dir + "/iframes.m3u8",
		})
	}
	return pl, nil
}

func videoVariant(v rendition, codecs []string, peak, average int64, audio, subtitles string) hls.Variant {
	supplemental := ""
	if v.info.SupplementalCodec != "" {
		supplemental = v.info.SupplementalCodec
	}
	return hls.Variant{
		Bandwidth: peak, AverageBandwidth: average, Codecs: codecs, SupplementalCodecs: supplemental,
		Width: v.info.Width, Height: v.info.Height, FrameRate: math.Round(v.info.FrameRate*1000) / 1000,
		VideoRange: cmp.Or(v.info.VideoRange, "SDR"), Audio: audio, Subtitles: subtitles,
		ClosedCaptions: "NONE", URI: v.dir + "/index.m3u8",
	}
}

// orderLadder lists the rungs tallest first but starts with the one nearest
// 720p: AVPlayer begins with the first variant before it measures bandwidth.
func orderLadder(rungs []rendition) []rendition {
	out := slices.Clone(rungs)
	slices.SortFunc(out, func(a, b rendition) int { return cmp.Compare(b.info.Height, a.info.Height) })
	start := 0
	for i, r := range out {
		if abs(r.info.Height-720) < abs(out[start].info.Height-720) {
			start = i
		}
	}
	if start > 0 {
		first := out[start]
		out = append([]rendition{first}, slices.Delete(out, start, start+1)...)
	}
	return out
}

func abs(v int) int { return max(v, -v) }

// audioGroup is one EXT-X-MEDIA audio group with what its variants need.
type audioGroup struct {
	id            string
	renditions    []hls.Rendition
	codecs        []string
	peak, average int64
}

// audioGroups offers AAC stereo for every track, and a surround group with the
// multichannel renditions the client takes (tracks without one fall back to
// their stereo rendition there). Surround comes first so AVPlayer prefers it.
func audioGroups(tracks []audioTrack, surround []string) []audioGroup {
	if len(tracks) == 0 {
		return nil
	}
	accepted := map[string]bool{}
	for _, c := range surround {
		accepted[surroundCodecNames[c]] = true
	}
	hasSurround := false
	for _, t := range tracks {
		if t.surround != nil && accepted[t.surround.info.Codec] {
			hasSurround = true
		}
	}
	defaultTrack := 0
	for i, t := range tracks {
		if t.stereo.info.Default {
			defaultTrack = i
			break
		}
	}
	build := func(id string, pick func(audioTrack) rendition) audioGroup {
		g := audioGroup{id: id}
		names := uniqueNames{}
		for i, t := range tracks {
			r := pick(t)
			name := t.stereo.info.Name
			if name == "" || name == t.stereo.info.Language {
				name = audioLabel(t.stereo.info.Language)
			}
			g.renditions = append(g.renditions, hls.Rendition{
				Type: "AUDIO", GroupID: id, Name: names.take(name), Language: t.stereo.info.Language,
				Default: i == defaultTrack, Autoselect: true, Channels: r.info.Channels, URI: r.dir + "/index.m3u8",
			})
			if !slices.Contains(g.codecs, r.info.Codec) {
				g.codecs = append(g.codecs, r.info.Codec)
			}
			g.peak, g.average = max(g.peak, r.info.Peak), max(g.average, r.info.Average)
		}
		return g
	}
	groups := []audioGroup{}
	if hasSurround {
		groups = append(groups, build("surround", func(t audioTrack) rendition {
			if t.surround != nil && accepted[t.surround.info.Codec] {
				return *t.surround
			}
			return t.stereo
		}))
	}
	return append(groups, build("stereo", func(t audioTrack) rendition { return t.stereo }))
}

// uniqueNames keeps rendition names unique within a group, as HLS requires.
type uniqueNames map[string]int

func (u uniqueNames) take(name string) string {
	u[name]++
	if n := u[name]; n > 1 {
		return name + " " + strconv.Itoa(n)
	}
	return name
}

// legacyMaster lists the pre-v2 MPEG-TS variants (video and AAC stereo muxed).
func legacyMaster(mf *media.MediaFile, variants []library.TranscodeVariant) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, v := range variants {
		// the multiaudio remux is a presentation of its own with its own master
		if v.Status != "ready" || v.Format != "ts" || v.Name == "multiaudio" {
			continue
		}
		bandwidth := v.VideoBitrate + v.AudioBitrate
		if bandwidth <= 0 {
			bandwidth = mf.Bitrate
		}
		width, height := v.Width, v.Height
		if v.Mode == "copy" {
			width, height = mf.Width, mf.Height
		}
		if width == 0 && height > 0 && mf.Height > 0 {
			width = mf.Width * height / mf.Height
		}
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n%s/index.m3u8\n",
			bandwidth, width, height, v.Name, v.Name)
	}
	return b.String()
}

// subtitlePlaylist segments a sidecar subtitle track along the video's
// segment grid.
func subtitlePlaylist(duration float64) *hls.Media {
	m := &hls.Media{Version: 7, TargetDuration: int(segmentSeconds), PlaylistType: "VOD", EndList: true}
	for i := 0; float64(i)*segmentSeconds < duration-0.05; i++ {
		d := min(segmentSeconds, duration-float64(i)*segmentSeconds)
		m.Segments = append(m.Segments, hls.Segment{Duration: d, URI: strconv.Itoa(i) + ".vtt"})
	}
	return m
}

// vttCache keeps recently parsed sidecar files: a player asks for the same
// track once per segment.
type vttCache struct {
	mu      sync.Mutex
	entries map[string]vttEntry
}

type vttEntry struct {
	mtime time.Time
	vtt   *hls.WebVTT
	used  time.Time
}

func (c *vttCache) get(ctx context.Context, path string) (*hls.WebVTT, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[path]; ok && e.mtime.Equal(st.ModTime()) {
		e.used = time.Now()
		c.entries[path] = e
		return e.vtt, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vtt, err := hls.ParseWebVTT(string(data))
	if err != nil {
		return nil, err
	}
	if c.entries == nil {
		c.entries = map[string]vttEntry{}
	}
	if len(c.entries) >= 32 {
		oldest := ""
		for k, e := range c.entries {
			if oldest == "" || e.used.Before(c.entries[oldest].used) {
				oldest = k
			}
		}
		delete(c.entries, oldest)
	}
	c.entries[path] = vttEntry{mtime: st.ModTime(), vtt: vtt, used: time.Now()}
	return vtt, nil
}
