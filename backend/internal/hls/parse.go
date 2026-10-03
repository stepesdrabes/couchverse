package hls

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Playlist is the result of Parse: exactly one of the two is set.
type Playlist struct {
	Multivariant *Multivariant
	Media        *Media
	// Tags lists every tag name in order of appearance, for rules about
	// presence and placement.
	Tags []string
}

var errNotPlaylist = errors.New("hls: missing #EXTM3U")

// Parse reads a multivariant or media playlist.
func Parse(text string) (*Playlist, error) {
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	if !sc.Scan() || strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff")) != "#EXTM3U" {
		return nil, errNotPlaylist
	}
	pl := &Playlist{}
	mv := &Multivariant{}
	md := &Media{TargetDuration: -1}
	var (
		pendingVariant *Variant
		pending        Segment
		haveInf        bool
		currentMap     *Map
		nextOffset     int64
		lineNo         = 1
	)
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			switch {
			case pendingVariant != nil:
				pendingVariant.URI = line
				mv.Variants = append(mv.Variants, *pendingVariant)
				pendingVariant = nil
			case haveInf:
				pending.URI = line
				pending.Map = currentMap
				md.Segments = append(md.Segments, pending)
				pending, haveInf = Segment{}, false
			default:
				return nil, fmt.Errorf("line %d: URI %q without EXTINF or EXT-X-STREAM-INF", lineNo, line)
			}
			continue
		}
		if !strings.HasPrefix(line, "#EXT") {
			continue // a comment
		}
		name, value, _ := strings.Cut(line, ":")
		pl.Tags = append(pl.Tags, name)
		var err error
		switch name {
		case "#EXT-X-VERSION":
			v, perr := strconv.Atoi(value)
			mv.Version, md.Version, err = v, v, perr
		case "#EXT-X-INDEPENDENT-SEGMENTS":
			mv.IndependentSegments, md.IndependentSegments = true, true
		case "#EXT-X-MEDIA":
			var r Rendition
			r, err = parseRendition(value)
			mv.Renditions = append(mv.Renditions, r)
		case "#EXT-X-STREAM-INF":
			var v Variant
			v, err = parseVariant(value)
			pendingVariant = &v
		case "#EXT-X-I-FRAME-STREAM-INF":
			var v IFrameVariant
			v, err = parseIFrameVariant(value)
			mv.IFrameVariants = append(mv.IFrameVariants, v)
		case "#EXT-X-TARGETDURATION":
			md.TargetDuration, err = strconv.Atoi(value)
		case "#EXT-X-MEDIA-SEQUENCE":
			md.MediaSequence, err = strconv.Atoi(value)
		case "#EXT-X-PLAYLIST-TYPE":
			md.PlaylistType = value
		case "#EXT-X-I-FRAMES-ONLY":
			md.IFramesOnly = true
		case "#EXT-X-ENDLIST":
			md.EndList = true
		case "#EXT-X-DISCONTINUITY":
			pending.Discontinuity = true
		case "#EXT-X-MAP":
			attrs := parseAttrs(value)
			currentMap = &Map{URI: attrs["URI"]}
			if br, ok := attrs["BYTERANGE"]; ok {
				currentMap.ByteRange, _, err = parseByteRange(br, 0)
			}
		case "#EXTINF":
			durText, _, _ := strings.Cut(value, ",")
			pending.Duration, err = strconv.ParseFloat(durText, 64)
			haveInf = true
		case "#EXT-X-BYTERANGE":
			pending.ByteRange, nextOffset, err = parseByteRange(value, nextOffset)
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", lineNo, name, err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if pendingVariant != nil {
		return nil, errors.New("EXT-X-STREAM-INF without a URI")
	}
	if len(mv.Variants) > 0 || len(mv.IFrameVariants) > 0 || len(mv.Renditions) > 0 {
		pl.Multivariant = mv
	} else {
		pl.Media = md
	}
	return pl, nil
}

// parseAttrs splits an attribute list, unquoting quoted values.
func parseAttrs(s string) map[string]string {
	out := map[string]string{}
	for len(s) > 0 {
		name, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				value, rest = rest[1:], ""
			} else {
				value, rest = rest[1:end+1], rest[end+2:]
			}
		} else if i := strings.IndexByte(rest, ','); i >= 0 {
			value, rest = rest[:i], rest[i:]
		} else {
			value, rest = rest, ""
		}
		out[strings.TrimSpace(name)] = value
		s = strings.TrimPrefix(rest, ",")
	}
	return out
}

func parseRendition(s string) (Rendition, error) {
	a := parseAttrs(s)
	return Rendition{
		Type: a["TYPE"], GroupID: a["GROUP-ID"], Name: a["NAME"], Language: a["LANGUAGE"],
		Default: a["DEFAULT"] == "YES", Autoselect: a["AUTOSELECT"] == "YES", Forced: a["FORCED"] == "YES",
		Channels: a["CHANNELS"], URI: a["URI"],
	}, nil
}

func parseVariant(s string) (Variant, error) {
	a := parseAttrs(s)
	v := Variant{
		SupplementalCodecs: a["SUPPLEMENTAL-CODECS"], VideoRange: a["VIDEO-RANGE"],
		Audio: a["AUDIO"], Subtitles: a["SUBTITLES"], ClosedCaptions: a["CLOSED-CAPTIONS"],
	}
	var err error
	if v.Bandwidth, err = optionalInt(a, "BANDWIDTH"); err != nil {
		return v, err
	}
	if v.AverageBandwidth, err = optionalInt(a, "AVERAGE-BANDWIDTH"); err != nil {
		return v, err
	}
	if c := a["CODECS"]; c != "" {
		v.Codecs = strings.Split(c, ",")
	}
	if v.Width, v.Height, err = parseResolution(a["RESOLUTION"]); err != nil {
		return v, err
	}
	if fr, ok := a["FRAME-RATE"]; ok {
		if v.FrameRate, err = strconv.ParseFloat(fr, 64); err != nil {
			return v, err
		}
	}
	return v, nil
}

func parseIFrameVariant(s string) (IFrameVariant, error) {
	a := parseAttrs(s)
	v := IFrameVariant{VideoRange: a["VIDEO-RANGE"], URI: a["URI"]}
	var err error
	if v.Bandwidth, err = optionalInt(a, "BANDWIDTH"); err != nil {
		return v, err
	}
	if v.AverageBandwidth, err = optionalInt(a, "AVERAGE-BANDWIDTH"); err != nil {
		return v, err
	}
	if c := a["CODECS"]; c != "" {
		v.Codecs = strings.Split(c, ",")
	}
	v.Width, v.Height, err = parseResolution(a["RESOLUTION"])
	return v, err
}

func optionalInt(a map[string]string, name string) (int64, error) {
	s, ok := a[name]
	if !ok {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}

func parseResolution(s string) (int, int, error) {
	if s == "" {
		return 0, 0, nil
	}
	w, h, ok := strings.Cut(s, "x")
	wi, err1 := strconv.Atoi(w)
	hi, err2 := strconv.Atoi(h)
	if !ok || err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("RESOLUTION %q is not WxH", s)
	}
	return wi, hi, nil
}

// parseByteRange reads "n[@o]"; without an offset the range continues where
// the previous one ended.
func parseByteRange(s string, next int64) (ByteRange, int64, error) {
	n, o, hasOffset := strings.Cut(s, "@")
	length, err := strconv.ParseInt(n, 10, 64)
	if err != nil {
		return ByteRange{}, next, err
	}
	offset := next
	if hasOffset {
		if offset, err = strconv.ParseInt(o, 10, 64); err != nil {
			return ByteRange{}, next, err
		}
	}
	return ByteRange{Length: length, Offset: offset}, offset + length, nil
}
