package couchtv

import (
	"math"
	"path/filepath"
	"strconv"
	"time"
)

// Bulletin slots (plan 6.10, D36-D38). A break that carries a bulletin keeps a slot for it: a
// fixed length of whole frames after the promos and the lineup, before up next. Breaks are
// composed a day ahead and rendered at night, while a bulletin is good for hours, so the slot
// binds late: the break is composed and assembled with a fallback of exactly the slot's length
// (promos and stings, renderable ahead like any part), and the live stream airs the newest
// rendered bulletin in its place when it puts the break on air, if one is still valid then.

// A bulletin's slot is as long as its clip: a length of its kind, whole frames.
const (
	weatherFrames = 375 // 15 s
	newsFrames    = 500 // 20 s
)

// bulletinFrames is the length of a kind's slot and clip.
var bulletinFrames = map[string]int{BulletinWeather: weatherFrames, BulletinNews: newsFrames}

// Slot is a break's slot for a bulletin of a kind, in frames into the break.
type Slot struct {
	Kind   string
	At     int
	Frames int
}

// slotRestFrames is the least a break keeps beside its slots: the squeeze, the short ident, a
// short promo and the shortest up next. A shorter break carries no bulletin.
var slotRestFrames = squeezeIdentFrames + identShortFrames + shortPromoFrames + upNextSpan.min

// keptSlots are the bulletins a break has room for, in order, and the frames they take:
// the later ones go first when it is too short for all of them.
func keptSlots(in BreakInput) ([]string, int) {
	if in.Next == nil {
		return nil, 0
	}
	kinds := in.Bulletins
	for len(kinds) > 0 {
		total := 0
		for _, k := range kinds {
			total += bulletinFrames[k]
		}
		if in.Frames-total >= slotRestFrames {
			return kinds, total
		}
		kinds = kinds[:len(kinds)-1]
	}
	return nil, 0
}

// slotPieces are the fallbacks of the slots: for each, promos of the titles the break does not
// promote yet (those it does when none is left), the best first, closed with stings to the
// slot's length.
func slotPieces(in BreakInput, kinds []string, groups [][]piece) []piece {
	used := map[string]bool{}
	for _, g := range groups {
		for _, p := range g {
			if p.part.TitleID != "" {
				used[p.part.TitleID] = true
			}
		}
	}
	ranked := rankCandidates(in)
	var out []piece
	for _, kind := range kinds {
		fallback := slotFallback(ranked, used, bulletinFrames[kind])
		for i := range fallback {
			fallback[i].part.Slot = kind
		}
		out = append(out, fallback...)
	}
	return out
}

// slotFallback fills frames with promos (each title once) and stings. A promo must leave either
// nothing or room for a sting, so a bigger promo that would leave a sliver gives way to a
// smaller one.
func slotFallback(ranked []Candidate, used map[string]bool, frames int) []piece {
	var out []piece
	left := frames
	taken := map[string]bool{}
	// the titles the break does not promote yet first, then the others
	for _, again := range []bool{false, true} {
		for _, c := range ranked {
			if left < shortPromoFrames {
				break
			}
			if used[c.TitleID] != again || taken[c.TitleID] {
				continue
			}
			grp, n := promoPieces(c, left)
			if n > 0 && left-n > 0 && left-n < stingSpan.min {
				grp, n = promoPieces(c, left-stingSpan.min)
			}
			if n == 0 {
				continue
			}
			out = append(out, grp...)
			left -= n
			taken[c.TitleID] = true
		}
	}
	for id := range taken {
		used[id] = true
	}
	if left > 0 {
		sizes, _, ok := fit(nil, left)
		if !ok {
			// no promo and under a sting's shortest: cannot happen for a slot of 2 s or more
			sizes = []int{left}
		}
		for _, n := range sizes {
			out = append(out, fixed(Part{Kind: ClipSting, Spec: Spec{Template: "sting", Duration: frameSeconds(n)}, Frames: n}))
		}
	}
	return out
}

// slotLoudness is a slot's loudness: the break's in whole LU, which every bulletin clip has a
// rendition of (bulletinLoudness), so a bulletin sounds as loud as the fallback it replaces.
func slotLoudness(lufs float64) float64 { return math.Round(lufs) }

// slotsOf are the slots of a composed break: its runs of parts holding one.
func slotsOf(parts []Part) []Slot {
	var out []Slot
	for _, p := range parts {
		switch {
		case p.Slot == "":
		case len(out) > 0 && out[len(out)-1].Kind == p.Slot && out[len(out)-1].At+out[len(out)-1].Frames == p.At:
			out[len(out)-1].Frames += p.Frames
		default:
			out = append(out, Slot{Kind: p.Slot, At: p.At, Frames: p.Frames})
		}
	}
	return out
}

// section is a stretch of a break with bulletin slots that is assembled as a package of its
// own (couchtv/breaks/<id>/<index>): a slot's fallback, or the clips around the slots.
type section struct {
	// from and to are where it starts and ends in the break, in frames
	from, to int
	// slot is the slot it holds, nil around them
	slot *RecipeSlot
}

// sectionsOf cuts a break at the edges of its slots, in order.
func sectionsOf(r Recipe) []section {
	var out []section
	at := 0
	for i := range r.Slots {
		s := &r.Slots[i]
		if s.At > at {
			out = append(out, section{from: at, to: s.At})
		}
		out = append(out, section{from: s.At, to: s.At + s.Frames, slot: s})
		at = s.At + s.Frames
	}
	if at < r.Frames {
		out = append(out, section{from: at, to: r.Frames})
	}
	return out
}

// sectionPath is where a break's section is assembled, under the break's own path.
func sectionPath(path string, i int) string { return filepath.Join(path, strconv.Itoa(i)) }

// When the breaks carry bulletins (plan 6.10, D37): the news in the breaks that end on the
// hour every two hours from 06:00 to midnight (the owner's choice over every hour), the weather
// in the first break that ends after 07:00, 12:00 and 17:00 (within the hour) and in the break
// before the evening movie. A slot is only made where a bulletin fetched on the grid
// (nextBulletin) in time to be rendered is still valid when the break ends.
var (
	newsFirstHour  = 6
	newsEvery      = 2
	weatherAt      = []int{7, 12, 17}
	weatherWithin  = time.Hour
	primeFrom      = 19
	primeTo        = 21
	bulletinLead   = 15 * time.Minute
	slotKindsOrder = []string{BulletinNews, BulletinWeather}
)

// bulletinKinds are the bulletins the break [start, end) carries, after a program that began at
// prevStart (zero for none) and before one of the fill nextFill.
func bulletinKinds(start, end, prevStart time.Time, nextFill string, loc *time.Location) []string {
	l := end.In(loc)
	want := map[string]bool{}
	hour := l.Hour()
	if hour == 0 {
		hour = 24
	}
	if l.Minute() == 0 && l.Second() == 0 && l.Nanosecond() == 0 && hour >= newsFirstHour && (hour-newsFirstHour)%newsEvery == 0 {
		want[BulletinNews] = true
	}
	for _, h := range weatherAt {
		target := time.Date(l.Year(), l.Month(), l.Day(), h, 0, 0, 0, loc)
		if !end.Before(target) && end.Sub(target) < weatherWithin && (prevStart.IsZero() || prevStart.Before(target)) {
			want[BulletinWeather] = true
		}
	}
	if primeTime(nextFill) && l.Hour() >= primeFrom && l.Hour() < primeTo {
		want[BulletinWeather] = true
	}
	var out []string
	for _, k := range slotKindsOrder {
		if want[k] && coverable(k, start, end, loc) {
			out = append(out, k)
		}
	}
	return out
}

// coverable says whether a bulletin of a kind fetched on the grid at least bulletinLead before
// the break starts is still valid when it ends: news after the night's pause in fetching is
// not, and a forecast is never valid past the midnight after it was fetched.
func coverable(kind string, start, end time.Time, loc *time.Location) bool {
	fetched := lastBulletin(start.Add(-bulletinLead), loc, bulletinEvery[kind])
	until := fetched.Add(bulletinValid[kind])
	if kind == BulletinWeather {
		until = weatherUntil(fetched, loc)
	}
	return !end.After(until)
}

// lastBulletin is the latest time on the fetch grid of every (nextBulletin's) at or before at.
func lastBulletin(at time.Time, loc *time.Location, every time.Duration) time.Time {
	l := at.In(loc)
	for d := 0; ; d-- {
		open := time.Date(l.Year(), l.Month(), l.Day()+d, awakeHour, awakeMinute, 0, 0, loc)
		if at.Before(open) {
			continue
		}
		if last := open.Add(awakeFor / every * every); at.After(last) {
			return last
		}
		return open.Add(at.Sub(open) / every * every)
	}
}
