package couchtv

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"

	"couchverse/internal/feature/catalog"
)

// genreClasses are the genres behind each pick, by their English identity. TMDB names
// genres in the title's base language, so the Czech labels match too.
var genreClasses = map[string][]string{
	PickComedy:    {"Comedy"},
	PickAnimation: {"Animation"},
	PickDrama:     {"Drama", "Crime", "Mystery", "Thriller"},
	PickFamily:    {"Family", "Kids", "Animation"},
}

func hasGenre(genres, names []string) bool {
	for _, g := range genres {
		for _, n := range names {
			if strings.EqualFold(g, n) || strings.EqualFold(g, catalog.GenreLabel(n, "cs")) {
				return true
			}
		}
	}
	return false
}

// matches reports whether a title's genres suit a pick. A drama pick wants live-action
// drama, so a comedy or a cartoon with a dark streak stays with the sitcoms.
func matches(pick string, genres []string) bool {
	switch pick {
	case "", PickAny:
		return true
	case PickDrama:
		if hasGenre(genres, []string{"Comedy", "Animation"}) {
			return false
		}
	}
	return hasGenre(genres, genreClasses[pick])
}

func pickOf(s CouchTvSlot) string {
	if s.Pick == "" {
		return PickAny
	}
	return s.Pick
}

// assignSeries gives every strip and marathon slot its series, once for the whole
// template: pinned series first, then strips of a genre, strips of any genre and
// marathons, each taking the series of its genre with the most episodes that no slot
// has yet. A marathon that finds none shares the biggest series of its genre, cursor
// included, so a weekday strip catches up at the weekend.
func (p *planner) assignSeries() {
	p.assigned = map[string]*Series{}
	taken := map[string]bool{}
	inOrder := func(s CouchTvSlot) bool { return s.Fill == FillStrip || s.Fill == FillMarathon }
	for _, s := range p.cfg.Template.Slots {
		if inOrder(s) && len(s.Titles) > 0 {
			if ser := p.seriesByID[s.Titles[0]]; ser != nil {
				p.assigned[s.Key], taken[ser.ID] = ser, true
			}
		}
	}
	passes := []func(CouchTvSlot) bool{
		func(s CouchTvSlot) bool { return s.Fill == FillStrip && pickOf(s) != PickAny },
		func(s CouchTvSlot) bool { return s.Fill == FillStrip && pickOf(s) == PickAny },
		func(s CouchTvSlot) bool { return s.Fill == FillMarathon },
	}
	ranked := slices.Clone(p.series)
	slices.SortStableFunc(ranked, func(a, b *Series) int { return cmp.Compare(len(b.Episodes), len(a.Episodes)) })
	for _, pass := range passes {
		for _, s := range p.cfg.Template.Slots {
			if !inOrder(s) || !pass(s) || p.assigned[s.Key] != nil {
				continue
			}
			if ser := bestSeries(ranked, pickOf(s), taken); ser != nil {
				p.assigned[s.Key], taken[ser.ID] = ser, true
			}
		}
	}
}

func bestSeries(ranked []*Series, pick string, taken map[string]bool) *Series {
	for _, want := range []struct{ genre, free bool }{{true, true}, {true, false}, {false, true}, {false, false}} {
		for _, s := range ranked {
			if (!want.genre || matches(pick, s.Genres)) && (!want.free || !taken[s.ID]) {
				return s
			}
		}
	}
	return nil
}

// pick chooses the slot's next program, starting at start and ending before limit.
func (p *planner) pick(s *daySlot, start, limit time.Time) (placement, bool) {
	mode := p.readinessAt(start)
	switch s.Fill {
	case FillStrip, FillMarathon:
		if ser := p.assigned[s.Key]; ser != nil && ser.Random {
			return p.pickAtRandom(s, ser, start, limit, mode)
		}
		return p.pickInOrder(s, start, limit)
	case FillReruns:
		var soft time.Time
		if s.softEnd {
			soft = s.windowEnd
		}
		return p.pickRerun(s.Key, p.rerunPool(s.CouchTvSlot), start, limit, soft, FillReruns, mode)
	case FillMovie:
		return p.pickPrime(&s.CouchTvSlot, start, limit, mode)
	case FillMatinee:
		return p.pickMatinee(&s.CouchTvSlot, start, limit, FillMatinee, mode)
	case FillPremiere:
		return p.pickPremiere(&s.CouchTvSlot, start, limit, mode)
	case FillSaga:
		return p.pickSaga(&s.CouchTvSlot, start, limit, mode)
	case FillEncore:
		return p.pickEncore(&s.CouchTvSlot, start, limit, mode)
	}
	return placement{}, false
}

// cursor is the index of the series' next strip episode: after its latest strip or
// marathon airing (wrapping around at the end), or where an admin restarted it.
func (p *planner) cursor(ser *Series) int {
	m, aired := p.strips[ser.ID]
	if c, ok := p.cursors[ser.ID]; ok && (!aired || m.at.Before(c.From)) {
		if ref, ok := p.episodeAt[c.EpisodeID]; ok && ref.series == ser {
			return ref.index
		}
	}
	if !aired {
		return 0
	}
	ref, ok := p.episodeAt[m.id]
	if !ok || ref.series != ser {
		return 0
	}
	return (ref.index + 1) % len(ser.Episodes)
}

// pickInOrder is a strip's or marathon's next episode. One that does not fit ends the
// block for the day and airs the next day; one longer than the slot could ever hold is
// passed over, or it would stall the strip for good, and so is one an admin took off
// the day.
func (p *planner) pickInOrder(s *daySlot, start, limit time.Time) (placement, bool) {
	ser := p.assigned[s.Key]
	if ser == nil {
		return placement{}, false
	}
	skip := func(e Episode) bool { return e.onAir()+p.minBreak > s.maxRoom || p.removed(e.ID) }
	i := p.cursor(ser)
	for n := 0; n < len(ser.Episodes) && skip(ser.Episodes[i]); n++ {
		i = (i + 1) % len(ser.Episodes)
	}
	ep := ser.Episodes[i]
	if p.removed(ep.ID) || !p.fits(ep.Media, start, limit) {
		return placement{}, false
	}
	if !ep.Ready && start.Before(p.now.Add(swapAhead)) {
		// too close to air unprepared: a prepared rerun stands in and the strip waits
		for _, pool := range [][]*Series{{ser}, p.series} {
			if pl, ok := p.pickRerun(s.Key, pool, start, limit, time.Time{}, FillReruns, onlyReady); ok {
				return pl, true
			}
		}
	}
	return placement{ser.ID, ep.ID, ep.Media, s.Fill}, true
}

// pickAtRandom is a strip's or marathon's next episode of a series whose episodes may
// air in any order, so it has no cursor: any episode that fits (randomEpisode). Too close
// to air with nothing of it prepared, a prepared rerun stands in, as for a strip in order.
func (p *planner) pickAtRandom(s *daySlot, ser *Series, start, limit time.Time, mode readiness) (placement, bool) {
	for _, pass := range mode.passes() {
		if ep, ok := p.randomEpisode(ser, start, limit, pass); ok {
			return placement{ser.ID, ep.ID, ep.Media, s.Fill}, true
		}
		if pass == onlyReady {
			if pl, ok := p.pickRerun(s.Key, p.series, start, limit, time.Time{}, FillReruns, onlyReady); ok {
				return pl, true
			}
		}
	}
	return placement{}, false
}

// randomEpisode picks an episode of a series airing at random that fits before limit and
// does not air today already: one that never aired, in the day's random order, else the
// one aired longest ago, outside rerunGapDays first. One that a fixed program airs within
// rerunGapDays after start goes last, since it would repeat that soon.
func (p *planner) randomEpisode(ser *Series, start, limit time.Time, pass readiness) (Episode, bool) {
	days := p.cfg.Rotation.RerunGapDays
	since, until := start.AddDate(0, 0, -days), start.AddDate(0, 0, days)
	unplanned := func(e Episode) bool { return !p.fixedIn(e.ID, start, until) }
	tiers := []func(Episode) bool{
		func(e Episode) bool { return p.lastAired[e.ID].IsZero() && unplanned(e) },
		func(e Episode) bool { return p.lastAired[e.ID].Before(since) && unplanned(e) },
		unplanned,
		func(Episode) bool { return true },
	}
	for t, tier := range tiers {
		var cands []Episode
		for _, e := range ser.Episodes {
			if tier(e) && p.fits(e.Media, start, limit) && !p.removed(e.ID) && !p.airsToday(e.ID) {
				cands = append(cands, e)
			}
		}
		cands = byReadiness(cands, func(e Episode) bool { return e.Ready }, pass)
		if len(cands) == 0 {
			continue
		}
		if t == 0 {
			return slices.MinFunc(cands, func(a, b Episode) int { return cmp.Compare(p.day.draw(a.ID), p.day.draw(b.ID)) }), true
		}
		return slices.MinFunc(cands, func(a, b Episode) int { return p.lastAired[a.ID].Compare(p.lastAired[b.ID]) }), true
	}
	return Episode{}, false
}

// rerunPool is the series a reruns slot draws from: its pinned series, else those of
// its genre, else every series.
func (p *planner) rerunPool(s CouchTvSlot) []*Series {
	var pool []*Series
	for _, id := range s.Titles {
		if ser := p.seriesByID[id]; ser != nil {
			pool = append(pool, ser)
		}
	}
	if len(pool) > 0 {
		return pool
	}
	for _, ser := range p.series {
		if matches(pickOf(s), ser.Genres) {
			pool = append(pool, ser)
		}
	}
	if len(pool) == 0 {
		return p.series
	}
	return pool
}

// pickRerun takes turns through the pool's series (from a seeded first one) and picks
// an episode that fits: one that aired before (any, for a series airing at random) but
// not within rerunGapDays, else any not within it, else the one aired longest ago. With
// a soft end, the episode must be at least half over by then.
func (p *planner) pickRerun(key string, pool []*Series, start, limit, soft time.Time, fill string, mode readiness) (placement, bool) {
	if len(pool) == 0 {
		return placement{}, false
	}
	turn, ok := p.day.turns[key]
	if !ok {
		turn = p.day.rng.IntN(len(pool))
	}
	gap := start.AddDate(0, 0, -p.cfg.Rotation.RerunGapDays)
	tiers := []func(*Series, Episode) bool{
		func(s *Series, e Episode) bool {
			at := p.lastAired[e.ID]
			return (s.Random || !at.IsZero()) && at.Before(gap)
		},
		func(_ *Series, e Episode) bool { return p.lastAired[e.ID].Before(gap) },
		func(*Series, Episode) bool { return true },
	}
	for _, pass := range mode.passes() {
		for t, tier := range tiers {
			for k := range pool {
				ser := pool[(turn+k)%len(pool)]
				var cands []Episode
				for _, e := range ser.Episodes {
					if p.fits(e.Media, start, limit) && (soft.IsZero() || !start.Add(e.onAir()/2).After(soft)) && tier(ser, e) &&
						!p.removed(e.ID) {
						cands = append(cands, e)
					}
				}
				cands = byReadiness(cands, func(e Episode) bool { return e.Ready }, pass)
				if len(cands) == 0 {
					continue
				}
				ep := cands[0]
				if t < len(tiers)-1 {
					ep = cands[p.day.rng.IntN(len(cands))]
				} else {
					for _, e := range cands[1:] {
						if p.lastAired[e.ID].Before(p.lastAired[ep.ID]) {
							ep = e
						}
					}
				}
				p.day.turns[key] = turn + k + 1
				return placement{ser.ID, ep.ID, ep.Media, fill}, true
			}
		}
	}
	return placement{}, false
}

func byReadiness[T any](cands []T, ready func(T) bool, mode readiness) []T {
	if mode == anyMedia {
		return cands
	}
	prepared := slices.DeleteFunc(slices.Clone(cands), func(c T) bool { return !ready(c) })
	if len(prepared) > 0 || mode == onlyReady {
		return prepared
	}
	return cands
}

// pickFiller fills a gap before a fixed time: a rerun from every series, else a movie
// that fits.
func (p *planner) pickFiller(start, end time.Time) (placement, bool) {
	mode := p.readinessAt(start)
	if pl, ok := p.pickRerun(FillFiller, p.series, start, end, time.Time{}, FillFiller, mode); ok {
		return pl, true
	}
	return p.pickMatinee(nil, start, end, FillFiller, mode)
}

// movieCands are the movies a slot may air that fit before limit: its pinned movies,
// else every movie, leaving out those an admin took off the day.
func (p *planner) movieCands(s *CouchTvSlot, start, limit time.Time) []*Movie {
	pool := p.movies
	if s != nil && len(s.Titles) > 0 {
		var pinned []*Movie
		for _, id := range s.Titles {
			if m := p.movieByID[id]; m != nil {
				pinned = append(pinned, m)
			}
		}
		if len(pinned) > 0 {
			pool = pinned
		}
	}
	var out []*Movie
	for _, m := range pool {
		if p.fits(m.Media, start, limit) && !p.removed(m.ID) {
			out = append(out, m)
		}
	}
	return out
}

// chooseMovie takes the first tier with a candidate, the slot's genre before the rest
// within a tier, and in it the movie that comes first by key (oldest first), seeded
// ties broken at random.
func (p *planner) chooseMovie(cands []*Movie, pick string, tiers []func(*Movie) bool, mode readiness, key func(*Movie) time.Time) *Movie {
	for _, pass := range mode.passes() {
		for _, tier := range tiers {
			for _, genre := range []bool{true, false} {
				var list []*Movie
				for _, m := range cands {
					if tier(m) && (!genre || matches(pick, m.Genres)) {
						list = append(list, m)
					}
				}
				list = byReadiness(list, func(m *Movie) bool { return m.Ready }, pass)
				if len(list) == 0 {
					continue
				}
				p.day.rng.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
				slices.SortStableFunc(list, func(a, b *Movie) int { return key(a).Compare(key(b)) })
				return list[0]
			}
		}
	}
	return nil
}

func (p *planner) held(m *Movie) bool { return p.day.held[m.ID] }

func (p *planner) airedToday(m *Movie) bool { return !p.lastAired[m.ID].Before(p.day.start) }

// pickPrime is the prime-time rotation: a movie returns to prime time only after
// max(primeRepeatMinDays, primeRepeatFactor x the movie count) days, longest away
// first; movies held for the premiere or the saga wait for their own slot.
func (p *planner) pickPrime(s *CouchTvSlot, start, limit time.Time, mode readiness) (placement, bool) {
	since := start.Add(-p.primeWindow())
	gap := start.AddDate(0, 0, -p.cfg.Rotation.MovieGapDays)
	m := p.chooseMovie(p.movieCands(s, start, limit), pickOf(*s), []func(*Movie) bool{
		func(m *Movie) bool {
			return !p.held(m) && p.lastPrime[m.ID].Before(since) && p.lastAired[m.ID].Before(gap)
		},
		func(m *Movie) bool { return !p.held(m) && !p.airedToday(m) },
		func(*Movie) bool { return true },
	}, mode, func(m *Movie) time.Time { return p.lastPrime[m.ID] })
	if m == nil {
		return placement{}, false
	}
	return placement{m.ID, "", m.Media, FillMovie}, true
}

// primeWindow is how long a movie stays out of prime time after airing there:
// max(primeRepeatMinDays, primeRepeatFactor x the movie count) days.
func (p *planner) primeWindow() time.Duration {
	r := p.cfg.Rotation
	return time.Duration(math.Max(float64(r.PrimeRepeatMinDays), r.PrimeRepeatFactor*float64(len(p.movies))) * float64(24*time.Hour))
}

// pickMatinee reruns a movie outside prime time: one that aired in prime time recently
// (so the rerun takes nothing from the rotation), else one that ever did, else any;
// none aired within movieGapDays, the one aired longest ago first; then any not aired
// today. Without a slot it serves the filler.
func (p *planner) pickMatinee(s *CouchTvSlot, start, limit time.Time, fill string, mode readiness) (placement, bool) {
	pick := PickAny
	if s != nil {
		pick = pickOf(*s)
	}
	since := start.Add(-p.primeWindow())
	gap := start.AddDate(0, 0, -p.cfg.Rotation.MovieGapDays)
	m := p.chooseMovie(p.movieCands(s, start, limit), pick, []func(*Movie) bool{
		func(m *Movie) bool {
			return !p.held(m) && p.lastPrime[m.ID].After(since) && p.lastAired[m.ID].Before(gap)
		},
		func(m *Movie) bool { return !p.held(m) && !p.lastPrime[m.ID].IsZero() && p.lastAired[m.ID].Before(gap) },
		func(m *Movie) bool { return !p.held(m) && p.lastAired[m.ID].Before(gap) },
		func(m *Movie) bool { return !p.held(m) && !p.airedToday(m) },
		func(*Movie) bool { return true },
	}, mode, func(m *Movie) time.Time { return p.lastAired[m.ID] })
	if m == nil {
		return placement{}, false
	}
	return placement{m.ID, "", m.Media, fill}, true
}

// premiereCandidate is the newest movie added within premiereWindowDays of at that
// never aired (D24).
func (p *planner) premiereCandidate(at time.Time, cands []*Movie, prepared bool) *Movie {
	since := at.AddDate(0, 0, -p.cfg.Rotation.PremiereWindowDays)
	var best *Movie
	for _, m := range cands {
		if !p.lastAired[m.ID].IsZero() || m.AddedAt.Before(since) || prepared && !m.Ready {
			continue
		}
		if best == nil || m.AddedAt.After(best.AddedAt) {
			best = m
		}
	}
	return best
}

// pickPremiere premieres the newest movie that never aired, else runs the rotation.
// Like a strip, the premiere keeps its movie while it is being prepared, until it is
// too close to air unprepared.
func (p *planner) pickPremiere(s *CouchTvSlot, start, limit time.Time, mode readiness) (placement, bool) {
	cands := p.movieCands(s, start, limit)
	m := p.premiereCandidate(start, cands, false)
	if m != nil && !m.Ready && start.Before(p.now.Add(swapAhead)) {
		m = p.premiereCandidate(start, cands, true)
	}
	if m != nil {
		return placement{m.ID, "", m.Media, FillPremiere}, true
	}
	return p.pickPrime(s, start, limit, mode)
}

// pickSaga airs the next film of the running franchise (D30), else runs the rotation.
// Pinned titles narrow the saga to the franchises they belong to.
func (p *planner) pickSaga(s *CouchTvSlot, start, limit time.Time, mode readiness) (placement, bool) {
	var allowed func(sagaGroup) bool
	if len(s.Titles) > 0 {
		allowed = func(g sagaGroup) bool {
			return slices.ContainsFunc(g.movies, func(m *Movie) bool { return slices.Contains(s.Titles, m.ID) })
		}
	}
	m := p.nextSagaFilm(allowed)
	if m != nil && !p.removed(m.ID) && p.fits(m.Media, start, limit) && (m.Ready || !start.Before(p.now.Add(swapAhead))) {
		return placement{m.ID, "", m.Media, FillSaga}, true
	}
	return p.pickPrime(s, start, limit, mode)
}

// pickEncore airs tonight's prime-time movie again, else a matinee.
func (p *planner) pickEncore(s *CouchTvSlot, start, limit time.Time, mode readiness) (placement, bool) {
	if m := p.day.prime; m != nil && !p.removed(m.ID) && p.fits(m.Media, start, limit) && (m.Ready || !start.Before(p.now.Add(swapAhead))) {
		return placement{m.ID, "", m.Media, FillEncore}, true
	}
	return p.pickMatinee(s, start, limit, FillMatinee, mode)
}

// standIn is a prepared program of the same kind for an airing that cannot air.
func (p *planner) standIn(a Airing, limit time.Time) (placement, bool) {
	if a.EpisodeID != "" {
		fill := a.Fill
		if fill == FillStrip || fill == FillMarathon {
			fill = FillReruns
		}
		var pools [][]*Series
		if ser := p.seriesByID[a.TitleID]; ser != nil {
			pools = append(pools, []*Series{ser})
		}
		for _, pool := range append(pools, p.series) {
			if pl, ok := p.pickRerun(a.Slot, pool, a.StartsAt, limit, time.Time{}, fill, onlyReady); ok {
				return pl, true
			}
		}
		return placement{}, false
	}
	fill := a.Fill
	switch fill {
	case FillMovie, FillPremiere, FillSaga:
		fill = FillMovie
	case FillEncore:
		fill = FillMatinee
	}
	return p.pickMatinee(nil, a.StartsAt, limit, fill, onlyReady)
}
