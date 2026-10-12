package couchtv

import (
	"cmp"
	"context"
	"math"
	"slices"

	"couchverse/internal/feature/library"
	"couchverse/internal/media"
)

// The promo soundtrack (D25): a series' theme from its intro markers, a movie's credits
// music, else its loudest steady stretch. The excerpt is at most promoMax long; the
// composer cuts the beds it needs from it.
const (
	promoMax       = 40.0
	promoMinSteady = 20.0
)

// trustedIntros is how many episodes of a season must share an intro for it to count
// as the theme before seasons with fewer.
const trustedIntros = 3

// strongestIntro is the intro a series' promos play: from the season whose episodes
// most often share one (seasons where at least trustedIntros do first, then the bigger
// season, then the longer intro), the episode with the season's median intro. sizes
// counts each season's episodes.
func strongestIntro(intros []marker, sizes map[int]int) (marker, bool) {
	bySeason := map[int][]marker{}
	for _, m := range intros {
		bySeason[m.Season] = append(bySeason[m.Season], m)
	}
	type pick struct {
		trusted bool
		share   float64
		size    int
		median  marker
	}
	better := func(p, q pick) bool {
		if p.trusted != q.trusted {
			return p.trusted
		}
		if p.share != q.share {
			return p.share > q.share
		}
		if p.size != q.size {
			return p.size > q.size
		}
		return length(p.median) > length(q.median)
	}
	var best *pick
	for season, ms := range bySeason {
		slices.SortFunc(ms, func(a, b marker) int {
			return cmp.Or(cmp.Compare(length(a), length(b)), cmp.Compare(a.FileID, b.FileID))
		})
		size := max(sizes[season], len(ms))
		p := pick{trusted: len(ms) >= trustedIntros, share: float64(len(ms)) / float64(size), size: size, median: ms[len(ms)/2]}
		if best == nil || better(p, *best) {
			best = &p
		}
	}
	if best == nil {
		return marker{}, false
	}
	return best.median, true
}

func length(m marker) int { return m.End - m.Start }

// loudestSteady is the loudest steady stretch (music, typically) of a whole film that
// is at least promoMinSteady long.
func loudestSteady(l *media.Loudness) (media.Stretch, bool) {
	stretches := slices.DeleteFunc(l.Steady(), func(s media.Stretch) bool { return s.Length() < promoMinSteady })
	if len(stretches) == 0 {
		return media.Stretch{}, false
	}
	return slices.MaxFunc(stretches, func(a, b media.Stretch) int { return cmp.Compare(a.Loudness, b.Loudness) }), true
}

// promo picks a title's promo excerpt; nil when nothing suits. profile is a movie's
// whole-film loudness when it was measured in this run.
func (a *Analyzer) promo(ctx context.Context, t *titleRow, files []titleFile, profile *media.Loudness) (*CouchTvPromoAudio, error) {
	readable := map[string]titleFile{}
	for _, f := range files {
		if f.Readable {
			readable[f.ID] = f
		}
	}
	if t.TitleKind == "series" {
		intros, err := a.Store.Markers(ctx, t.TitleID, library.MarkerIntro)
		if err != nil {
			return nil, err
		}
		intros = slices.DeleteFunc(intros, func(m marker) bool { _, ok := readable[m.FileID]; return !ok })
		sizes, err := a.Store.SeasonSizes(ctx, t.TitleID)
		if err != nil {
			return nil, err
		}
		m, ok := strongestIntro(intros, sizes)
		if !ok {
			return nil, nil
		}
		return a.excerpt(ctx, readable[m.FileID], float64(m.Start)/1000, float64(m.End)/1000, nil)
	}
	movie, ok := readable[files[0].ID]
	if !ok {
		return nil, nil
	}
	music, err := a.Store.Markers(ctx, t.TitleID, library.MarkerCreditsMusic)
	if err != nil {
		return nil, err
	}
	for _, m := range music {
		if m.FileID == movie.ID {
			return a.excerpt(ctx, movie, float64(m.Start)/1000, float64(m.End)/1000, profile)
		}
	}
	if profile == nil {
		if profile, err = media.MeasureLoudness(ctx, a.FFmpegPath, movie.Path, 0, 0); err != nil {
			return nil, err
		}
	}
	s, ok := loudestSteady(profile)
	if !ok {
		return nil, nil
	}
	return a.excerpt(ctx, movie, s.Start, s.End, profile)
}

// excerpt is the loudest promoMax of [start, end) of a file, or all of it when shorter.
// profile, when given, is the file's loudness from its start.
func (a *Analyzer) excerpt(ctx context.Context, f titleFile, start, end float64, profile *media.Loudness) (*CouchTvPromoAudio, error) {
	at := start
	if end-start > promoMax {
		if profile != nil {
			at = profile.Loudest(start, end, promoMax)
		} else {
			l, err := media.MeasureLoudness(ctx, a.FFmpegPath, f.Path, start, end-start)
			if err != nil {
				return nil, err
			}
			at = start + l.Loudest(0, end-start, promoMax)
		}
	}
	return &CouchTvPromoAudio{
		MediaFileID: f.ID,
		StartMs:     int(math.Round(at * 1000)),
		DurationMs:  int(math.Round(min(end-at, promoMax) * 1000)),
		Source:      SourceAuto,
	}, nil
}
