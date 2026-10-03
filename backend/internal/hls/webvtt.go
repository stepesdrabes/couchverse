package hls

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Cue is one WebVTT cue; times are in seconds.
type Cue struct {
	ID         string
	Start, End float64
	// Settings is whatever follows the timing on the cue line (position, align...).
	Settings string
	Payload  string
}

// WebVTT is a parsed WebVTT file.
type WebVTT struct {
	// TimestampMap is the X-TIMESTAMP-MAP header: cue time Local maps to the
	// media time MPEGTS (90 kHz). Present reports whether the header was there.
	TimestampMap struct {
		Present bool
		MPEGTS  int64
		Local   float64
	}
	// Blocks are the STYLE and REGION blocks, kept verbatim.
	Blocks []string
	Cues   []Cue
}

var errNotWebVTT = errors.New("webvtt: missing WEBVTT signature")

// ParseWebVTT reads a WebVTT file leniently: unknown blocks are skipped and
// NOTE comments dropped, as players do.
func ParseWebVTT(text string) (*WebVTT, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	blocks := strings.Split(text, "\n\n")
	header := blocks[0]
	if !strings.HasPrefix(header, "WEBVTT") {
		return nil, errNotWebVTT
	}
	v := &WebVTT{}
	for _, line := range strings.Split(header, "\n")[1:] {
		if rest, ok := strings.CutPrefix(line, "X-TIMESTAMP-MAP="); ok {
			v.TimestampMap.Present = true
			for _, part := range strings.Split(rest, ",") {
				key, value, _ := strings.Cut(part, ":")
				switch key {
				case "MPEGTS":
					v.TimestampMap.MPEGTS, _ = strconv.ParseInt(value, 10, 64)
				case "LOCAL":
					v.TimestampMap.Local, _ = parseTimestamp(value)
				}
			}
		}
	}
	for _, block := range blocks[1:] {
		block = strings.Trim(block, "\n")
		if block == "" || strings.HasPrefix(block, "NOTE") {
			continue
		}
		if strings.HasPrefix(block, "STYLE") || strings.HasPrefix(block, "REGION") {
			v.Blocks = append(v.Blocks, block)
			continue
		}
		cue, err := parseCue(block)
		if err != nil {
			continue
		}
		v.Cues = append(v.Cues, cue)
	}
	return v, nil
}

func parseCue(block string) (Cue, error) {
	sc := bufio.NewScanner(strings.NewReader(block))
	var c Cue
	if !sc.Scan() {
		return c, errors.New("empty cue")
	}
	line := sc.Text()
	if !strings.Contains(line, "-->") {
		c.ID = line
		if !sc.Scan() {
			return c, errors.New("cue without timing")
		}
		line = sc.Text()
	}
	start, rest, ok := strings.Cut(line, "-->")
	if !ok {
		return c, errors.New("cue without timing")
	}
	rest = strings.TrimSpace(rest)
	end, settings, _ := strings.Cut(rest, " ")
	var err error
	if c.Start, err = parseTimestamp(strings.TrimSpace(start)); err != nil {
		return c, err
	}
	if c.End, err = parseTimestamp(end); err != nil {
		return c, err
	}
	c.Settings = strings.TrimSpace(settings)
	var payload []string
	for sc.Scan() {
		payload = append(payload, sc.Text())
	}
	c.Payload = strings.Join(payload, "\n")
	return c, nil
}

// parseTimestamp reads [hh:]mm:ss.ttt.
func parseTimestamp(s string) (float64, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("webvtt: bad timestamp %q", s)
	}
	var total float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || (i < len(parts)-1 && strings.Contains(p, ".")) {
			return 0, fmt.Errorf("webvtt: bad timestamp %q", s)
		}
		total = total*60 + v
	}
	return total, nil
}

// FormatTimestamp writes hh:mm:ss.ttt.
func FormatTimestamp(seconds float64) string {
	ms := int64(seconds*1000 + 0.5)
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// Segment renders the cues overlapping [start, end) as one HLS WebVTT segment.
// Cue times stay on the source timeline; the X-TIMESTAMP-MAP header maps that
// timeline (LOCAL 0) onto the media timestamps that start at mpegts. A cue that
// spans a boundary appears in every segment it touches, as the HLS spec requires.
func (v *WebVTT) Segment(start, end float64, mpegts int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:%d,LOCAL:00:00:00.000\n", mpegts)
	for _, block := range v.Blocks {
		b.WriteString("\n" + block + "\n")
	}
	for _, c := range v.Cues {
		if c.End <= start || c.Start >= end {
			continue
		}
		b.WriteString("\n")
		if c.ID != "" {
			b.WriteString(c.ID + "\n")
		}
		b.WriteString(FormatTimestamp(c.Start) + " --> " + FormatTimestamp(c.End))
		if c.Settings != "" {
			b.WriteString(" " + c.Settings)
		}
		b.WriteString("\n" + c.Payload + "\n")
	}
	return b.String()
}
