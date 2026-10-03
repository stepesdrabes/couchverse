package playback

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"couchverse/internal/media"
)

// QualityOriginal keeps the source picture in a download, copied when the
// device decodes it; the other download qualities are ladder rungs.
const QualityOriginal = "original"

// ErrNoDownload is returned when no MP4 the device plays can be made.
var ErrNoDownload = errors.New("playback: the device plays no download of this file")

// DownloadPlan is how a device-ready MP4 is made from one media file.
type DownloadPlan struct {
	// Copy keeps the source video; otherwise it becomes H.264 SDR at Rendition.
	Copy      bool            `json:"copy"`
	Rendition media.Rendition `json:"rendition"`
	Audio     []DownloadAudio `json:"audio"`
}

// DownloadAudio is one source audio track in a download.
type DownloadAudio struct {
	Track media.AudioStream `json:"track"`
	// Copy keeps the track as is; otherwise it becomes AAC with Channels.
	Copy     bool `json:"copy"`
	Channels int  `json:"channels"`
}

// DownloadSubtitle is a WebVTT track muxed into a download as mov_text, which
// AVPlayer and ExoPlayer both show offline.
type DownloadSubtitle struct {
	Path   string
	Lang   string
	Label  string
	Forced bool
}

// mp4Audio are the audio codecs an MP4 carries that are worth keeping as is.
var mp4Audio = []string{"aac", "ac3", "eac3", "mp3", "alac"}

// PlanDownload decides how a file becomes an MP4 the device plays offline:
// the source video when the device decodes it (and, below original, when it is
// no bigger than the rung asked for), else the H.264 rung; the tracks of the
// requested languages (the default track when none matches), copied when the
// device takes them and AAC otherwise.
func PlanDownload(p DeviceProfile, mf *media.MediaFile, tracks []media.AudioStream, quality string, langs []string, f Features) (DownloadPlan, error) {
	s := sourceFacts(mf, tracks, 0)
	copyable := canCopySource(mf, f) && fitVideo(p, s, Server{DolbyVisionCopy: f.DolbyVision}).original
	var plan DownloadPlan
	if quality == QualityOriginal {
		plan.Copy = copyable
		ladder := make([]string, 0, len(media.Renditions))
		for name := range media.Renditions {
			ladder = append(ladder, name)
		}
		picked := media.PrepareRenditions(ladder, mf.Height)
		plan.Rendition = slices.MaxFunc(picked, func(a, b media.Rendition) int { return a.Height - b.Height })
	} else {
		r, ok := media.Renditions[quality]
		if !ok {
			return DownloadPlan{}, fmt.Errorf("playback: unknown download quality %q", quality)
		}
		plan.Rendition = r
		plan.Copy = copyable && mf.Height > 0 && mf.Height <= r.Height &&
			mf.Bitrate > 0 && mf.Bitrate <= r.VideoBitrate+r.AudioBitrate
	}
	if !plan.Copy {
		if !p.video("h264").supports("high", 4.1, 8) {
			return DownloadPlan{}, ErrNoDownload
		}
		// never upscale; scale needs an even height
		if mf.Height > 0 && mf.Height < plan.Rendition.Height {
			plan.Rendition.Height = mf.Height &^ 1
		}
		plan.Rendition = plan.Rendition.CappedAt(mf.Bitrate)
	}
	plan.Audio = downloadAudio(p, s, langs)
	return plan, nil
}

func downloadAudio(p DeviceProfile, s Source, langs []string) []DownloadAudio {
	var picked []media.AudioStream
	for _, lang := range langs {
		for _, t := range s.Audio {
			if media.SameLanguage(t.Lang, lang) && !slices.ContainsFunc(picked, func(o media.AudioStream) bool { return o.Index == t.Index }) {
				picked = append(picked, t)
				break
			}
		}
	}
	if len(picked) == 0 {
		if a := defaultAudio(s); a != nil {
			picked = append(picked, *a)
		}
	}
	aacChannels := 2
	if a := p.audio("aac"); a != nil {
		aacChannels = min(max(a.MaxChannels, 2), 6)
	}
	out := make([]DownloadAudio, 0, len(picked))
	for _, t := range picked {
		if slices.Contains(mp4Audio, t.Codec) && p.playsAudio(t.Codec, t.Channels) {
			out = append(out, DownloadAudio{Track: t, Copy: true, Channels: t.Channels})
			continue
		}
		out = append(out, DownloadAudio{Track: t, Channels: min(max(t.Channels, 1), aacChannels)})
	}
	return out
}

// Spec names what the plan makes, so requests that would make the same MP4
// share one.
func (d DownloadPlan) Spec() string {
	var b strings.Builder
	if d.Copy {
		b.WriteString("v=copy")
	} else {
		fmt.Fprintf(&b, "v=h264:%d:%d", d.Rendition.Height, d.Rendition.VideoBitrate)
	}
	b.WriteString(";a=")
	for i, a := range d.Audio {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(a.Track.Index))
		if a.Copy {
			b.WriteString(":copy")
		} else {
			fmt.Fprintf(&b, ":aac%d", a.Channels)
		}
	}
	return b.String()
}

// DownloadArgs are the ffmpeg arguments that write the plan to out: one
// faststart MP4 with the video, the planned audio tracks and the subtitles.
func DownloadArgs(mf *media.MediaFile, input string, plan DownloadPlan, subs []DownloadSubtitle, out, encoder, preset string, f Features) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", input}
	for _, s := range subs {
		args = append(args, "-i", s.Path)
	}
	if plan.Copy {
		args = append(args, sourceVideoArgs(mf, f)...)
	} else {
		args = append(args, "-map", "0:v:0", "-vf", videoFilter(mf, plan.Rendition.Height, f))
		args = append(args, encoderArgs(encoder, preset, plan.Rendition.VideoBitrate)...)
		args = append(args, sdrTags...)
	}
	for i, a := range plan.Audio {
		n := strconv.Itoa(i)
		args = append(args, "-map", fmt.Sprintf("0:%d", a.Track.Index))
		if a.Copy {
			args = append(args, "-c:a:"+n, "copy")
		} else {
			rate := "160k"
			if a.Channels > 2 {
				rate = "384k"
			}
			args = append(args, "-c:a:"+n, "aac", "-ac:a:"+n, strconv.Itoa(a.Channels), "-b:a:"+n, rate)
		}
		args = append(args, "-metadata:s:a:"+n, "language="+media.ISO639(a.Track.Lang))
		if a.Track.Title != "" {
			args = append(args, "-metadata:s:a:"+n, "title="+a.Track.Title)
		}
		args = append(args, "-disposition:a:"+n, disposition(i == 0, false))
	}
	for i, s := range subs {
		n := strconv.Itoa(i)
		args = append(args, "-map", strconv.Itoa(i+1)+":0", "-c:s:"+n, "mov_text",
			"-metadata:s:s:"+n, "language="+media.ISO639(s.Lang), "-disposition:s:"+n, disposition(false, s.Forced))
		if s.Label != "" {
			args = append(args, "-metadata:s:s:"+n, "title="+s.Label)
		}
	}
	// a long copied stream next to sparse subtitle packets overflows the default queue
	return append(args, "-max_muxing_queue_size", "4096", "-movflags", "+faststart", "-f", "mp4", out)
}

func disposition(isDefault, forced bool) string {
	switch {
	case forced:
		return "forced"
	case isDefault:
		return "default"
	}
	return "0"
}
