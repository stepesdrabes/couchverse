package couchtv

import (
	"cmp"
	"context"
	"log/slog"
	"math"
	"slices"

	"couchverse/internal/media"
)

// Footage for promos (D18): a few short shots from the first half of a title, spread
// over it, where the picture moves and never black. Keyframes are the candidates (cheap
// to decode, and encoders put one on most cuts); the best-looking keyframe of each
// stretch is then decoded for a few seconds to find where its shot ends (a cut, scored
// the way ffmpeg's scdet scores one) and how much it moves. Pictures are also checked
// (pictureRisk) for what the channel does not want on every screen: gore, and faces
// filling the frame, which come after wide and medium shots.
const (
	shotsMax       = 6
	shotMinLength  = 1.0
	shotMaxLength  = 3.0
	shotWindowFrom = 0.05 // of the runtime: past studio logos and most cold opens
	shotWindowTo   = 0.5  // the first half only, so promos spoil nothing
	shotW, shotH   = 160, 90
	shotFPS        = 10
	// riskEvery is how often a shot's picture is rated: every 5th frame, 0.5 s
	riskEvery = 5
	// avoidRisk is the picture risk from which a shot never airs, when two frames in a row
	// reach it; a face close-up never does (closeUpRisk), so a title keeps footage when its
	// scenes are all faces
	avoidRisk = 0.85
	// sceneThreshold is scdet's default, on its 0-100 scale
	sceneThreshold = 10.0
	// airFrame is a frame on air (25 fps): shots are whole frames
	airFrame = 0.04
)

// What a usable picture is: not near-black (its brightest 5% above blackP95), not
// washed out, with some contrast.
const (
	blackP95             = 60
	minMean, maxMean     = 30, 210
	minContrast          = 18
	motionFull, contFull = 6.0, 60.0
)

// still is what the candidate pass reads of a keyframe.
type still struct {
	t        float64
	mean     float64
	p95      int
	contrast float64
	risk     float64 // pictureRisk; read for usable stills only
}

func readStill(f media.Frame) still {
	var hist [256]int
	sum := 0
	for _, y := range f.Y {
		hist[y]++
		sum += int(y)
	}
	n := len(f.Y)
	mean := float64(sum) / float64(n)
	variance := 0.0
	for v, c := range hist {
		d := float64(v) - mean
		variance += d * d * float64(c)
	}
	p95, seen := 0, 0
	for v, c := range hist {
		seen += c
		if seen >= n*95/100 {
			p95 = v
			break
		}
	}
	return still{t: f.Time, mean: mean, p95: p95, contrast: math.Sqrt(variance / float64(n))}
}

func (s still) usable() bool {
	return s.p95 >= blackP95 && s.mean >= minMean && s.mean <= maxMean && s.contrast >= minContrast
}

// rank is what a stretch's stills compete on: contrast, less the risk of the picture.
func (s still) rank() float64 { return s.contrast * (1 - s.risk) }

// pickStills chooses the best ranked usable still of each of n equal stretches of
// [from, to), leaving out the ones to avoid.
func pickStills(stills []still, from, to float64, n int) []still {
	best := make([]*still, n)
	for i := range stills {
		s := &stills[i]
		k := int(float64(n) * (s.t - from) / (to - from))
		if k < 0 || k >= n || !s.usable() || s.risk >= avoidRisk {
			continue
		}
		if best[k] == nil || s.rank() > best[k].rank() {
			best[k] = s
		}
	}
	var out []still
	for _, s := range best {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// measureShot reads the frames decoded from a candidate on (shotFPS): its shot ends at
// the first cut or near-black frame, at most shotMaxLength in; motion is the mean
// difference between frames (percent of full scale).
func measureShot(frames []media.Frame) (length, motion float64) {
	prevMAFD, total := 0.0, 0.0
	n := 0
	for i := 1; i < len(frames); i++ {
		mafd := frameDiff(frames[i-1], frames[i])
		if min(mafd, math.Abs(mafd-prevMAFD)) >= sceneThreshold || readStill(frames[i]).p95 < blackP95 {
			break
		}
		prevMAFD = mafd
		total += mafd
		n++
	}
	length = min(float64(n)/shotFPS, shotMaxLength)
	if n > 0 {
		motion = total / float64(n)
	}
	return length, motion
}

// frameDiff is the mean absolute difference of two frames as a percentage of full scale
// (scdet's MAFD).
func frameDiff(a, b media.Frame) float64 {
	sum := 0
	for i := range a.Y {
		d := int(a.Y[i]) - int(b.Y[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return float64(sum) * 100 / float64(len(a.Y)*255)
}

// shotScore rates a shot for a promo: mostly motion, some contrast; 0 to 1.
func shotScore(motion, contrast float64) float64 {
	s := 0.7*min(motion/motionFull, 1) + 0.3*min(contrast/contFull, 1)
	return math.Round(s*1000) / 1000
}

// shotRating is a shot's score less the risk of its picture: a face close-up keeps
// 30 % of it, so a wide or medium shot wins over one that moves as much.
func shotRating(score, risk float64) float64 {
	return math.Round(score*(1-risk)*1000) / 1000
}

// shotRisk reads a shot's pictures. The risk rates the shot: the highest of the frames checked
// every riskEvery frames and at the last. Whether the shot is to be avoided is decided on every
// frame, because a wound shows for a fraction of a second and the checks fall between: it is
// when two frames in a row reach avoidRisk (one odd frame does not drop a shot).
func shotRisk(frames []media.Frame) (risk float64, avoid bool) {
	each := make([]float64, len(frames))
	for i, f := range frames {
		each[i] = pictureRisk(rgbOf(f), f.Width, f.Height)
		if i%riskEvery == 0 || i == len(frames)-1 {
			risk = max(risk, each[i])
		}
	}
	return risk, steadyRisk(each) >= avoidRisk
}

// candidate is a measured shot of one of the title's files.
type candidate struct {
	shot  CouchTvShot
	file  int // index of the file it is from, for the order
	t     float64
	risk  float64
	avoid bool // its picture is never to air (shotRisk)
}

// chooseShots keeps the best candidates, at most perFile from one file, in the order
// they come in the title. Candidates whose picture is to be avoided never air.
func chooseShots(cands []candidate, perFile int) []CouchTvShot {
	cands = slices.DeleteFunc(cands, func(c candidate) bool { return c.avoid })
	slices.SortStableFunc(cands, func(a, b candidate) int { return cmp.Compare(b.shot.Score, a.shot.Score) })
	var chosen []candidate
	taken := map[int]int{}
	for _, c := range cands {
		if len(chosen) < shotsMax && taken[c.file] < perFile {
			chosen = append(chosen, c)
			taken[c.file]++
		}
	}
	slices.SortFunc(chosen, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.t, b.t))
	})
	out := []CouchTvShot{}
	for _, c := range chosen {
		out = append(out, c.shot)
	}
	return out
}

// fileShots finds a file's candidate shots: one per stretch of its first half.
func (a *Analyzer) fileShots(ctx context.Context, f titleFile, index, stretches int) ([]candidate, error) {
	from, to := f.Duration*shotWindowFrom, f.Duration*shotWindowTo
	var stills []still
	err := media.Frames(ctx, a.FFmpegPath, f.Path, media.FrameOptions{
		From: from, Length: to - from, Keyframes: true, Width: shotW, Height: shotH, Chroma: true, FullRange: true,
	}, func(fr media.Frame) error {
		s := readStill(fr)
		if s.usable() {
			s.risk = pictureRisk(rgbOf(fr), fr.Width, fr.Height)
		}
		stills = append(stills, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []candidate
	for _, s := range pickStills(stills, from, to, stretches) {
		var frames []media.Frame
		err := media.Frames(ctx, a.FFmpegPath, f.Path, media.FrameOptions{
			From: s.t, Length: shotMaxLength + 0.2, FPS: shotFPS, Width: shotW, Height: shotH, Chroma: true, FullRange: true,
		}, func(fr media.Frame) error {
			fr.Y, fr.U, fr.V = slices.Clone(fr.Y), slices.Clone(fr.U), slices.Clone(fr.V)
			frames = append(frames, fr)
			return nil
		})
		if err != nil {
			return nil, err
		}
		length, motion := measureShot(frames)
		if length < shotMinLength {
			continue
		}
		frames25 := int(length / airFrame)
		risk, avoid := shotRisk(frames[:min(len(frames), int(length*shotFPS)+1)])
		out = append(out, candidate{
			shot: CouchTvShot{MediaFileID: f.ID, StartMs: int(math.Round(s.t * 1000)), DurationMs: frames25 * 40,
				Score: shotRating(shotScore(motion, s.contrast), risk)},
			file: index, t: s.t, risk: risk, avoid: avoid,
		})
	}
	return out, nil
}

// shots picks a title's footage: a movie's from 12 stretches of its first half, a
// series' from three episodes spread over it, two at most from each.
func (a *Analyzer) shots(ctx context.Context, kind string, files []titleFile) []CouchTvShot {
	picks, stretches, perFile := []titleFile{files[0]}, 12, shotsMax
	if kind == "series" {
		picks, stretches, perFile = spread(files, 3), 4, 2
	}
	var cands []candidate
	for i, f := range picks {
		if !f.Readable {
			continue
		}
		c, err := a.fileShots(ctx, f, i, stretches)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("couchtv: no shots", "mediaFileId", f.ID, "err", err)
			}
			continue
		}
		cands = append(cands, c...)
	}
	return chooseShots(cands, perFile)
}

// spread picks up to n files evenly spread over the list (at 1/2n, 3/2n, ...).
func spread(files []titleFile, n int) []titleFile {
	if len(files) <= n {
		return files
	}
	out := make([]titleFile, 0, n)
	for k := range n {
		out = append(out, files[len(files)*(2*k+1)/(2*n)])
	}
	return out
}
