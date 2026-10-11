package hls

import (
	"context"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"couchverse/internal/media/mp4"
)

// Severity of a validation issue. Errors are what AVPlayer (or Apple's
// mediastreamvalidator) rejects; warnings are authoring-spec recommendations.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is one finding.
type Issue struct {
	Severity Severity
	Rule     string
	URI      string
	Message  string
}

func (i Issue) String() string {
	return fmt.Sprintf("%s %s %s: %s", i.Severity, i.Rule, i.URI, i.Message)
}

// Report collects the issues of one presentation.
type Report struct {
	Issues []Issue
	// Playlists counts the playlists checked and Segments the media segments parsed.
	Playlists, Segments int
}

// Errors returns the error-level issues.
func (r *Report) Errors() []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			out = append(out, i)
		}
	}
	return out
}

func (r *Report) add(sev Severity, rule, uri, format string, args ...any) {
	r.Issues = append(r.Issues, Issue{Severity: sev, Rule: rule, URI: uri, Message: fmt.Sprintf(format, args...)})
}

// Fetcher loads a playlist or segment and reports its content type ("" when
// the source has none, like a file).
type Fetcher interface {
	Fetch(ctx context.Context, u *url.URL) (body []byte, contentType string, err error)
}

// HTTPFetcher fetches http(s) URLs and, for local runs, file URLs.
type HTTPFetcher struct{ Client *http.Client }

func (f HTTPFetcher) Fetch(ctx context.Context, u *url.URL) ([]byte, string, error) {
	if u.Scheme == "file" {
		data, err := os.ReadFile(u.Path)
		return data, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return body, res.Header.Get("Content-Type"), nil
}

// Options tune a validation run.
type Options struct {
	// MaxSegments parses only the first n segments of each media playlist (and
	// the last one); 0 parses all of them.
	MaxSegments int
}

// mediaInfo is what a media playlist turned out to contain.
type mediaInfo struct {
	kind     string // video, audio, subtitles, iframes
	duration float64
	// live playlists have no EXT-X-ENDLIST and may change item (and init
	// section) at every discontinuity
	live bool
	// track is the main (first) track; tracks holds the main track of every
	// init section and codecs every track's codec, for playlists that mux
	// audio with the video or change items
	track  *mp4.Track
	tracks []mp4.Track
	codecs []string
	inits  map[string]bool
	// start is the presentation time of the first segment, in seconds.
	start    float64
	hasStart bool
	// timeline places every segment; anchors map each discontinuity's media
	// time onto the program date, and starts are the program dates at which
	// the listed discontinuities begin.
	timeline []segmentTime
	anchors  map[int]anchor
	starts   map[int]time.Time
	// peak and average bit rates measured from the parsed segments (bits/s).
	peak, average float64
	frameRate     float64
	// frameRates are measured per init section (frames, seconds).
	frameRates map[string][2]float64
	format     string // fmp4, ts, webvtt
}

// segmentTime is where a segment sits: its start in the playlist (seconds),
// its discontinuity sequence number and its program date, when known.
type segmentTime struct {
	start float64
	dsn   int
	date  time.Time
}

// anchor pins the media time of a discontinuity's first parsed segment to
// its place in the playlist and its program date.
type anchor struct {
	media, pos float64
	date       time.Time
}

// lenient reports whether the playlist plays several items, so a variant's
// attributes must cover them rather than equal them.
func (m *mediaInfo) lenient() bool {
	return m.live || len(m.inits) > 1
}

type validator struct {
	ctx    context.Context
	fetch  Fetcher
	opts   Options
	report *Report
	media  map[string]*mediaInfo
	inits  map[string]*mp4.Init
	// vttBase is the presentation start subtitle cues are mapped against, and
	// vttAnchors the same per discontinuity in a live presentation.
	vttBase    float64
	vttAnchors map[int]anchor
}

// Validate checks the presentation at rawURL: a multivariant playlist with
// everything it references, or a single media playlist.
func Validate(ctx context.Context, f Fetcher, rawURL string, opts Options) (*Report, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	v := &validator{ctx: ctx, fetch: f, opts: opts, report: &Report{}, media: map[string]*mediaInfo{}, inits: map[string]*mp4.Init{}}
	body, ct, err := f.Fetch(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", u, err)
	}
	v.checkContentType(u, ct, "playlist")
	pl, err := Parse(string(body))
	if err != nil {
		v.report.add(SeverityError, "playlist.syntax", u.String(), "%v", err)
		return v.report, nil
	}
	v.report.Playlists++
	if pl.Multivariant != nil {
		v.multivariant(u, pl)
	} else {
		v.mediaPlaylist(u, pl, "")
	}
	return v.report, nil
}

var bcp47 = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

func (v *validator) multivariant(u *url.URL, pl *Playlist) {
	m := pl.Multivariant
	r, uri := v.report, u.String()
	if m.Version == 0 {
		r.add(SeverityError, "version.missing", uri, "EXT-X-VERSION is required")
	}
	if !m.IndependentSegments {
		r.add(SeverityWarning, "multivariant.independent-segments", uri, "EXT-X-INDEPENDENT-SEGMENTS should be present")
	}
	if len(m.Variants) == 0 {
		r.add(SeverityError, "multivariant.no-variants", uri, "no EXT-X-STREAM-INF")
		return
	}

	groups := map[string][]Rendition{}
	for _, rd := range m.Renditions {
		key := rd.Type + "/" + rd.GroupID
		groups[key] = append(groups[key], rd)
		v.rendition(u, rd)
	}
	for key, members := range groups {
		names, defaults := map[string]bool{}, 0
		for _, rd := range members {
			if names[rd.Name] {
				r.add(SeverityError, "media.name-unique", uri, "group %s has two renditions named %q", key, rd.Name)
			}
			names[rd.Name] = true
			if rd.Default {
				defaults++
			}
		}
		if defaults > 1 {
			r.add(SeverityError, "media.default-unique", uri, "group %s has %d DEFAULT=YES renditions", key, defaults)
		}
	}

	// the first variant's timeline is the one subtitle cues are mapped onto
	var first *mediaInfo
	for i, vr := range m.Variants {
		info := v.mediaAt(u, vr.URI, "variant")
		if i == 0 && info != nil {
			first = info
			if info.hasStart {
				v.vttBase = info.start
			}
			v.vttAnchors = info.anchors
		}
	}
	for _, rd := range m.Renditions {
		if rd.URI != "" {
			v.mediaAt(u, rd.URI, strings.ToLower(rd.Type))
		}
	}

	var hasSDR, hasHDR bool
	durations := map[string]float64{}
	for _, vr := range m.Variants {
		info := v.media[resolve(u, vr.URI)]
		v.variant(u, vr, info, groups)
		if vr.VideoRange == "PQ" || vr.VideoRange == "HLG" {
			hasHDR = true
		} else {
			hasSDR = true
		}
		if info != nil {
			durations[vr.URI] = info.duration
		}
	}
	for _, rd := range m.Renditions {
		if info := v.media[resolve(u, rd.URI)]; rd.URI != "" && info != nil && info.kind != "subtitles" {
			durations[rd.URI] = info.duration
		}
	}
	if hasHDR && !hasSDR {
		r.add(SeverityWarning, "multivariant.sdr-fallback", uri, "HDR variants should come with SDR variants for SDR-only devices")
	}
	if first != nil && first.live {
		// live playlists are snapshots fetched one after another, so their
		// windows differ; what must agree is where every item starts
		v.liveAlignment(u, m, first)
	} else {
		var lo, hi float64 = math.Inf(1), 0
		for _, d := range durations {
			lo, hi = math.Min(lo, d), math.Max(hi, d)
		}
		if hi-lo > 2 {
			r.add(SeverityError, "multivariant.durations", uri, "variant and rendition durations differ by %.2fs", hi-lo)
		}
		if len(m.IFrameVariants) == 0 {
			r.add(SeverityWarning, "multivariant.iframes", uri, "no I-frame playlist: trick play and scrubbing thumbnails are unavailable")
		}
	}
	for _, iv := range m.IFrameVariants {
		v.iframeVariant(u, iv)
	}
}

func (v *validator) rendition(u *url.URL, rd Rendition) {
	r, uri := v.report, u.String()
	label := fmt.Sprintf("%s rendition %q", rd.Type, rd.Name)
	if rd.GroupID == "" || rd.Name == "" {
		r.add(SeverityError, "media.attributes", uri, "%s needs GROUP-ID and NAME", label)
	}
	switch rd.Type {
	case "AUDIO", "SUBTITLES":
	case "CLOSED-CAPTIONS":
		if rd.URI != "" {
			r.add(SeverityError, "media.uri", uri, "%s must not have a URI", label)
		}
	default:
		r.add(SeverityError, "media.type", uri, "%s has an unknown TYPE", label)
	}
	if rd.Type == "SUBTITLES" && rd.URI == "" {
		r.add(SeverityError, "media.uri", uri, "%s needs a URI", label)
	}
	if rd.Language == "" {
		r.add(SeverityWarning, "media.language", uri, "%s should have a LANGUAGE", label)
	} else if !bcp47.MatchString(rd.Language) {
		r.add(SeverityError, "media.language", uri, "%s LANGUAGE %q is not a BCP 47 tag", label, rd.Language)
	}
	if rd.Default && !rd.Autoselect {
		r.add(SeverityError, "media.autoselect", uri, "%s is DEFAULT=YES, so AUTOSELECT must be YES", label)
	}
	if rd.Forced && rd.Type != "SUBTITLES" {
		r.add(SeverityError, "media.forced", uri, "%s: FORCED is only for subtitles", label)
	}
	if rd.Forced && !rd.Autoselect {
		r.add(SeverityWarning, "media.forced", uri, "%s is FORCED, so it should be AUTOSELECT=YES", label)
	}
	if rd.Type == "AUDIO" && rd.Channels == "" {
		r.add(SeverityError, "media.channels", uri, "%s needs CHANNELS", label)
	}
}

func (v *validator) variant(u *url.URL, vr Variant, info *mediaInfo, groups map[string][]Rendition) {
	r, uri := v.report, u.String()
	label := "variant " + vr.URI
	if vr.Bandwidth <= 0 {
		r.add(SeverityError, "variant.bandwidth", uri, "%s needs BANDWIDTH", label)
	}
	if vr.AverageBandwidth <= 0 {
		r.add(SeverityError, "variant.average-bandwidth", uri, "%s needs AVERAGE-BANDWIDTH", label)
	}
	if len(vr.Codecs) == 0 {
		r.add(SeverityError, "variant.codecs", uri, "%s needs CODECS", label)
	}
	var audio []Rendition
	if vr.Audio != "" {
		if audio = groups["AUDIO/"+vr.Audio]; audio == nil {
			r.add(SeverityError, "variant.group", uri, "%s references the missing AUDIO group %q", label, vr.Audio)
		}
	}
	if vr.Subtitles != "" && groups["SUBTITLES/"+vr.Subtitles] == nil {
		r.add(SeverityError, "variant.group", uri, "%s references the missing SUBTITLES group %q", label, vr.Subtitles)
	}
	if vr.ClosedCaptions == "" {
		r.add(SeverityWarning, "variant.closed-captions", uri, "%s should say CLOSED-CAPTIONS=NONE when there are none", label)
	}
	if info == nil {
		return
	}

	// every codec the variant can carry must be declared, and nothing else;
	// a variant of several items declares one string covering theirs
	lenient := info.lenient()
	covers := func(declared, actual string) bool {
		return declared == actual || lenient && codecCovers(declared, actual)
	}
	present := []string{}
	present = append(present, info.codecs...)
	audioPeak, audioAverage := 0.0, 0.0
	for _, rd := range audio {
		ai := v.media[resolve(u, rd.URI)]
		if rd.URI == "" || ai == nil {
			continue
		}
		for _, t := range ai.tracks {
			present = append(present, t.Codec)
			v.channels(u, rd, &t, ai.lenient())
		}
		audioPeak, audioAverage = math.Max(audioPeak, ai.peak), math.Max(audioAverage, ai.average)
	}
	for _, c := range present {
		if c != "" && !slices.ContainsFunc(vr.Codecs, func(d string) bool { return covers(d, c) }) {
			r.add(SeverityError, "variant.codecs", uri, "%s plays %s but CODECS is %q", label, c, strings.Join(vr.Codecs, ","))
		}
	}
	for _, c := range vr.Codecs {
		if !slices.ContainsFunc(present, func(p string) bool { return covers(c, p) }) {
			r.add(SeverityWarning, "variant.codecs", uri, "%s declares %s that none of its media uses", label, c)
		}
	}

	if info.kind == "video" {
		for _, t := range info.tracks {
			v.variantVideo(u, vr, info, t)
		}
		if vr.FrameRate == 0 {
			r.add(SeverityError, "variant.frame-rate", uri, "%s needs FRAME-RATE", label)
		} else if !lenient && info.frameRate > 0 && math.Abs(vr.FrameRate-info.frameRate) > 0.5 {
			r.add(SeverityWarning, "variant.frame-rate", uri, "%s FRAME-RATE %.3f, measured %.3f", label, vr.FrameRate, info.frameRate)
		}
		for _, fr := range info.frameRates {
			if lenient && fr[1] > 0 && fr[0]/fr[1] > vr.FrameRate+0.5 {
				r.add(SeverityWarning, "variant.frame-rate", uri, "%s FRAME-RATE %.3f, an item measures %.3f", label, vr.FrameRate, fr[0]/fr[1])
			}
		}
	}

	// BANDWIDTH is a peak and must cover the measured one (Apple allows 10%)
	peak, average := info.peak+audioPeak, info.average+audioAverage
	if peak > 0 && float64(vr.Bandwidth)*1.1 < peak {
		r.add(SeverityError, "variant.bandwidth", uri, "%s BANDWIDTH %d is below the measured peak %.0f", label, vr.Bandwidth, peak)
	}
	if average > 0 && vr.AverageBandwidth > 0 && math.Abs(float64(vr.AverageBandwidth)-average) > 0.1*average {
		r.add(SeverityWarning, "variant.average-bandwidth", uri, "%s AVERAGE-BANDWIDTH %d differs from the measured %.0f by more than 10%%", label, vr.AverageBandwidth, average)
	}
}

// variantVideo checks a variant's video attributes against the video of one
// item (an init section); a variant of several items must fit the largest.
func (v *validator) variantVideo(u *url.URL, vr Variant, info *mediaInfo, t mp4.Track) {
	r, uri := v.report, u.String()
	label := "variant " + vr.URI
	if info.lenient() {
		if t.Width > vr.Width || t.Height > vr.Height {
			r.add(SeverityError, "variant.resolution", uri, "%s RESOLUTION %dx%d, an item is %dx%d", label, vr.Width, vr.Height, t.Width, t.Height)
		}
	} else if vr.Width != t.Width || vr.Height != t.Height {
		r.add(SeverityError, "variant.resolution", uri, "%s RESOLUTION %dx%d, media is %dx%d", label, vr.Width, vr.Height, t.Width, t.Height)
	}
	want := "SDR"
	switch t.Transfer {
	case 16:
		want = "PQ"
	case 18:
		want = "HLG"
	}
	got := vr.VideoRange
	if got == "" {
		got = "SDR"
	}
	if t.Transfer != 0 && got != want {
		r.add(SeverityError, "variant.video-range", uri, "%s VIDEO-RANGE=%s but the video is %s", label, got, want)
	}
	if t.DoviProfile > 0 && t.SupplementalCodec != "" && vr.SupplementalCodecs == "" {
		r.add(SeverityWarning, "variant.supplemental-codecs", uri, "%s carries Dolby Vision %s; declare it in SUPPLEMENTAL-CODECS", label, t.SupplementalCodec)
	}
}

// channels checks an audio rendition's CHANNELS against one of its streams;
// a rendition of several items declares the most any of them has.
func (v *validator) channels(u *url.URL, rd Rendition, t *mp4.Track, lenient bool) {
	count, _, _ := strings.Cut(rd.Channels, "/")
	want := fmt.Sprint(t.Channels)
	if t.JOC {
		want = "16"
	}
	if declared, err := strconv.Atoi(count); lenient && err == nil && !t.JOC && t.Channels <= declared {
		return
	}
	if count != want && t.Channels > 0 {
		v.report.add(SeverityError, "media.channels", u.String(), "audio rendition %q CHANNELS=%q but the stream has %s", rd.Name, rd.Channels, want)
	}
}

func (v *validator) iframeVariant(u *url.URL, iv IFrameVariant) {
	r, uri := v.report, u.String()
	label := "I-frame variant " + iv.URI
	if iv.Bandwidth <= 0 || iv.URI == "" || len(iv.Codecs) == 0 || iv.Width == 0 {
		r.add(SeverityError, "iframe.attributes", uri, "%s needs BANDWIDTH, CODECS, RESOLUTION and URI", label)
	}
	info := v.mediaAt(u, iv.URI, "iframes")
	if info == nil || info.track == nil {
		return
	}
	if !slices.Contains(iv.Codecs, info.track.Codec) {
		r.add(SeverityError, "iframe.codecs", uri, "%s plays %s but CODECS is %q", label, info.track.Codec, strings.Join(iv.Codecs, ","))
	}
	if iv.Width != info.track.Width || iv.Height != info.track.Height {
		r.add(SeverityError, "iframe.resolution", uri, "%s RESOLUTION %dx%d, media is %dx%d", label, iv.Width, iv.Height, info.track.Width, info.track.Height)
	}
}

func resolve(base *url.URL, ref string) string {
	u, err := base.Parse(ref)
	if err != nil {
		return ref
	}
	return u.String()
}

// mediaAt validates the media playlist at ref once and returns what it holds.
func (v *validator) mediaAt(base *url.URL, ref, kind string) *mediaInfo {
	abs := resolve(base, ref)
	if info, ok := v.media[abs]; ok {
		return info
	}
	v.media[abs] = nil
	u, _ := url.Parse(abs)
	body, ct, err := v.fetch.Fetch(v.ctx, u)
	if err != nil {
		v.report.add(SeverityError, "fetch", abs, "%v", err)
		return nil
	}
	v.checkContentType(u, ct, "playlist")
	pl, err := Parse(string(body))
	if err != nil {
		v.report.add(SeverityError, "playlist.syntax", abs, "%v", err)
		return nil
	}
	if pl.Media == nil {
		v.report.add(SeverityError, "playlist.kind", abs, "a %s must be a media playlist", kind)
		return nil
	}
	v.report.Playlists++
	info := v.mediaPlaylist(u, pl, kind)
	v.media[abs] = info
	return info
}

var segmentTypes = map[string][]string{
	"playlist": {"application/vnd.apple.mpegurl", "audio/mpegurl", "application/x-mpegurl"},
	"fmp4":     {"video/mp4", "audio/mp4", "video/iso.segment", "audio/iso.segment", "application/mp4"},
	"ts":       {"video/mp2t"},
	"webvtt":   {"text/vtt"},
}

func (v *validator) checkContentType(u *url.URL, ct, kind string) {
	if u.Scheme == "file" {
		return
	}
	mt, _, _ := mime.ParseMediaType(ct)
	if !slices.Contains(segmentTypes[kind], mt) {
		v.report.add(SeverityError, "http.content-type", u.String(), "served as %q, want one of %v", ct, segmentTypes[kind])
	}
}

func segmentFormat(uri string) string {
	path := uri
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	switch {
	case strings.HasSuffix(path, ".ts"):
		return "ts"
	case strings.HasSuffix(path, ".vtt") || strings.HasSuffix(path, ".webvtt"):
		return "webvtt"
	}
	return "fmp4"
}

func (v *validator) mediaPlaylist(u *url.URL, pl *Playlist, kind string) *mediaInfo {
	m := pl.Media
	r, uri := v.report, u.String()
	info := &mediaInfo{kind: kind, live: !m.EndList && m.PlaylistType != "VOD", inits: map[string]bool{},
		anchors: map[int]anchor{}, starts: map[int]time.Time{}, frameRates: map[string][2]float64{}}
	if m.TargetDuration < 0 {
		r.add(SeverityError, "media.target-duration", uri, "EXT-X-TARGETDURATION is required")
	}
	if len(m.Segments) == 0 {
		r.add(SeverityError, "media.segments", uri, "no segments")
		return info
	}
	if m.PlaylistType == "VOD" && !m.EndList {
		r.add(SeverityError, "media.endlist", uri, "a VOD playlist must end with EXT-X-ENDLIST")
	}
	if m.PlaylistType == "" && m.EndList {
		r.add(SeverityWarning, "media.playlist-type", uri, "on-demand playlists should say EXT-X-PLAYLIST-TYPE:VOD")
	}
	if kind == "iframes" && !m.IFramesOnly {
		r.add(SeverityError, "iframe.tag", uri, "an I-frame playlist needs EXT-X-I-FRAMES-ONLY")
	}

	info.format = segmentFormat(m.Segments[0].URI)
	needVersion := 3
	usesMap := m.Segments[0].Map != nil
	if usesMap && !m.IFramesOnly {
		needVersion = 6
	} else if m.IFramesOnly || m.Segments[0].ByteRange.Length > 0 {
		needVersion = 4
	}
	if m.Version < needVersion {
		r.add(SeverityError, "version.features", uri, "EXT-X-VERSION %d is below the %d its tags need", m.Version, needVersion)
	}
	if info.format == "fmp4" && !usesMap {
		r.add(SeverityError, "fmp4.map", uri, "fragmented MP4 segments need EXT-X-MAP")
	}
	if m.TargetDuration > 10 {
		r.add(SeverityWarning, "media.target-duration", uri, "target duration %ds; Apple recommends 6", m.TargetDuration)
	}

	v.timeline(u, m, info)
	if info.live {
		var total float64
		for _, seg := range m.Segments {
			total += seg.Duration
		}
		if total < 3*float64(m.TargetDuration) {
			r.add(SeverityError, "live.window", uri, "a live playlist must list three target durations (%ds), it lists %.3fs", 3*m.TargetDuration, total)
		}
	}

	var (
		elapsed    float64
		durations  []float64
		sizes      []float64
		frames     int
		frameTime  float64
		parsedLast = len(m.Segments) - 1
	)
	for i, s := range m.Segments {
		if int(s.Duration+0.5) > m.TargetDuration {
			r.add(SeverityError, "media.extinf", uri, "segment %d lasts %.3fs, over the %ds target duration", i, s.Duration, m.TargetDuration)
		}
		durations = append(durations, s.Duration)
		elapsed += s.Duration
		if v.opts.MaxSegments > 0 && i >= v.opts.MaxSegments && i != parsedLast {
			sizes = append(sizes, -1)
			continue
		}
		size, segFrames, segTime := v.segment(u, m, i, info)
		sizes = append(sizes, size)
		frames += segFrames
		frameTime += segTime
	}
	info.duration = elapsed
	if frames > 0 && frameTime > 0 && info.kind != "iframes" {
		info.frameRate = float64(frames) / frameTime
	}
	info.peak, info.average = Bitrates(durations, sizes, m.TargetDuration)
	return info
}

// timeline places every segment by its discontinuity sequence number and
// program date, and checks the dates: a jump without a discontinuity makes the
// mapping ambiguous; a live item without a date cannot be lined up.
func (v *validator) timeline(u *url.URL, m *Media, info *mediaInfo) {
	r, uri := v.report, u.String()
	info.timeline = make([]segmentTime, len(m.Segments))
	var (
		elapsed float64
		dsn     = m.DiscontinuitySequence
		next    time.Time
	)
	for i, s := range m.Segments {
		if s.Discontinuity {
			dsn++
		}
		date := next
		if !s.ProgramDateTime.IsZero() {
			if jump := s.ProgramDateTime.Sub(next).Seconds(); !next.IsZero() && math.Abs(jump) > 0.1 {
				if s.Discontinuity {
					r.add(SeverityWarning, "live.gap", uri, "segment %d: the program date jumps %.3fs at a discontinuity", i, jump)
				} else {
					r.add(SeverityError, "media.program-date-time", uri, "segment %d: the program date jumps %.3fs without a discontinuity", i, jump)
				}
			}
			date = s.ProgramDateTime
		} else if info.live && (s.Discontinuity || i == 0) {
			r.add(SeverityWarning, "live.program-date-time", uri, "segment %d starts an item without EXT-X-PROGRAM-DATE-TIME", i)
		}
		if s.Discontinuity && !date.IsZero() {
			info.starts[dsn] = date
		}
		info.timeline[i] = segmentTime{start: elapsed, dsn: dsn, date: date}
		elapsed += s.Duration
		if !date.IsZero() {
			next = date.Add(time.Duration(s.Duration * float64(time.Second)))
		}
	}
}

// liveAlignment checks that every rendition of a live presentation starts each
// item at the same program date as the first variant, under the same
// discontinuity sequence number.
func (v *validator) liveAlignment(u *url.URL, m *Multivariant, first *mediaInfo) {
	r := v.report
	var refs []string
	for _, vr := range m.Variants[1:] {
		refs = append(refs, vr.URI)
	}
	for _, rd := range m.Renditions {
		if rd.URI != "" {
			refs = append(refs, rd.URI)
		}
	}
	for _, ref := range refs {
		abs := resolve(u, ref)
		info := v.media[abs]
		if info == nil {
			continue
		}
		if !info.live {
			r.add(SeverityError, "live.alignment", abs, "a live presentation lists an on-demand playlist")
			continue
		}
		for dsn, date := range info.starts {
			for want, wantDate := range first.starts {
				sameDate := math.Abs(date.Sub(wantDate).Seconds()) <= 0.1
				if want == dsn && !sameDate || want != dsn && sameDate {
					r.add(SeverityError, "live.alignment", abs, "discontinuity %d starts at %s; the video starts discontinuity %d there at %s",
						dsn, FormatDateTime(date), want, FormatDateTime(wantDate))
				}
			}
		}
	}
}

// Bitrates measures the peak (Apple: the highest bit rate of any run of
// segments lasting between 0.5 and 1.5 target durations) and the average.
func Bitrates(durations, sizes []float64, target int) (peak, average float64) {
	var bits, secs float64
	for i := range durations {
		if sizes[i] < 0 {
			continue
		}
		bits += sizes[i] * 8
		secs += durations[i]
		var runBits, runSecs float64
		for j := i; j < len(durations) && sizes[j] >= 0; j++ {
			runBits += sizes[j] * 8
			runSecs += durations[j]
			if runSecs > 1.5*float64(target) {
				break
			}
			if runSecs >= 0.5*float64(target) || j == len(durations)-1 && i == 0 {
				peak = math.Max(peak, runBits/runSecs)
			}
		}
	}
	if secs > 0 {
		average = bits / secs
	}
	return peak, average
}

// segment fetches and checks one segment; it returns its size in bytes and,
// for video, how many frames it holds over how many seconds.
func (v *validator) segment(u *url.URL, m *Media, i int, info *mediaInfo) (float64, int, float64) {
	s := m.Segments[i]
	r := v.report
	segURL, err := u.Parse(s.URI)
	if err != nil {
		r.add(SeverityError, "segment.uri", u.String(), "segment %d: %v", i, err)
		return -1, 0, 0
	}
	body, ct, err := v.fetch.Fetch(v.ctx, segURL)
	if err != nil {
		r.add(SeverityError, "fetch", segURL.String(), "%v", err)
		return -1, 0, 0
	}
	v.report.Segments++
	format := segmentFormat(s.URI)
	v.checkContentType(segURL, ct, format)
	if s.ByteRange.Length > 0 {
		end := s.ByteRange.Offset + s.ByteRange.Length
		if end > int64(len(body)) {
			r.add(SeverityError, "segment.byterange", segURL.String(), "byte range %s past the end (%d bytes)", s.ByteRange, len(body))
			return -1, 0, 0
		}
		body = body[s.ByteRange.Offset:end]
	}
	if len(body) == 0 {
		r.add(SeverityError, "segment.empty", segURL.String(), "empty segment")
		return 0, 0, 0
	}

	switch format {
	case "ts":
		if body[0] != 0x47 || len(body) > 188 && body[188] != 0x47 {
			r.add(SeverityError, "ts.sync", segURL.String(), "not an MPEG-TS segment")
		}
		return float64(len(body)), 0, 0
	case "webvtt":
		v.webvtt(segURL, string(body), m, i, info)
		return float64(len(body)), 0, 0
	}

	if s.Map == nil {
		return float64(len(body)), 0, 0
	}
	in := v.initAt(u, s.Map)
	if in == nil {
		return float64(len(body)), 0, 0
	}
	if mp4.Child(body, "moov") != nil {
		r.add(SeverityError, "fmp4.segment", segURL.String(), "media segment carries a moov; it belongs in the EXT-X-MAP")
	}
	frags, err := mp4.ParseSegment(body, in)
	if err != nil {
		r.add(SeverityError, "fmp4.segment", segURL.String(), "%v", err)
		return float64(len(body)), 0, 0
	}
	main := in.Tracks[0]
	if info.track == nil {
		info.track = &main
		switch {
		case info.kind == "iframes":
		case main.Handler == "vide":
			info.kind = "video"
		case main.Handler == "soun":
			info.kind = "audio"
		}
	}
	if mapURI := resolve(u, s.Map.URI); !info.inits[mapURI] {
		info.inits[mapURI] = true
		info.tracks = append(info.tracks, main)
		for _, t := range in.Tracks {
			if !slices.Contains(info.codecs, t.Codec) {
				info.codecs = append(info.codecs, t.Codec)
			}
		}
	}
	f := frags[0]
	if in.Track(f.TrackID) == nil {
		r.add(SeverityError, "fmp4.track", segURL.String(), "fragment of track %d, which the init section lacks", f.TrackID)
		return float64(len(body)), 0, 0
	}
	if !f.FirstSync && (m.IndependentSegments || info.kind == "iframes" || main.Handler == "vide") {
		r.add(SeverityError, "fmp4.independent", segURL.String(), "segment %d does not start with a sync sample", i)
	}
	scale := float64(main.Timescale)
	start := (float64(f.BaseDecodeTime) + float64(f.FirstCompositionOffset)) / scale
	if i == 0 {
		info.start, info.hasStart = start, true
	}
	// every item has its own timeline: segments line up with the playlist
	// from the first one parsed after each discontinuity
	at := info.timeline[i]
	if a, ok := info.anchors[at.dsn]; !ok {
		info.anchors[at.dsn] = anchor{media: start, pos: at.start, date: at.date}
	} else if info.kind != "iframes" {
		if drift := start - a.media - (at.start - a.pos); math.Abs(drift) > 0.25 {
			r.add(SeverityError, "fmp4.timeline", segURL.String(), "segment %d starts %.3fs off the playlist timeline", i, drift)
		}
	}
	if info.kind != "iframes" {
		measured := float64(f.Duration) / scale
		if math.Abs(measured-s.Duration) > math.Max(0.5, 0.1*s.Duration) && i < len(m.Segments)-1 {
			r.add(SeverityWarning, "fmp4.duration", segURL.String(), "segment %d holds %.3fs of media but EXTINF says %.3fs", i, measured, s.Duration)
		}
	}
	if main.Handler == "vide" {
		mapURI := resolve(u, s.Map.URI)
		fr := info.frameRates[mapURI]
		info.frameRates[mapURI] = [2]float64{fr[0] + float64(f.Samples), fr[1] + float64(f.Duration)/scale}
		return float64(len(body)), f.Samples, float64(f.Duration) / scale
	}
	return float64(len(body)), 0, 0
}

func (v *validator) initAt(u *url.URL, m *Map) *mp4.Init {
	abs := resolve(u, m.URI)
	if in, ok := v.inits[abs]; ok {
		return in
	}
	v.inits[abs] = nil
	iu, _ := url.Parse(abs)
	body, ct, err := v.fetch.Fetch(v.ctx, iu)
	if err != nil {
		v.report.add(SeverityError, "fetch", abs, "%v", err)
		return nil
	}
	v.checkContentType(iu, ct, "fmp4")
	if m.ByteRange.Length > 0 && m.ByteRange.Offset+m.ByteRange.Length <= int64(len(body)) {
		body = body[m.ByteRange.Offset : m.ByteRange.Offset+m.ByteRange.Length]
	}
	in, err := mp4.ParseInit(body)
	if err != nil {
		v.report.add(SeverityError, "fmp4.init", abs, "%v", err)
		return nil
	}
	v.inits[abs] = in
	return in
}

func (v *validator) webvtt(u *url.URL, text string, m *Media, i int, info *mediaInfo) {
	r := v.report
	vtt, err := ParseWebVTT(text)
	if err != nil {
		r.add(SeverityError, "webvtt.header", u.String(), "%v", err)
		return
	}
	if !vtt.TimestampMap.Present {
		r.add(SeverityError, "webvtt.timestamp-map", u.String(), "HLS WebVTT segments need an X-TIMESTAMP-MAP header")
		return
	}
	var start float64
	for _, s := range m.Segments[:i] {
		start += s.Duration
	}
	end := start + m.Segments[i].Duration
	offset := float64(vtt.TimestampMap.MPEGTS)/90000 - vtt.TimestampMap.Local - v.vttBase
	mapped := true
	if info.live {
		// a live cue maps through the video of its own item, by program date
		at := info.timeline[i]
		a, ok := v.vttAnchors[at.dsn]
		mapped = ok && !a.date.IsZero() && !at.date.IsZero()
		start, end = 0, m.Segments[i].Duration
		offset = float64(vtt.TimestampMap.MPEGTS)/90000 - vtt.TimestampMap.Local - a.media + a.date.Sub(at.date).Seconds()
	}
	for _, c := range vtt.Cues {
		if c.End < c.Start {
			r.add(SeverityError, "webvtt.cue", u.String(), "cue %s ends before it starts", FormatTimestamp(c.Start))
		}
		if mapped && (c.End+offset <= start-0.5 || c.Start+offset >= end+0.5) {
			r.add(SeverityWarning, "webvtt.cue-window", u.String(), "cue at %s falls outside segment %d (%.3f-%.3fs)", FormatTimestamp(c.Start), i, start, end)
		}
	}
}
