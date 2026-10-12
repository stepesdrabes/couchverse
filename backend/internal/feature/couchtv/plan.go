package couchtv

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"
)

// Readiness horizons (plan 5.2): a program further out than prepareAhead may be
// unprepared (its preparation gets queued); closer, free choices prefer prepared media;
// within swapAhead an unprepared program is swapped for a prepared one.
const (
	prepareAhead = 48 * time.Hour
	swapAhead    = 6 * time.Hour
)

// flowStep is the grid flow programs start on.
const flowStep = 5 * time.Minute

// PlanInput is everything Plan reads.
type PlanInput struct {
	Config  CouchTvConfig
	Library Library
	// History holds programs before the first planned day. The latest airing of every
	// movie and episode per fill is all the rules read (Store.History).
	History []Airing
	// Fixed are the programs inside the planned days that stay as they are: admin rows,
	// locked rows and rows already on air. The plan flows around them.
	Fixed []Airing
	// Removed are programs an admin took off (StateReplaced): planning the broadcast day
	// one was in leaves its movie or episode out.
	Removed []Airing
	Cursors []Cursor
	// From is the first broadcast day to plan (its date; the clock is ignored) and Days
	// how many to plan.
	From time.Time
	Days int
	Now  time.Time
}

// Plan generates whole broadcast days from the weekly template: programs with exact
// UTC times and the breaks between them. It is a pure function of its input; every
// day draws from its own seed, so planning an unchanged day again gives the same day.
// The output holds the fixed programs too, and the break leading into the first day.
func Plan(in PlanInput) []Airing {
	p := newPlanner(in.Config, in.Library, in.History, in.Cursors, in.Now)
	prevEnd := p.prevEnd
	fixed := make([]Airing, 0, len(in.Fixed))
	for _, f := range in.Fixed {
		if f.Kind == KindProgram {
			fixed = append(fixed, f)
		}
	}
	slices.SortStableFunc(fixed, func(a, b Airing) int { return a.StartsAt.Compare(b.StartsAt) })
	date := p.civil(in.From)
	for range in.Days {
		start, end := p.cfg.dayStart(date), p.cfg.dayStart(nextDate(date))
		within := func(a Airing) bool { return !a.StartsAt.Before(start) && a.StartsAt.Before(end) }
		var today, removed []Airing
		for _, f := range fixed {
			if within(f) {
				today = append(today, f)
			}
		}
		for _, r := range in.Removed {
			if within(r) {
				removed = append(removed, r)
			}
		}
		p.planDay(date, today, removed)
		date = nextDate(date)
	}
	return withBreaks(p.out, prevEnd)
}

// SwapInput is everything SwapUnready reads.
type SwapInput struct {
	Config  CouchTvConfig
	Library Library
	History []Airing // programs before Rows
	Rows    []Airing // the programs from now on, in order
	Now     time.Time
}

// SwapUnready replaces every auto program due within swapAhead whose media is not
// prepared with a prepared one of the same kind that ends before the next program
// (plan 5.2). It returns the changed rows, with their IDs; the break after each grows.
// A strip's stand-in is a rerun, so the strip airs that episode another day.
func SwapUnready(in SwapInput) []Airing {
	p := newPlanner(in.Config, in.Library, in.History, nil, in.Now)
	p.swapping = true
	var out []Airing
	for i, a := range in.Rows {
		due := !a.StartsAt.Before(in.Now) && a.StartsAt.Before(in.Now.Add(swapAhead))
		if a.Kind != KindProgram || a.Source != SourceAuto || a.Locked || !due || p.ready(a) {
			p.record(a)
			continue
		}
		limit := a.EndsAt.Add(p.minBreak)
		if i+1 < len(in.Rows) {
			limit = in.Rows[i+1].StartsAt
		}
		p.day = p.newDay(p.cfg.dayOf(a.StartsAt), nil, nil)
		pl, ok := p.standIn(a, limit)
		if !ok {
			p.record(a)
			continue
		}
		b := a
		b.TitleID, b.EpisodeID, b.MediaFileID, b.Fill = pl.titleID, pl.episodeID, pl.media.FileID, pl.fill
		b.EndsAt, b.CreditsAt = a.StartsAt.Add(pl.media.onAir()), pl.media.CreditsAt
		out = append(out, b)
		p.record(b)
	}
	return out
}

// mark is the latest airing of something: when, and which item.
type mark struct {
	at time.Time
	id string
}

// readiness is how much a choice cares whether media is prepared.
type readiness int

const (
	anyMedia    readiness = iota // far enough out to prepare whatever is chosen
	preferReady                  // within a rule's tier, prepared media first
	readyFirst                   // prepared media under any tier before unprepared under the best
	onlyReady                    // prepared media or nothing
)

// passes are the readiness filters a choice tries in turn.
func (r readiness) passes() []readiness {
	if r == readyFirst {
		return []readiness{onlyReady, anyMedia}
	}
	return []readiness{r}
}

type planner struct {
	cfg       CouchTvConfig
	loc       *time.Location
	now       time.Time
	minBreak  time.Duration
	fillerMin time.Duration
	// swapping makes every choice demand prepared media
	swapping bool

	movies     []*Movie
	series     []*Series
	movieByID  map[string]*Movie
	seriesByID map[string]*Series
	episodeAt  map[string]episodeRef
	assigned   map[string]*Series // strip and marathon slot key -> its series
	cursors    map[string]Cursor
	groups     []sagaGroup

	// what has aired, kept current as programs are placed
	lastAired map[string]time.Time // movie or episode id
	lastPrime map[string]time.Time // movie id
	lastSaga  map[string]time.Time // movie id
	sagaMark  mark                 // the latest saga airing
	strips    map[string]mark      // series id -> its latest strip or marathon episode
	prevEnd   time.Time

	out []Airing
	day *dayState
}

type episodeRef struct {
	series *Series
	index  int
}

func newPlanner(cfg CouchTvConfig, lib Library, history []Airing, cursors []Cursor, now time.Time) *planner {
	p := &planner{
		cfg:        cfg,
		loc:        cfg.location(),
		now:        now,
		minBreak:   time.Duration(cfg.BreakMinSeconds) * time.Second,
		fillerMin:  time.Duration(cfg.Rotation.FillerMinMinutes) * time.Minute,
		movieByID:  map[string]*Movie{},
		seriesByID: map[string]*Series{},
		episodeAt:  map[string]episodeRef{},
		cursors:    map[string]Cursor{},
		lastAired:  map[string]time.Time{},
		lastPrime:  map[string]time.Time{},
		lastSaga:   map[string]time.Time{},
		strips:     map[string]mark{},
	}
	for i := range lib.Movies {
		if m := &lib.Movies[i]; m.onAir() > 0 {
			p.movies = append(p.movies, m)
		}
	}
	slices.SortFunc(p.movies, func(a, b *Movie) int { return byName(a.Name, a.ID, b.Name, b.ID) })
	for _, m := range p.movies {
		p.movieByID[m.ID] = m
	}
	for _, s := range lib.Series {
		eps := make([]Episode, 0, len(s.Episodes))
		for _, e := range s.Episodes {
			if e.onAir() > 0 {
				eps = append(eps, e)
			}
		}
		if len(eps) > 0 {
			s.Episodes = eps
			p.series = append(p.series, &s)
		}
	}
	slices.SortFunc(p.series, func(a, b *Series) int { return byName(a.Name, a.ID, b.Name, b.ID) })
	for _, s := range p.series {
		p.seriesByID[s.ID] = s
		for i, e := range s.Episodes {
			p.episodeAt[e.ID] = episodeRef{s, i}
		}
	}
	for _, c := range cursors {
		p.cursors[c.SeriesID] = c
	}
	for _, a := range history {
		if a.Kind == KindProgram {
			p.record(a)
		}
	}
	p.assignSeries()
	p.groups = sagaGroups(p.movies)
	return p
}

func byName(nameA, idA, nameB, idB string) int {
	if c := strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)); c != 0 {
		return c
	}
	return strings.Compare(idA, idB)
}

// record notes a program as aired, for the rotation rules and the strip cursors.
func (p *planner) record(a Airing) {
	if a.Kind != KindProgram {
		return
	}
	key := a.EpisodeID
	if key == "" {
		key = a.TitleID
	}
	latest(p.lastAired, key, a.StartsAt)
	switch a.Fill {
	case FillMovie, FillPremiere, FillSaga:
		latest(p.lastPrime, a.TitleID, a.StartsAt)
		if p.day != nil && a.EpisodeID == "" && !a.StartsAt.Before(p.day.start) {
			p.day.prime = p.movieByID[a.TitleID]
		}
	}
	if a.Fill == FillSaga {
		latest(p.lastSaga, a.TitleID, a.StartsAt)
		if a.StartsAt.After(p.sagaMark.at) {
			p.sagaMark = mark{a.StartsAt, a.TitleID}
		}
	}
	if (a.Fill == FillStrip || a.Fill == FillMarathon) && a.EpisodeID != "" {
		if m, ok := p.strips[a.TitleID]; !ok || a.StartsAt.After(m.at) {
			p.strips[a.TitleID] = mark{a.StartsAt, a.EpisodeID}
		}
	}
	if a.EndsAt.After(p.prevEnd) {
		p.prevEnd = a.EndsAt
	}
}

func latest(m map[string]time.Time, key string, at time.Time) {
	if at.After(m[key]) {
		m[key] = at
	}
}

// ready reports whether a program's media is prepared; media the library no longer
// offers never is.
func (p *planner) ready(a Airing) bool {
	if a.EpisodeID != "" {
		ref, ok := p.episodeAt[a.EpisodeID]
		return ok && ref.series.Episodes[ref.index].Ready
	}
	m := p.movieByID[a.TitleID]
	return m != nil && m.Ready
}

// readinessAt is how a free choice weighs preparation for a program starting at start:
// close to air a repeat beats an unprepared program; further out preparation still has
// time, so the rotation rules come first.
func (p *planner) readinessAt(start time.Time) readiness {
	switch {
	case p.swapping:
		return onlyReady
	case start.Before(p.now.Add(swapAhead)):
		return readyFirst
	case start.Before(p.now.Add(prepareAhead)):
		return preferReady
	}
	return anyMedia
}

// daySlot is a template slot placed on a date.
type daySlot struct {
	CouchTvSlot
	at time.Time // zero for a flow slot without a time
	// windowEnd is the next timed slot's time, or the day's end; softEnd says it is a
	// flow slot's target rather than an anchor or the day's end
	windowEnd time.Time
	softEnd   bool
	// maxRoom is the longest program the slot could ever hold: from its window's start
	// to the first anchor after it
	maxRoom time.Duration
}

// wants is how many programs the slot airs; reruns fill their window instead.
func (s *daySlot) wants() int {
	switch s.Fill {
	case FillStrip, FillMarathon:
		return max(1, s.Count)
	case FillReruns:
		return math.MaxInt
	}
	return 1
}

type dayState struct {
	date       time.Time
	start, end time.Time
	slots      []daySlot
	fixed      []Airing
	nextFixed  int
	fixedIn    map[string]int  // fixed programs per slot, which count toward what it airs
	removed    map[string]bool // movies and episodes an admin took off this day
	prime      *Movie          // tonight's prime-time movie, for the encore
	turns      map[string]int  // where each reruns rotation stands
	held       map[string]bool // movies kept back for the premiere or the saga
	rng        *rand.Rand
}

func (p *planner) newDay(date time.Time, fixed, removed []Airing) *dayState {
	slots := p.cfg.Template.slotsOn(date.Weekday())
	d := &dayState{
		date:    date,
		start:   p.cfg.dayStart(date),
		end:     p.cfg.dayStart(nextDate(date)),
		fixed:   fixed,
		fixedIn: map[string]int{},
		removed: map[string]bool{},
		turns:   map[string]int{},
		held:    map[string]bool{},
		rng:     daySeed(date),
	}
	for _, f := range fixed {
		if f.Slot != "" {
			d.fixedIn[f.Slot]++
		}
	}
	for _, r := range removed {
		d.removed[programKey(r)] = true
	}
	d.slots = make([]daySlot, len(slots))
	for i, s := range slots {
		d.slots[i].CouchTvSlot = s
		if s.Start != "" {
			d.slots[i].at = p.cfg.slotTime(date, s.Start)
		}
	}
	windowStart := d.start
	for i := range d.slots {
		s := &d.slots[i]
		s.windowEnd = d.end
		for _, n := range d.slots[i+1:] {
			if !n.at.IsZero() {
				s.windowEnd, s.softEnd = n.at, !n.Anchor
				break
			}
		}
		if !s.at.IsZero() {
			windowStart = s.at
		}
		hard := d.end
		for _, n := range d.slots[i+1:] {
			if n.Anchor {
				hard = n.at
				break
			}
		}
		s.maxRoom = hard.Sub(windowStart)
	}
	if p.templateHas(FillPremiere) {
		if m := p.premiereCandidate(d.start, p.movies, false); m != nil {
			d.held[m.ID] = true
		}
	}
	if p.templateHas(FillSaga) {
		if m := p.nextSagaFilm(nil); m != nil {
			d.held[m.ID] = true
		}
	}
	return d
}

// daySeed makes a day's choices depend on its date alone.
func daySeed(date time.Time) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(date.Format(time.DateOnly)))
	seed := h.Sum64()
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

func (p *planner) templateHas(fill string) bool {
	return slices.ContainsFunc(p.cfg.Template.Slots, func(s CouchTvSlot) bool { return s.Fill == fill })
}

// programKey is what a removal takes off a day: the episode, or the movie.
func programKey(a Airing) string {
	if a.EpisodeID != "" {
		return a.EpisodeID
	}
	return a.TitleID
}

// removed reports whether an admin took a movie or an episode (by id) off today.
func (p *planner) removed(id string) bool {
	return p.day != nil && p.day.removed[id]
}

// filled reports whether the fixed programs carrying a slot already air all it would.
func (d *dayState) filled(s *daySlot) bool {
	return d.fixedIn[s.Key] >= s.wants()
}

// planDay walks the day's slots in order. Anchors start on time, flow programs start on
// the next 5-minute mark after the previous program and its break, and fixed programs
// are obstacles the flow fits around.
func (p *planner) planDay(date time.Time, fixed, removed []Airing) {
	p.day = p.newDay(date, fixed, removed)
	for i := range p.day.slots {
		p.runSlot(i)
	}
	p.fillUntil(p.day.end)
	p.day = nil
}

func (p *planner) runSlot(i int) {
	d := p.day
	s := &d.slots[i]
	if d.filled(s) {
		return
	}
	// the slot's fixed programs count toward it wherever they are in the day, so a
	// locked strip episode leaves the strip's other episodes in place
	for placed := 0; placed+d.fixedIn[s.Key] < s.wants(); {
		start := p.startFor(s, placed)
		if !start.Before(s.windowEnd) {
			return
		}
		if ob := p.obstacle(); ob != nil && !ob.StartsAt.After(start) {
			p.emitFixed()
			continue
		}
		if s.Anchor && placed == 0 && start.Equal(s.at) {
			p.fillUntil(s.at)
			if !p.startFor(s, placed).Equal(start) {
				continue
			}
		}
		limit, blocked := p.limit(i, start)
		pl, ok := p.pick(s, start, limit)
		if !ok {
			if !blocked {
				// overrun: the rest of the block does not fit before the next anchor
				return
			}
			p.fillUntil(limit)
			p.emitFixed()
			continue
		}
		p.place(pl, start, s.Key)
		placed++
	}
}

// startFor is when the slot's next program can start: its anchor for the first one
// when the previous program ends in time, else the next flow mark.
func (p *planner) startFor(s *daySlot, placed int) time.Time {
	if s.Anchor && placed == 0 && !s.at.Before(p.prevEnd.Add(p.minBreak)) && !s.at.Before(p.day.start) {
		return s.at
	}
	return p.free()
}

// free is the first flow mark after the previous program and the shortest break.
func (p *planner) free() time.Time {
	t := roundUp(p.prevEnd.Add(p.minBreak), flowStep)
	if t.Before(p.day.start) {
		return p.day.start
	}
	return t
}

func roundUp(t time.Time, step time.Duration) time.Time {
	r := t.Truncate(step)
	if r.Before(t) {
		r = r.Add(step)
	}
	return r
}

// limit is the next fixed point a program starting at start must end before (with a
// break): the next anchor, the next fixed program or the day's end. blocked says the
// fixed point is a fixed program, which the slot may continue after.
func (p *planner) limit(i int, start time.Time) (limit time.Time, blocked bool) {
	d := p.day
	limit = d.end
	for j := i + 1; j < len(d.slots); j++ {
		if n := &d.slots[j]; n.Anchor && n.at.After(start) && !d.filled(n) {
			limit = n.at
			break
		}
	}
	if ob := p.obstacle(); ob != nil && ob.StartsAt.Before(limit) {
		return ob.StartsAt, true
	}
	return limit, false
}

func (p *planner) fits(m Media, start, limit time.Time) bool {
	return !start.Add(m.onAir() + p.minBreak).After(limit)
}

// fillUntil fills the gap up to until (and around the fixed programs before it) with
// reruns while one fits and the gap is at least fillerMin; what is left grows the break.
func (p *planner) fillUntil(until time.Time) {
	for {
		end := until
		ob := p.obstacle()
		if ob != nil && ob.StartsAt.Before(end) {
			end = ob.StartsAt
		}
		start := p.free()
		if end.Sub(start) >= p.fillerMin {
			if pl, ok := p.pickFiller(start, end); ok {
				p.place(pl, start, "")
				continue
			}
		}
		if ob == nil || !ob.StartsAt.Before(until) {
			return
		}
		p.emitFixed()
	}
}

func (p *planner) obstacle() *Airing {
	if d := p.day; d.nextFixed < len(d.fixed) {
		return &d.fixed[d.nextFixed]
	}
	return nil
}

func (p *planner) emitFixed() {
	a := *p.obstacle()
	p.day.nextFixed++
	p.out = append(p.out, a)
	p.record(a)
}

// placement is a chosen program.
type placement struct {
	titleID   string
	episodeID string
	media     Media
	fill      string
}

func (p *planner) place(pl placement, start time.Time, slot string) {
	a := Airing{
		StartsAt:    start,
		EndsAt:      start.Add(pl.media.onAir()),
		Kind:        KindProgram,
		TitleID:     pl.titleID,
		EpisodeID:   pl.episodeID,
		MediaFileID: pl.media.FileID,
		Slot:        slot,
		Fill:        pl.fill,
		Source:      SourceAuto,
		State:       StatePlanned,
		CreditsAt:   pl.media.CreditsAt,
	}
	p.out = append(p.out, a)
	p.record(a)
}

// withBreaks puts a break row into every gap between consecutive programs, starting
// from the end of the program before them (when it ended within a day of them).
func withBreaks(programs []Airing, prevEnd time.Time) []Airing {
	slices.SortStableFunc(programs, func(a, b Airing) int { return a.StartsAt.Compare(b.StartsAt) })
	out := make([]Airing, 0, 2*len(programs))
	end := prevEnd
	if len(programs) > 0 && end.Before(programs[0].StartsAt.Add(-24*time.Hour)) {
		end = time.Time{}
	}
	for _, a := range programs {
		if !end.IsZero() && a.StartsAt.After(end) {
			out = append(out, Airing{StartsAt: end, EndsAt: a.StartsAt, Kind: KindBreak, Source: SourceAuto, State: StatePlanned})
		}
		out = append(out, a)
		if a.EndsAt.After(end) {
			end = a.EndsAt
		}
	}
	return out
}

// civil is t's date in the channel's zone, at midnight.
func (p *planner) civil(t time.Time) time.Time {
	return civilIn(t, p.loc)
}

func civilIn(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func nextDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, date.Location())
}

// slotTime places a template time on a broadcast day: times before the day's first
// anchor are in the night after it.
func (c CouchTvConfig) slotTime(date time.Time, hhmm string) time.Time {
	loc := c.location()
	slots := c.Template.slotsOn(date.Weekday())
	first, _ := clockMinutes(slots[0].Start)
	m, _ := clockMinutes(hhmm)
	day := date.Day()
	if m < first {
		day++
	}
	return time.Date(date.Year(), date.Month(), day, m/60, m%60, 0, 0, loc)
}

// dayStart is when a broadcast day begins: its first anchor.
func (c CouchTvConfig) dayStart(date time.Time) time.Time {
	date = civilIn(date, c.location())
	return c.slotTime(date, c.Template.slotsOn(date.Weekday())[0].Start)
}

// dayOf is the broadcast day t falls in, as its date.
func (c CouchTvConfig) dayOf(t time.Time) time.Time {
	date := civilIn(t, c.location())
	if t.Before(c.dayStart(date)) {
		return time.Date(date.Year(), date.Month(), date.Day()-1, 0, 0, 0, 0, date.Location())
	}
	return date
}
