package couchtv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// The compose job (plan 6.5) keeps the breaks of the next composeAhead composed: their
// recipes stored, their clips requested from the render sidecar, renders queued in the
// order they air. It also keeps an evergreen break and the standby card rendered, so the
// live stream always has something to air.
const (
	// composeAhead reaches from one night's quiet hours to the next one's: everything
	// that airs before them is composed when they start
	composeAhead = 24 * time.Hour
	composeEvery = time.Hour
	// A render waits for the quiet hours [quietFrom, quietTo) in the channel's zone, so
	// a small server renders at night rather than while people watch, unless what it is
	// for airs within renderSoon.
	quietFrom  = 1
	quietTo    = 9
	renderSoon = 3 * time.Hour
	// promoHorizon is how far after a break its promos look for airings.
	promoHorizon = 7 * 24 * time.Hour
	// recentBreaks is how many breaks before one its promos avoid repeating.
	recentBreaks = 3
	// evergreenPromos is how many promos the evergreen break carries.
	evergreenPromos = 3
)

// ComposePayload names an hourly run by its hour, so queueing the next one is idempotent.
type ComposePayload struct {
	At string `json:"at,omitempty"`
}

// Composer is the couchtv_compose job.
type Composer struct {
	Store    *Store
	Settings *settings.Store
	Jobs     *jobs.Store
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time
}

func (c *Composer) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Start queues a run now; each run queues the next one an hour later.
func (c *Composer) Start(ctx context.Context) error {
	_, err := c.Jobs.EnqueueJobOnce(ctx, ComposeJob, ComposePayload{}, jobs.EnqueueOpts{})
	return err
}

// Handle composes what is missing. Nothing is composed before a render sidecar has run
// once: the clip keys need its templates' versions, and nothing could render them anyway.
func (c *Composer) Handle(ctx context.Context, _ *jobs.Job, report func(int)) error {
	now := c.now()
	next := now.Truncate(composeEvery).Add(composeEvery)
	if _, err := c.Jobs.EnqueueJobOnce(ctx, ComposeJob, ComposePayload{At: next.UTC().Format(time.RFC3339)},
		jobs.EnqueueOpts{RunAt: next}); err != nil {
		return err
	}
	renderer, ok := loadRendererState(ctx, c.Settings)
	if !ok {
		slog.Info("couchtv: no render sidecar has published its templates yet, breaks wait for it")
		return nil
	}
	run := &composition{Composer: c, versions: renderer.Versions, cfg: LoadConfig(ctx, c.Settings), now: now, ready: map[string]int{}}
	if err := run.load(ctx); err != nil {
		return err
	}
	report(20)
	if err := run.fallbacks(ctx); err != nil {
		return err
	}
	breaks, err := c.Store.uncomposed(ctx, now, now.Add(composeAhead))
	if err != nil {
		return err
	}
	// a changed template reaches the breaks composed before it
	outdated, err := c.Store.outdated(ctx, now, now.Add(composeAhead), renderer.Versions)
	if err != nil {
		return err
	}
	for _, b := range outdated {
		run.forget(b.StartsAt)
	}
	breaks = append(breaks, outdated...)
	slices.SortStableFunc(breaks, func(a, b Airing) int { return a.StartsAt.Compare(b.StartsAt) })
	for i, b := range breaks {
		if err := run.compose(ctx, b); err != nil {
			return err
		}
		report(20 + 80*(i+1)/len(breaks))
	}
	if len(breaks) > 0 {
		slog.Info("couchtv: composed breaks", "count", len(breaks), "outdated", len(outdated))
	}
	return run.dropUnused(ctx)
}

// dropUnused cancels the renders of clips no composition uses any more: a replan moved
// their break, or it was composed again.
func (r *composition) dropUnused(ctx context.Context) error {
	ids, err := r.Store.unusedClips(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.Jobs.CancelPending(ctx, RenderJob, RenderPayload{ClipID: id}); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		slog.Info("couchtv: dropped renders nothing uses", "count", len(ids))
	}
	return nil
}

// renderAt is when the render of a clip for what airs at airs may start: at once for
// what airs any time or soon, else in the quiet hours, but no later than renderSoon
// before it airs.
func renderAt(now, airs time.Time, loc *time.Location) time.Time {
	if airs.IsZero() || airs.Sub(now) <= renderSoon {
		return now
	}
	t := now.In(loc)
	quiet := time.Date(t.Year(), t.Month(), t.Day(), quietFrom, 0, 0, 0, loc)
	switch {
	case t.Hour() >= quietFrom && t.Hour() < quietTo:
		return now
	case !t.Before(quiet):
		quiet = quiet.AddDate(0, 0, 1)
	}
	if latest := airs.Add(-renderSoon); latest.Before(quiet) {
		quiet = latest
	}
	if quiet.Before(now) {
		return now
	}
	return quiet
}

// composition is one run of the compose job: what it read and what it composed so far.
type composition struct {
	*Composer
	versions map[string]int
	cfg      CouchTvConfig
	now      time.Time
	programs []program
	titles   map[string]titleFacts
	// promoted are the titles each break composed or read promotes, by its start
	promoted []promotion
	// ready caches promo body keys to their rendered length (0: not rendered)
	ready map[string]int
}

type promotion struct {
	at     time.Time
	titles []string
}

// forget drops what the break at `at` promoted, before it is composed again.
func (r *composition) forget(at time.Time) {
	r.promoted = slices.DeleteFunc(r.promoted, func(p promotion) bool { return p.at.Equal(at) })
}

func (r *composition) load(ctx context.Context) error {
	var err error
	if r.programs, err = r.Store.programs(ctx, r.now.Add(-24*time.Hour), r.now.Add(composeAhead+promoHorizon)); err != nil {
		return err
	}
	var ids []string
	for _, p := range r.programs {
		if !slices.Contains(ids, p.TitleID) {
			ids = append(ids, p.TitleID)
		}
	}
	if r.titles, err = r.Store.titleFacts(ctx, ids); err != nil {
		return err
	}
	r.promoted, err = r.Store.promotions(ctx, r.now.Add(-6*time.Hour), r.now.Add(composeAhead))
	return err
}

// compose composes one break.
func (r *composition) compose(ctx context.Context, b Airing) error {
	in, err := r.input(ctx, b)
	if err != nil {
		return err
	}
	comp, err := Compose(in)
	if err != nil {
		slog.Warn("couchtv: a break cannot be composed", "airingId", b.ID, "frames", in.Frames, "err", err)
		_, err := r.Store.saveBreak(ctx, b.ID, Recipe{Frames: in.Frames}, BreakFailed, err.Error())
		return err
	}
	if err := r.requestBodies(ctx, in.Candidates); err != nil {
		return err
	}
	_, err = r.save(ctx, b.ID, b.StartsAt, comp)
	if err == nil {
		at, _ := slices.BinarySearchFunc(r.promoted, b.StartsAt, func(p promotion, t time.Time) int { return p.at.Compare(t) })
		r.promoted = slices.Insert(r.promoted, at, promotion{at: b.StartsAt, titles: comp.Promos})
	}
	return err
}

// save requests a composition's clips and stores it; it queues its assembly at once when
// every clip is rendered already. An invalid spec fails the break, not the run.
func (r *composition) save(ctx context.Context, airingID string, airs time.Time, comp Composition) (string, error) {
	recipe := Recipe{Frames: comp.Frames, Sound: comp.Sound, Promos: comp.Promos}
	if recipe.Promos == nil {
		recipe.Promos = []string{}
	}
	invalid := validateSpec(comp.Sound)
	keys := make([]string, len(comp.Parts))
	for i, p := range comp.Parts {
		if err := validateSpec(p.Spec); err != nil && invalid == nil {
			invalid = fmt.Errorf("%s: %w", p.Kind, err)
		}
		key, err := clipKey(r.versions[p.Spec.Template], p.Spec)
		if err != nil {
			return "", err
		}
		keys[i] = key
	}
	if invalid != nil {
		slog.Error("couchtv: a composed spec is invalid", "airingId", airingID, "err", invalid)
		return r.Store.saveBreak(ctx, airingID, recipe, BreakFailed, invalid.Error())
	}
	sum := sha256.Sum256([]byte(strings.Join(keys, ",")))
	recipe.Key = hex.EncodeToString(sum[:16])
	ready := true
	for i, p := range comp.Parts {
		needed := airs.Add(time.Duration(p.At) * time.Second / FPS)
		if airs.IsZero() {
			needed = r.now
		}
		id, state, err := r.request(ctx, p.Kind, keys[i], p.Spec, p.Frames, needed, airs)
		if err != nil {
			return "", err
		}
		ready = ready && state == ClipReady
		recipe.Clips = append(recipe.Clips, RecipeClip{ClipID: id, Kind: p.Kind, At: p.At, Frames: p.Frames})
	}
	id, err := r.Store.saveBreak(ctx, airingID, recipe, BreakComposed, "")
	if err != nil || !ready {
		return id, err
	}
	return id, queueAssembly(ctx, r.Jobs, Break{ID: id, StartsAt: airs})
}

// request asks for a clip and queues its render unless it is rendered: soonest-airing
// first, what airs any time (airs zero) before all.
func (r *composition) request(ctx context.Context, kind, key string, spec Spec, frames int, needed, airs time.Time) (string, string, error) {
	id, state, err := r.Store.RequestClip(ctx, kind, key, spec, frames, needed)
	if err != nil || state == ClipReady {
		return id, state, err
	}
	if state == ClipFailed {
		if err := r.Store.RetryClip(ctx, id); err != nil {
			return "", "", err
		}
	}
	payload, opts := RenderPayload{ClipID: id}, jobs.EnqueueOpts{Priority: urgency(airs), RunAt: renderAt(r.now, airs, r.cfg.location())}
	if _, err := r.Jobs.EnqueueJobOnce(ctx, RenderJob, payload, opts); err != nil {
		return "", "", err
	}
	// a clip another break asked for later is needed sooner now
	return id, state, r.Jobs.Expedite(ctx, RenderJob, payload, opts.RunAt, opts.Priority)
}

// requestBodies asks for the promo bodies the candidates do not have yet, so later breaks
// can air full promos; they render after the breaks that need clips sooner.
func (r *composition) requestBodies(ctx context.Context, cands []Candidate) error {
	for _, c := range cands {
		if c.BodyFrames > 0 || c.Strip != nil {
			continue
		}
		body := promoBody(c)
		key, err := clipKey(r.versions["promo"], body)
		if err != nil {
			return err
		}
		if _, _, err := r.request(ctx, ClipPromoBody, key, body, 0, c.Airs, c.Airs); err != nil {
			return err
		}
	}
	return nil
}

// fallbacks keeps the standby card and an evergreen break of the current templates, and
// promo bodies rendered for it.
func (r *composition) fallbacks(ctx context.Context) error {
	standby := Spec{Template: "standby", Duration: frameSeconds(standbyFrames)}
	key, err := clipKey(r.versions["standby"], standby)
	if err != nil {
		return err
	}
	if _, _, err := r.request(ctx, ClipStandby, key, standby, standbyFrames, r.now, time.Time{}); err != nil {
		return err
	}

	var cands, bodies []Candidate
	for _, id := range r.sortedTitles() {
		cands = append(cands, r.candidate(id, program{}, r.now))
	}
	if err := r.bodies(ctx, cands); err != nil {
		return err
	}
	for _, c := range cands {
		if len(bodies) < evergreenPromos && c.BodyFrames > 0 {
			bodies = append(bodies, c)
		}
	}
	comp := ComposeEvergreen(bodies)
	var keys []string
	for _, p := range comp.Parts {
		k, err := clipKey(r.versions[p.Spec.Template], p.Spec)
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}
	sum := sha256.Sum256([]byte(strings.Join(keys, ",")))
	exists, err := r.Store.evergreenComposed(ctx, hex.EncodeToString(sum[:16]))
	if err != nil || exists {
		return err
	}
	_, err = r.save(ctx, "", time.Time{}, comp)
	return err
}

// sortedTitles are the titles that air in the window, by name.
func (r *composition) sortedTitles() []string {
	var ids []string
	for id := range r.titles {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int { return strings.Compare(r.titles[a].Name+a, r.titles[b].Name+b) })
	return ids
}

// input is everything a break's composition depends on.
func (r *composition) input(ctx context.Context, b Airing) (BreakInput, error) {
	loc := r.cfg.location()
	in := BreakInput{Start: b.StartsAt, Frames: framesOf((b.EndsAt.Sub(b.StartsAt)).Seconds()), Loc: loc, Recent: map[string]bool{}}
	var ending *program
	for i := range r.programs {
		p := &r.programs[i]
		switch {
		case p.EndsAt.Equal(b.StartsAt):
			ending = p
		case !p.StartsAt.Before(b.EndsAt) && in.Next == nil:
			in.Next = r.breakProgram(*p, loc)
		case !p.StartsAt.Before(b.EndsAt) && len(in.Later) < lineupRowsMax-1:
			in.Later = append(in.Later, *r.breakProgram(*p, loc))
		}
	}
	if ending != nil {
		in.Ending = r.breakProgram(*ending, loc)
		if ending.CreditsAt > 0 {
			// the squeeze picks the program up where it left the air
			offset := b.StartsAt.Sub(ending.StartsAt)
			in.Ending.Frames = FramesURLs(ending.MediaFileID, offset, squeezeExpandFrames)
			length := time.Duration(squeezeExpandFrames) * time.Second / FPS
			in.Ending.Bed = &Bed{URL: AudioURL(ending.MediaFileID, offset, length), Out: length.Seconds()}
		}
		in.Loudness = r.titles[ending.TitleID].Loudness
	}
	if in.Loudness == 0 && in.Next != nil {
		in.Loudness = r.titles[in.Next.TitleID].Loudness
	}

	// each title once, at its first airing after the break
	seen := map[string]bool{}
	for _, p := range r.programs {
		if p.StartsAt.Before(b.EndsAt) || p.StartsAt.After(b.EndsAt.Add(promoHorizon)) || seen[p.TitleID] {
			continue
		}
		seen[p.TitleID] = true
		in.Candidates = append(in.Candidates, r.candidate(p.TitleID, p, b.StartsAt))
	}
	if err := r.bodies(ctx, in.Candidates); err != nil {
		return in, err
	}

	n := 0
	for i := len(r.promoted) - 1; i >= 0 && n < recentBreaks; i-- {
		if r.promoted[i].at.Before(b.StartsAt) {
			for _, id := range r.promoted[i].titles {
				in.Recent[id] = true
			}
			n++
		}
	}
	return in, nil
}

// candidate is a title as a promo shows it, at an airing (zero for the evergreen break).
func (r *composition) candidate(id string, p program, from time.Time) Candidate {
	t := r.titles[id]
	c := Candidate{TitleID: id, Title: t.spec(), Airs: p.StartsAt, Fill: p.Fill, Mood: t.mood(), StripMood: t.stripMood(),
		Shots: t.shots(), Bed: t.bed()}
	if !p.StartsAt.IsZero() {
		c.Airing = airingPhrase(p.StartsAt, from, r.cfg.location(), p.Fill)
	}
	if p.Fill == FillStrip {
		for _, s := range r.cfg.Template.Slots {
			if s.Key == p.Slot && s.Anchor && s.Start != "" {
				phrase := stripPhrase(s.Days, s.Start)
				c.Strip, c.StripCount = &phrase, max(1, s.Count)
			}
		}
	}
	return c
}

// bodies fills in the lengths of the candidates' rendered promo bodies.
func (r *composition) bodies(ctx context.Context, cands []Candidate) error {
	keys := make([]string, len(cands))
	var missing []string
	for i := range cands {
		key, err := clipKey(r.versions["promo"], promoBody(cands[i]))
		if err != nil {
			return err
		}
		keys[i] = key
		if _, ok := r.ready[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		frames, err := r.Store.ReadyFrames(ctx, missing)
		if err != nil {
			return err
		}
		for _, k := range missing {
			r.ready[k] = frames[k]
		}
	}
	for i := range cands {
		cands[i].BodyFrames = r.ready[keys[i]]
	}
	return nil
}

func (r *composition) breakProgram(p program, loc *time.Location) *BreakProgram {
	t := r.titles[p.TitleID]
	bp := &BreakProgram{TitleID: p.TitleID, Title: t.spec(), Start: p.StartsAt, Fill: p.Fill, Promo: t.bed()}
	if p.EpisodeID != "" {
		bp.Episode = &SpecEpisode{Code: fmt.Sprintf("S%02dE%02d", p.Season, p.Number), Name: p.EpisodeName, RuntimeMinutes: p.Runtime}
		bp.Sub = bp.Episode.Code
		if p.EpisodeName != "" {
			bp.Sub += " · " + p.EpisodeName
		}
	} else {
		parts := []string{"Movie"}
		if len(t.Genres) > 0 {
			parts = append(parts, t.Genres[0])
		}
		if t.Runtime > 0 {
			parts = append(parts, fmt.Sprintf("%d min", t.Runtime))
		}
		bp.Sub = strings.Join(parts, " · ")
	}
	return bp
}

// titleFacts is what the breaks show of a title and its promo material.
type titleFacts struct {
	Name     string
	Year     int
	Runtime  int
	Genres   []string
	Tagline  string
	Backdrop string // artwork URLs
	Poster   string
	Accents  Accents
	Mood     string
	Shots    []analyzedShot
	Promo    *Bed
	Loudness float64
}

// analyzedShot is one footage shot couchtv_analyze picked (couchtv_titles.shots).
type analyzedShot struct {
	MediaFileID string  `json:"mediaFileId"`
	StartMs     int64   `json:"startMs"`
	DurationMs  int64   `json:"durationMs"`
	Score       float64 `json:"score"`
}

func (t titleFacts) spec() SpecTitle {
	genres := t.Genres
	if genres == nil {
		genres = []string{}
	}
	return SpecTitle{Name: t.Name, Year: t.Year, RuntimeMinutes: t.Runtime, Genres: genres, Accents: t.Accents,
		Backdrop: t.Backdrop, Poster: t.Poster, Taglines: taglineLines(t.Tagline)}
}

// mood is the analysed promo mood, else one from the genres.
func (t titleFacts) mood() string {
	if t.Mood != "" {
		return t.Mood
	}
	// TMDB names genres in a title's base language, so the Czech names count too
	for _, g := range t.Genres {
		switch strings.ToLower(g) {
		case "comedy", "animation", "family", "komedie", "animovaný", "rodinný":
			return "bright"
		case "crime", "thriller", "horror", "krimi", "horor":
			return "grit"
		}
	}
	return "cinematic"
}

func (t titleFacts) stripMood() string {
	if t.mood() == "bright" {
		return "bright"
	}
	return "dark"
}

// shots are the analysed footage shots (D18), else stills from the artwork.
func (t titleFacts) shots() []Shot {
	var out []Shot
	for _, s := range t.Shots {
		frames := int(s.DurationMs * FPS / 1000)
		if s.MediaFileID == "" || frames <= 0 {
			continue
		}
		out = append(out, Shot{Kind: "footage", Frames: FramesURLs(s.MediaFileID, time.Duration(s.StartMs)*time.Millisecond, frames)})
	}
	if len(out) > 0 {
		return out
	}
	return []Shot{
		{Kind: "still", Image: "backdrop", From: &Crop{0, 0, 1, 1}, To: &Crop{0.3, 0.05, 0.5, 0.5}},
		{Kind: "still", Image: "poster", From: &Crop{0, 0.05, 0.7, 0.3}},
		{Kind: "still", Image: "backdrop", From: &Crop{0.5, 0.4, 0.45, 0.5}},
		{Kind: "still", Image: "poster", From: &Crop{0.1, 0.45, 0.6, 0.25}},
	}
}

func (t titleFacts) bed() *Bed {
	if t.Promo == nil {
		return nil
	}
	b := *t.Promo
	return &b
}

// taglineLines splits a tagline into the promo's two lines, at a comma or full stop near
// the middle, else at the space nearest it.
func taglineLines(tagline string) []string {
	tagline = strings.TrimSpace(tagline)
	words := strings.Fields(tagline)
	if len(words) == 0 {
		return nil
	}
	if len(words) <= 3 {
		return []string{tagline}
	}
	best, score := 1, len(tagline)
	at := 0
	for i := 1; i < len(words); i++ {
		at += len(words[i-1]) + 1
		d := at - len(tagline)/2
		if d < 0 {
			d = -d
		}
		if strings.ContainsAny(words[i-1][len(words[i-1])-1:], ",.;:!?") {
			d -= len(tagline) / 4
		}
		if d < score {
			best, score = i, d
		}
	}
	return []string{strings.Join(words[:best], " "), strings.Join(words[best:], " ")}
}

// parseShots reads couchtv_titles.shots.
func parseShots(raw []byte) []analyzedShot {
	var shots []analyzedShot
	if json.Unmarshal(raw, &shots) != nil {
		return nil
	}
	return shots
}
