package couchtv

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
)

// The live stream (plan 6.6) is one live HLS presentation computed per request
// from the schedule by hls.LiveSchedule. Every program, break and stretch of
// slate is an item of its own (a discontinuity, a program date). An airing fixes
// what it airs (its feed) and its numbering (its origin) the moment it goes on
// air, chained from the airing before it, so every request and every restart
// lists the same playlists.

const (
	// TargetDuration is the channel's EXT-X-TARGETDURATION. RFC 8216 forbids
	// changing it, so a program whose package has longer segments airs from a
	// ladder rung (6 s segments), which on a Pi takes about as long to encode as
	// it plays. Copied sources keep their encoder's keyframes: x264's default of
	// one every 250 frames, or 15 s ones, cut segments of up to 16 s.
	TargetDuration = 16
	// liveWindow is how much of the past a playlist lists.
	liveWindow = 2 * time.Minute
	// airingReach is how long before and after its time on air an airing's
	// files can be fetched: players buffer ahead and lag behind.
	airingReach = 10 * time.Minute
	// maxGap is the longest hole in the schedule the slate fills; after a longer
	// one (nothing was planned) the stream starts its numbering afresh.
	maxGap = 6 * time.Hour
	// minPart is the shortest stretch of slate worth an item of its own; a
	// shorter one is absorbed by the part next to it.
	minPart = 500 * time.Millisecond
	// subtitleSeconds is the WebVTT segment grid, the packages' segment length.
	subtitleSeconds = 6.0
	// subtitleMPEGTS maps cue time 0 onto the packages' 1.4 s timeline offset.
	subtitleMPEGTS = 126000
)

// Languages are the channel's audio and subtitle languages (D20, D21); the first
// is the default audio when the display language is neither.
var Languages = []string{"cs", "en"}

var languageNames = map[string]string{"cs": "Čeština", "en": "English"}

// Rendition keys: the channel's media playlists are <key>.m3u8.
const videoKey = "video"

func audioKey(lang string) string { return "audio-" + lang }
func subsKey(lang string) string  { return "subs-" + lang }

func liveKeys() []string {
	keys := []string{videoKey}
	for _, l := range Languages {
		keys = append(keys, audioKey(l), subsKey(l))
	}
	return keys
}

// Kinds of parts. The standby card and the slate are fillers: what airs, looped,
// where nothing else can. A bulletin airs in a break's slot.
const (
	PartProgram  = "program"
	PartBreak    = "break"
	PartBulletin = "bulletin"
	PartStandby  = "standby"
	PartSlate    = "slate"
)

// LivePart is a stretch of what an airing airs: a program from its package, a
// break's assembled package (or a section of it), a bulletin's clip, the standby
// card or the slate.
type LivePart struct {
	Kind string `json:"kind"`
	// Ms is the part's time on air in milliseconds.
	Ms int64 `json:"ms"`
	// File is a program's media file or the standby card's or a bulletin's clip;
	// Path a break's or a clip's package, under DATA_DIR.
	File string `json:"file,omitempty"`
	Path string `json:"path,omitempty"`
	// Video names the video rendition's directory; Audio the audio rendition's
	// per channel language (absent: the slate's silence).
	Video string            `json:"video,omitempty"`
	Audio map[string]string `json:"audio,omitempty"`
	// Subtitles maps a channel language to a program's subtitle track.
	Subtitles map[string]string `json:"subtitles,omitempty"`
}

func (p LivePart) duration() time.Duration {
	return time.Duration(p.Ms) * time.Millisecond
}

func (p LivePart) filler() bool {
	return p.Kind == PartStandby || p.Kind == PartSlate
}

// Feed is what an airing airs, from Start, its parts back to back.
type Feed struct {
	Start time.Time  `json:"start"`
	Parts []LivePart `json:"parts"`
}

// End is when the feed's last part leaves the air.
func (f Feed) End() time.Time {
	end := f.Start
	for _, p := range f.Parts {
		end = end.Add(p.duration())
	}
	return end
}

// add appends a part, merging stretches of the same filler.
func (f *Feed) add(p LivePart) {
	if p.Ms <= 0 {
		return
	}
	if n := len(f.Parts); n > 0 && p.filler() && f.Parts[n-1].Kind == p.Kind && f.Parts[n-1].File == p.File {
		f.Parts[n-1].Ms += p.Ms
		return
	}
	f.Parts = append(f.Parts, p)
}

// mediaPart is the k-th part of a feed that is not a filler, from 0: its program or break,
// or one of the sections of a break with bulletin slots.
func (f Feed) mediaPart(k int) (LivePart, bool) {
	for _, p := range f.Parts {
		if p.filler() {
			continue
		}
		if k == 0 {
			return p, true
		}
		k--
	}
	return LivePart{}, false
}

func slatePart(d time.Duration) LivePart {
	return LivePart{Kind: PartSlate, Ms: d.Milliseconds()}
}

func milliseconds(seconds float64) int64 {
	return int64(math.Round(seconds * 1000))
}

// liveEpoch anchors fresh numbering.
var liveEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// freshOrigin numbers an airing with no aired airing before it to chain from
// (nobody tuned in for a while, or the first airing ever) from the clock: two
// media sequence numbers and one discontinuity per second since the epoch. A
// chain grows slower than that (segments last seconds, items much longer), so
// the numbers a player sees never go back.
func freshOrigin(start time.Time) hls.LiveOrigin {
	s := int(start.Sub(liveEpoch) / time.Second)
	o := hls.LiveOrigin{MediaSequence: map[string]int{}, DiscontinuitySequence: s}
	for _, k := range liveKeys() {
		o.MediaSequence[k] = 2 * s
	}
	return o
}

// chain is the origin of the airing after one that started at origin with these items.
func chain(origin hls.LiveOrigin, items []hls.LiveItem) hls.LiveOrigin {
	for _, item := range items {
		origin = origin.After(item)
	}
	return origin
}

// subtitleSegments cuts d along the subtitle grid.
func subtitleSegments(d time.Duration, uri func(k int) string) []hls.Segment {
	total := d.Seconds()
	var out []hls.Segment
	for k := 0; k == 0 || float64(k)*subtitleSeconds < total-0.001; k++ {
		out = append(out, hls.Segment{Duration: min(subtitleSeconds, total-float64(k)*subtitleSeconds), URI: uri(k)})
	}
	return out
}

const emptySubtitles = "../slate/subs/empty.vtt"

// rebase points a rendition's segments (URIs relative to its directory) at base.
func rebase(segments []hls.Segment, base string) []hls.Segment {
	out := slices.Clone(segments)
	var init *hls.Map
	for i := range out {
		out[i].URI = base + out[i].URI
		if m := out[i].Map; m != nil {
			if init == nil || init.URI != base+m.URI {
				init = &hls.Map{URI: base + m.URI, ByteRange: m.ByteRange}
			}
			out[i].Map = init
		}
	}
	return out
}

// breakMedia is a break's assembled rendition.
type breakMedia struct {
	Video, Audio *playback.PackageRendition
}

// duration is how long the break's video lasts.
func (b *breakMedia) duration() time.Duration {
	var total float64
	for _, s := range b.Video.Playlist.Segments {
		total += s.Duration
	}
	return time.Duration(total * float64(time.Second))
}

// liveMedia reads what the channel airs.
type liveMedia interface {
	Package(fileID string) (*playback.Package, error)
	Break(path string) (*breakMedia, error)
	// Clip is a rendered clip's package (a bulletin's).
	Clip(path string) (*playback.Package, error)
	// Fill is a filler to loop: the slate for path "", else the standby card's package.
	Fill(path string) (*fill, error)
}

// fillPrefix is where a filler part's copies are served.
func fillPrefix(p LivePart) string {
	if p.Kind == PartStandby {
		return "../standby/" + p.File + "/"
	}
	return "../slate/"
}

// airingBase is where the playlists reach the k-th media part of airing id's feed (mediaPart):
// the first under ../airings/<id>/, the others (the sections of a break with bulletin slots)
// under ../airings/<id>/parts/<k>/.
func airingBase(id string, k int) string {
	if k == 0 {
		return "../airings/" + id + "/"
	}
	return "../airings/" + id + "/parts/" + strconv.Itoa(k) + "/"
}

// partItem is the live item of one part of airing id's feed, its k-th media part
// unless it is a filler. A program's or a break's media is reached through
// airingBase, so a request names the airing, the part and the rendition, never a
// file of the library; a filler loops under ../slate/ or ../standby/<clip>/.
func partItem(id string, k int, p LivePart, m liveMedia) (hls.LiveItem, error) {
	d := p.duration()
	item := hls.LiveItem{Duration: d, Renditions: map[string][]hls.Segment{}}
	empty := subtitleSegments(d, func(int) string { return emptySubtitles })
	if p.filler() {
		f, err := m.Fill(p.Path)
		if err != nil {
			return item, err
		}
		prefix := fillPrefix(p)
		item.Renditions[videoKey] = f.Video.segments(prefix+"video/", d)
		audio := f.Audio.segments(prefix+"audio/", d)
		for _, l := range Languages {
			item.Renditions[audioKey(l)] = audio
			item.Renditions[subsKey(l)] = empty
		}
		return item, nil
	}
	slate, err := m.Fill("")
	if err != nil {
		return item, err
	}
	renditions, err := partRenditions(p, m)
	if err != nil {
		return item, err
	}
	base := airingBase(id, k)
	video := renditions[p.Video]
	if video == nil {
		return item, fmt.Errorf("couchtv: %s has no video rendition %q", id, p.Video)
	}
	item.Renditions[videoKey] = rebase(video.Playlist.Segments, base+videoKey+"/")
	for _, l := range Languages {
		if a := renditions[p.Audio[l]]; a != nil {
			item.Renditions[audioKey(l)] = rebase(a.Playlist.Segments, base+audioKey(l)+"/")
		} else {
			item.Renditions[audioKey(l)] = slate.Audio.segments("../slate/audio/", d)
		}
		item.Renditions[subsKey(l)] = empty
		if p.Subtitles[l] != "" {
			prefix := base + subsKey(l) + "/"
			item.Renditions[subsKey(l)] = subtitleSegments(d, func(k int) string { return prefix + strconv.Itoa(k) + ".vtt" })
		}
	}
	return item, nil
}

// partRenditions maps the renditions a program, break or bulletin part airs by their
// directory name.
func partRenditions(p LivePart, m liveMedia) (map[string]*playback.PackageRendition, error) {
	out := map[string]*playback.PackageRendition{}
	if p.Kind == PartBreak {
		b, err := m.Break(p.Path)
		if err != nil {
			return nil, err
		}
		out[b.Video.Name] = b.Video
		if b.Audio != nil {
			out[b.Audio.Name] = b.Audio
		}
		return out, nil
	}
	var pkg *playback.Package
	var err error
	if p.Kind == PartBulletin {
		pkg, err = m.Clip(p.Path)
	} else {
		pkg, err = m.Package(p.File)
	}
	if err != nil {
		return nil, err
	}
	if pkg.Source != nil {
		out[pkg.Source.Name] = pkg.Source
	}
	for i := range pkg.Ladder {
		out[pkg.Ladder[i].Name] = &pkg.Ladder[i]
	}
	for i := range pkg.Stereo {
		out[pkg.Stereo[i].Name] = &pkg.Stereo[i]
	}
	return out, nil
}

// airable picks the video a program airs from its package and where it leaves
// the air: the copied source when it is 8-bit SDR H.264 no larger than 1080p
// whose segments fit the target duration, else the tallest H.264 rung up to
// 1080p. It airs to the video boundary nearest its credits (when known), else
// to its end. Nil when nothing in the package can air.
func airable(p *playback.Package, credits time.Duration) (*playback.PackageRendition, float64) {
	cut := func(r *playback.PackageRendition) float64 {
		var total float64
		for _, s := range r.Playlist.Segments {
			total += s.Duration
		}
		at := total
		if c := credits.Seconds(); c > 0 && c < total {
			at = c
		}
		return hls.Cut(r.Playlist.Segments, at)
	}
	if s := p.Source; s != nil && channelVideo(s.Info) {
		at := cut(s)
		if longestUntil(s.Playlist.Segments, at) <= TargetDuration {
			return s, at
		}
	}
	for i := range p.Ladder {
		if r := &p.Ladder[i]; r.Info.Height <= 1080 && channelVideo(r.Info) {
			return r, cut(r)
		}
	}
	return nil, 0
}

// channelVideo reports whether a rendition fits the channel's one video
// variant: 8-bit 4:2:0 H.264 (Baseline, Main, Extended or High), SDR, up to 1080p.
func channelVideo(info playback.RenditionInfo) bool {
	profile, ok := strings.CutPrefix(info.Codec, "avc1.")
	return ok && len(profile) == 6 && slices.Contains([]string{"42", "4d", "58", "64"}, strings.ToLower(profile[:2])) &&
		(info.VideoRange == "" || info.VideoRange == "SDR") && info.Width <= 1920 && info.Height <= 1080
}

// longestUntil is the EXTINF target of the segments that start before at.
func longestUntil(segments []hls.Segment, at float64) int {
	var longest, t float64
	for _, s := range segments {
		if t >= at-0.001 {
			break
		}
		longest = max(longest, s.Duration)
		t += s.Duration
	}
	return hls.TargetDuration(longest)
}

// pickAudio maps every channel language to the program's AAC rendition in it
// (the default track among several), else to the original (the default track,
// else the first). Separate-language files (model B) are not on the played
// file's timeline, so only its own tracks can air.
func pickAudio(stereo []playback.PackageRendition) map[string]string {
	if len(stereo) == 0 {
		return nil
	}
	pick := func(match func(playback.RenditionInfo) bool) string {
		best := -1
		for i, r := range stereo {
			if match(r.Info) && (best < 0 || r.Info.Default && !stereo[best].Info.Default) {
				best = i
			}
		}
		if best < 0 {
			return ""
		}
		return stereo[best].Name
	}
	original := pick(func(playback.RenditionInfo) bool { return true })
	out := map[string]string{}
	for _, l := range Languages {
		out[l] = original
		if name := pick(func(info playback.RenditionInfo) bool { return info.Language == l }); name != "" {
			out[l] = name
		}
	}
	return out
}

// liveSubtitle is a subtitle track of a program's file.
type liveSubtitle struct {
	ID     string
	Lang   string // BCP 47
	Forced bool
}

// pickSubtitles maps every channel language the file has subtitles in to a
// track, full ones before forced ones.
func pickSubtitles(subs []liveSubtitle) map[string]string {
	out := map[string]string{}
	for _, forced := range []bool{false, true} {
		for _, s := range subs {
			if s.Forced == forced && slices.Contains(Languages, s.Lang) && out[s.Lang] == "" {
				out[s.Lang] = s.ID
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// lastBoundaryBefore is the latest segment boundary at or before t, never before
// the first segment ends.
func lastBoundaryBefore(segments []hls.Segment, t float64) float64 {
	var end, best float64
	for i, s := range segments {
		end += s.Duration
		if i == 0 || end <= t+0.001 {
			best = end
		}
		if end > t {
			break
		}
	}
	return best
}

// trimURI names a segment cut to its first keep seconds.
func trimURI(uri string, keep float64) string {
	return uri + "?keep=" + strconv.FormatFloat(keep, 'f', 3, 64)
}
