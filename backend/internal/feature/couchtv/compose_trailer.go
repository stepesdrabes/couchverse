package couchtv

import (
	"math"
	"time"
)

// The long promo (plan 6.5, D43): a trailer cut from the title's own scenes, with their sound,
// then the promo's 4 s end card. It is for the strongest candidates when a break has room: the
// best-ranked candidate whose trailer is rendered and fits, one a break, and the same title's
// not again within trailerRest.

// What a trailer asks of a break, and how often.
const (
	// trailerRest keeps a title's trailer from airing again for this long (either way: a
	// break composed again later must not put a second one beside it).
	trailerRest = 4 * time.Hour
	// A series' strip or marathon (fill weight under 2) gets a trailer only in a long break:
	// one of at least this many frames (8 minutes).
	longBreakFrames = 12000
	// trailersPerNight bounds how many trailers are rendered in a day. Each is a minute or
	// more of video (about 1900 frames, a quarter of an hour on a Pi, besides cutting the
	// scenes' frames), and the quiet hours are shared with every break's clips.
	trailersPerNight = 3
	trailerWindow    = 18 * time.Hour
)

// SpecScene is one scene of a trailer (spec.ts TrailerScene).
type SpecScene struct {
	Frames []string `json:"frames"`
	Focus  *Crop    `json:"focus,omitempty"`
	// Sound is the scene's own sound for exactly its frames, from the original-language track.
	Sound Bed `json:"sound"`
	// Lines are when its lines are spoken, seconds into the scene.
	Lines [][2]float64 `json:"lines"`
}

// trailerBody is a candidate's trailer body: timeless, so it renders once per title.
func trailerBody(c Candidate) Spec {
	title := c.Title
	title.Backdrop = c.BodyBackdrop
	return Spec{Template: "trailer", Part: "body", Mood: c.Mood, Premiere: c.Fill == FillPremiere, Title: &title,
		Scenes: c.Trailer, Bed: c.Bed}
}

// trailerEligible reports whether a candidate may have a trailer in a break of the given
// length: the strongest (premiere, movie, saga) always; a strip or a marathon in a long break.
func trailerEligible(c Candidate, breakFrames int) bool {
	return len(c.Trailer) >= trailerScenesMin && (fillWeight(c.Fill) >= 2 || breakFrames >= longBreakFrames)
}

// trailerPromo is the trailer of the best-ranked candidate that has one rendered and whose
// trailer and end card fit in room frames; none when no candidate does.
func trailerPromo(in BreakInput, ranked []Candidate, room int) (pieces []piece, frames int, titleID string) {
	for _, c := range ranked {
		n := c.TrailerFrames + endCardFrames
		if c.TrailerFrames == 0 || n > room || in.RecentTrailers[c.TitleID] || !trailerEligible(c, in.Frames) {
			continue
		}
		title := c.Title
		body := trailerBody(c)
		airing := c.Airing
		full := body
		full.Part, full.Airing = "full", &airing
		end := full
		end.Part, end.Title = "end", &title
		return []piece{
			fixed(Part{Kind: ClipTrailerBody, Spec: body, Frames: c.TrailerFrames, Sound: &full, TitleID: c.TitleID}),
			fixed(Part{Kind: ClipTrailerEnd, Spec: end, Frames: endCardFrames, TitleID: c.TitleID}),
		}, n, c.TitleID
	}
	return nil, 0, ""
}

// specScenes are a title's trailer scenes as a spec carries them. A scene whose file is gone is
// not here (the store leaves it out), and fewer than trailerScenesMin make no trailer.
func specScenes(scenes []CouchTvTrailerScene) []SpecScene {
	if len(scenes) < trailerScenesMin {
		return nil
	}
	out := make([]SpecScene, len(scenes))
	for i, s := range scenes {
		start := time.Duration(s.StartMs) * time.Millisecond
		length := time.Duration(s.DurationMs) * time.Millisecond
		sound := Bed{URL: AudioURL(s.AudioFileID, start, length), Out: length.Seconds()}
		if s.AudioStream != nil {
			sound.URL = StreamURL(s.AudioFileID, *s.AudioStream, start, length)
		}
		out[i] = SpecScene{Frames: FramesURLs(s.MediaFileID, start, s.DurationMs*FPS/1000), Sound: sound, Lines: specLines(s.Lines)}
	}
	return out
}

// lineJoin is the gap under which two lines of a scene are one stretch of speech: the music
// dips once under a quick exchange.
const lineJoin = 0.35

// specLines are a scene's lines as [from, to] seconds, in order, the close ones joined.
func specLines(lines []CouchTvTrailerLine) [][2]float64 {
	out := [][2]float64{}
	for _, l := range lines {
		from, to := float64(l.FromMs)/1000, float64(l.ToMs)/1000
		if n := len(out); n > 0 && from-out[n-1][1] < lineJoin {
			out[n-1][1] = math.Max(out[n-1][1], to)
			continue
		}
		out = append(out, [2]float64{from, to})
	}
	return out
}
