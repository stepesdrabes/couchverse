package couchtv

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

var prague = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		panic(err)
	}
	return loc
}()

// day parses a date in the channel's zone; the week of 2026-10-12 starts on a Monday.
func day(s string) time.Time {
	t, err := time.ParseInLocation(time.DateOnly, s, prague)
	if err != nil {
		panic(err)
	}
	return t
}

// local is a wall-clock time in Prague.
func local(date, hhmm string) time.Time {
	t, err := time.ParseInLocation(time.DateOnly+" 15:04", date+" "+hhmm, prague)
	if err != nil {
		panic(err)
	}
	return t
}

func uid(kind, n int) string { return fmt.Sprintf("00000000-0000-4000-8%03d-%012d", kind, n) }

func minutes(m float64) time.Duration { return time.Duration(m * float64(time.Minute)) }

// testLibrary names every title and episode for readable failures.
type testLibrary struct {
	Library
	names map[string]string
}

func (l testLibrary) label(a Airing) string {
	if a.Kind == KindBreak {
		return "break"
	}
	if a.EpisodeID != "" {
		return l.names[a.TitleID] + " " + l.names[a.EpisodeID]
	}
	return l.names[a.TitleID]
}

func (l *testLibrary) movie(n int, name string, year, mins int, collection int, collectionName string, genres ...string) {
	id := uid(1, n)
	l.names[id] = name
	l.Movies = append(l.Movies, Movie{
		ID: id, Name: name, Genres: genres,
		Released:     time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC),
		AddedAt:      time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		CollectionID: collection, CollectionName: collectionName,
		Media: Media{FileID: uid(3, n), Duration: minutes(float64(mins)), Ready: true},
	})
}

// series adds a series with the given episodes per season, each about mins long.
func (l *testLibrary) series(n int, name string, seasons []int, mins float64, genres ...string) {
	id := uid(2, n)
	l.names[id] = name
	s := Series{ID: id, Name: name, Genres: genres}
	k := 0
	for season, count := range seasons {
		for e := 1; e <= count; e++ {
			k++
			eid := uid(4, n*10000+k)
			l.names[eid] = fmt.Sprintf("S%02dE%02d", season+1, e)
			// a few seconds either way, as real episodes are
			length := minutes(mins) + time.Duration(k%7-3)*10*time.Second
			s.Episodes = append(s.Episodes, Episode{ID: eid, Season: season + 1, Number: e,
				Media: Media{FileID: uid(5, n*10000+k), Duration: length, Ready: true}})
		}
	}
	l.Series = append(l.Series, s)
}

func repeat(count, total int) []int {
	var out []int
	for total > 0 {
		n := min(count, total)
		out = append(out, n)
		total -= n
	}
	return out
}

// ownerLibrary is shaped like the production library of plan section 3.
func ownerLibrary() testLibrary {
	l := testLibrary{names: map[string]string{}}
	hp := []struct {
		name string
		year int
		mins int
	}{
		{"Harry Potter a Kámen mudrců", 2001, 152}, {"Harry Potter a Tajemná komnata", 2002, 161},
		{"Harry Potter a vězeň z Azkabanu", 2004, 142}, {"Harry Potter a Ohnivý pohár", 2005, 157},
		{"Harry Potter a Fénixův řád", 2007, 138}, {"Harry Potter a Princ dvojí krve", 2009, 153},
		{"Harry Potter a Relikvie smrti - část 1", 2010, 146}, {"Harry Potter a Relikvie smrti - část 2", 2011, 130},
	}
	for i, m := range hp {
		l.movie(i+1, m.name, m.year, m.mins, 1241, "Harry Potter (kolekce)", "Adventure", "Fantasy", "Family")
	}
	l.movie(9, "Iron Man", 2008, 126, 131292, "Iron Man (kolekce)", "Action", "Science Fiction")
	l.movie(10, "Iron Man 2", 2010, 124, 131292, "Iron Man (kolekce)", "Action", "Science Fiction")
	l.movie(11, "Iron Man 3", 2013, 130, 131292, "Iron Man (kolekce)", "Action", "Science Fiction")
	l.movie(12, "Pulp Fiction", 1994, 154, 0, "", "Thriller", "Crime")
	l.series(1, "Simpsonovi", append([]int{13}, repeat(22, 785-13)...), 22, "Animation", "Comedy", "Family")
	l.series(2, "Městečko South Park", repeat(14, 321), 22, "Animation", "Comedy")
	l.series(3, "Dexter", repeat(12, 96), 53, "Crime", "Drama", "Mystery")
	l.series(4, "Věřte nevěřte", repeat(9, 45), 44, "Documentary", "Mystery")
	l.series(5, "Invincible", []int{8, 8, 8, 9}, 45, "Animation", "Action & Adventure", "Drama")
	return l
}

func (l testLibrary) id(name string) string {
	for id, n := range l.names {
		if n == name && !strings.HasPrefix(n, "S0") {
			return id
		}
	}
	panic("no title " + name)
}

// longAgo is a planning "now" far enough before the planned days that readiness
// does not matter.
var longAgo = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func planWeek(lib Library, from string, days int, opts ...func(*PlanInput)) []Airing {
	in := PlanInput{Config: DefaultConfig(), Library: lib, From: day(from), Days: days, Now: longAgo}
	for _, o := range opts {
		o(&in)
	}
	return Plan(in)
}

func programs(rows []Airing) []Airing {
	return slices.DeleteFunc(slices.Clone(rows), func(a Airing) bool { return a.Kind != KindProgram })
}

// onDay keeps the rows that start within a broadcast day.
func onDay(cfg CouchTvConfig, rows []Airing, date string) []Airing {
	start, end := cfg.dayStart(day(date)), cfg.dayStart(nextDate(day(date)))
	return slices.DeleteFunc(slices.Clone(rows), func(a Airing) bool { return a.StartsAt.Before(start) || !a.StartsAt.Before(end) })
}

func inSlot(rows []Airing, slot string) []Airing {
	return slices.DeleteFunc(programs(rows), func(a Airing) bool { return a.Slot != slot })
}

func (l testLibrary) labels(rows []Airing) []string {
	out := make([]string, len(rows))
	for i, a := range rows {
		out[i] = l.label(a)
	}
	return out
}

func clock(t time.Time) string { return t.In(prague).Format("Mon 15:04") }

// checkTimeline asserts the invariants every plan keeps: rows in order without
// overlaps, a break in every gap and no shorter than the minimum, flow programs on the
// 5-minute grid.
func checkTimeline(t *testing.T, cfg CouchTvConfig, lib testLibrary, rows []Airing) {
	t.Helper()
	minBreak := time.Duration(cfg.BreakMinSeconds) * time.Second
	for i, a := range rows {
		if !a.EndsAt.After(a.StartsAt) {
			t.Fatalf("row %d (%s) at %s has no length", i, lib.label(a), clock(a.StartsAt))
		}
		if a.Kind == KindProgram && a.Source == SourceAuto && a.StartsAt.Truncate(flowStep) != a.StartsAt {
			t.Errorf("%s starts off the 5-minute grid at %s", lib.label(a), a.StartsAt.In(prague).Format("15:04:05"))
		}
		if a.Kind == KindBreak && a.EndsAt.Sub(a.StartsAt) < minBreak {
			t.Errorf("break at %s is %s", clock(a.StartsAt), a.EndsAt.Sub(a.StartsAt))
		}
		if i == 0 {
			continue
		}
		prev := rows[i-1]
		if a.StartsAt.Before(prev.EndsAt) {
			t.Fatalf("%s at %s overlaps %s ending %s", lib.label(a), clock(a.StartsAt), lib.label(prev), clock(prev.EndsAt))
		}
		if a.StartsAt.After(prev.EndsAt) {
			t.Fatalf("gap without a break between %s ending %s and %s", lib.label(prev), clock(prev.EndsAt), lib.label(a))
		}
		if a.Kind == prev.Kind && a.Kind == KindBreak {
			t.Fatalf("two breaks in a row at %s", clock(a.StartsAt))
		}
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("the shipped defaults do not pass their own validation: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CouchTvConfig)
		wantErr string
	}{
		{"defaults", func(*CouchTvConfig) {}, ""},
		{"other zone", func(c *CouchTvConfig) { c.TimeZone = "America/New_York" }, ""},
		{"unknown zone", func(c *CouchTvConfig) { c.TimeZone = "Mars/Olympus" }, "time zone"},
		{"break too short", func(c *CouchTvConfig) { c.BreakMinSeconds = 2 }, "breakMinSeconds"},
		{"negative rotation", func(c *CouchTvConfig) { c.Rotation.RerunGapDays = -1 }, "rerunGapDays"},
		{"no filler minimum", func(c *CouchTvConfig) { c.Rotation.FillerMinMinutes = 0 }, "fillerMinMinutes"},
		{"duplicate key", func(c *CouchTvConfig) { c.Template.Slots[1].Key = "morning" }, "twice"},
		{"day must open with an anchor", func(c *CouchTvConfig) { c.Template.Slots[0].Anchor = false }, "must be an anchor"},
		{"anchor needs a time", func(c *CouchTvConfig) { c.Template.Slots[6].Start = "" }, "needs a start"},
		{"times ascend", func(c *CouchTvConfig) { c.Template.Slots[7].Start = "16:00" }, "not after"},
		{"bad time", func(c *CouchTvConfig) { c.Template.Slots[7].Start = "25:00" }, "HH:MM"},
		{"a day without slots", func(c *CouchTvConfig) {
			for i := range c.Template.Slots {
				c.Template.Slots[i].Days = slices.DeleteFunc(slices.Clone(c.Template.Slots[i].Days), func(d string) bool { return d == "wed" })
			}
		}, "wed has no slots"},
		{"reruns need an end", func(c *CouchTvConfig) {
			c.Template.Slots = slices.DeleteFunc(c.Template.Slots, func(s CouchTvSlot) bool { return s.Key == "encore" })
			c.Template.Slots = append(c.Template.Slots, CouchTvSlot{Key: "tail", Days: []string{"mon"}, Fill: FillMovie})
		}, "needs a timed slot"},
		{"a strip pins one series", func(c *CouchTvConfig) { c.Template.Slots[7].Titles = []string{uid(2, 1), uid(2, 2)} }, "pins one"},
		{"pinned ids", func(c *CouchTvConfig) { c.Template.Slots[9].Titles = []string{"iron-man"} }, "not a title id"},
		{"unknown fill", func(c *CouchTvConfig) { c.Template.Slots[9].Fill = "news" }, "unknown fill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// A stored value missing parts, or no longer valid, falls back part by part.
func TestConfigDefaultsPartByPart(t *testing.T) {
	stored := CouchTvConfig{TimeZone: "America/New_York", Template: CouchTvTemplate{Slots: []CouchTvSlot{{Key: "x", Days: []string{"mon"}, Fill: FillMovie}}}}
	got := stored.withDefaults()
	def := DefaultConfig()
	if got.TimeZone != "America/New_York" {
		t.Errorf("kept zone %q", got.TimeZone)
	}
	if got.BreakMinSeconds != def.BreakMinSeconds || got.Rotation != def.Rotation || !reflect.DeepEqual(got.Template, def.Template) {
		t.Errorf("missing or invalid parts were not defaulted: %+v", got)
	}
}

func TestBroadcastDays(t *testing.T) {
	cfg := DefaultConfig()
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{local("2026-10-13", "05:59"), "2026-10-12"},
		{local("2026-10-13", "06:00"), "2026-10-13"},
		{local("2026-10-13", "23:00"), "2026-10-13"},
	} {
		if got := cfg.dayOf(c.at).Format(time.DateOnly); got != c.want {
			t.Errorf("dayOf(%s) = %s, want %s", c.at, got, c.want)
		}
	}
	if got := cfg.slotTime(day("2026-10-12"), "01:00"); !got.Equal(local("2026-10-13", "01:00")) {
		t.Errorf("01:00 belongs to the night after: %s", got)
	}
}

// The default template, given a library like the owner's, makes plan 5.1's week.
func TestFullWeekShape(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	rows := planWeek(lib.Library, "2026-10-12", 7)
	checkTimeline(t, cfg, lib, rows)

	first, last := rows[0], rows[len(rows)-1]
	if !first.StartsAt.Equal(local("2026-10-12", "06:00")) {
		t.Errorf("the week starts at %s", clock(first.StartsAt))
	}
	if last.EndsAt.After(local("2026-10-19", "06:00")) || last.EndsAt.Before(local("2026-10-19", "05:00")) {
		t.Errorf("the week ends at %s", clock(last.EndsAt))
	}

	anchors := map[string]string{"morning": "06:00", "matinee": "12:00", "family-movie": "12:00", "early-strip": "17:00",
		"sitcom-strip": "18:00", "prime-movie": "20:15", "premiere": "20:15", "saga": "20:15"}
	for _, date := range []string{"2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18"} {
		today := onDay(cfg, rows, date)
		for slot, hhmm := range anchors {
			got := inSlot(today, slot)
			if len(got) == 0 {
				continue
			}
			if !got[0].StartsAt.Equal(local(date, hhmm)) {
				t.Errorf("%s %s starts at %s, want %s", date, slot, clock(got[0].StartsAt), hhmm)
			}
		}
		for _, a := range inSlot(today, "morning") {
			if n := lib.names[a.TitleID]; n != "Simpsonovi" && n != "Městečko South Park" {
				t.Errorf("%s morning toons air %s", date, n)
			}
		}
		for _, a := range inSlot(today, "late") {
			if n := lib.names[a.TitleID]; n == "Dexter" || n == "Věřte nevěřte" {
				t.Errorf("%s late animation block airs %s", date, n)
			}
		}
		if got := lib.labels(inSlot(today, "early-strip")); len(got) != 1 || !strings.HasPrefix(got[0], "Věřte nevěřte") {
			t.Errorf("%s 17:00 strip: %v", date, got)
		}
		prime := slices.DeleteFunc(programs(today), func(a Airing) bool {
			return a.Fill != FillMovie && a.Fill != FillPremiere && a.Fill != FillSaga
		})
		encore := inSlot(today, "encore")
		if len(prime) != 1 || len(encore) != 1 || encore[0].TitleID != prime[0].TitleID {
			t.Errorf("%s: prime %v, encore %v", date, lib.labels(prime), lib.labels(encore))
		}
	}

	// the strips advance in order, two Simpsons and two South Parks a day
	var simpsons, southPark []string
	for _, a := range inSlot(rows, "sitcom-strip") {
		simpsons = append(simpsons, lib.label(a))
	}
	for _, a := range inSlot(rows, "second-sitcom-strip") {
		southPark = append(southPark, lib.label(a))
	}
	if len(simpsons) != 14 || simpsons[0] != "Simpsonovi S01E01" || simpsons[13] != "Simpsonovi S02E01" {
		t.Errorf("Simpsons strip: %v", simpsons)
	}
	if len(southPark) != 14 || southPark[0] != "Městečko South Park S01E01" || southPark[13] != "Městečko South Park S01E14" {
		t.Errorf("South Park strip: %v", southPark)
	}

	// Dexter: the weekday strip Mon-Thu, the marathon catching up at the weekend
	var dexter []string
	for _, a := range programs(rows) {
		if (a.Fill == FillStrip || a.Fill == FillMarathon) && lib.names[a.TitleID] == "Dexter" {
			dexter = append(dexter, fmt.Sprintf("%s %s %s", a.StartsAt.In(prague).Format("Mon"), a.Fill, lib.names[a.EpisodeID]))
		}
	}
	if len(dexter) < 6 || dexter[0] != "Mon strip S01E01" || dexter[3] != "Thu strip S01E04" || !strings.HasPrefix(dexter[4], "Sat marathon S01E05") {
		t.Errorf("Dexter: %v", dexter)
	}
	for _, slot := range []string{"late-strip"} {
		for _, a := range inSlot(rows, slot) {
			if lib.names[a.TitleID] != "Invincible" {
				t.Errorf("%s airs %s", slot, lib.label(a))
			}
		}
	}
	if got := lib.labels(inSlot(rows, "saga")); len(got) != 1 || got[0] != "Harry Potter a Kámen mudrců" {
		t.Errorf("Saga Sunday: %v", got)
	}
	if got := inSlot(onDay(cfg, rows, "2026-10-17"), "late-movie"); len(got) != 1 {
		t.Errorf("Saturday late movie: %v", lib.labels(got))
	}
}

// Anchors start exactly; flow starts on the next 5-minute mark after the previous
// program and the shortest break.
func TestAnchorsAndFlow(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Short", repeat(20, 100), 21.6, "Comedy")
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillStrip, Count: 3},
		{Key: "noon", Days: dayNames, Start: "12:00", Anchor: true, Fill: FillStrip, Count: 1},
		{Key: "rest", Days: dayNames, Fill: FillReruns},
	}}
	rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) { in.Config = cfg })
	checkTimeline(t, cfg, l, rows)
	var got []string
	for _, a := range inSlot(rows, "open") {
		got = append(got, a.StartsAt.In(prague).Format("15:04"))
	}
	// episodes of 21:36 +- 30 s: 06:00, then 06:21:36 + 23 s rounds up to 06:25
	if !reflect.DeepEqual(got, []string{"06:00", "06:25", "06:50"}) {
		t.Errorf("flow starts %v", got)
	}
	if noon := inSlot(rows, "noon"); len(noon) != 1 || !noon[0].StartsAt.Equal(local("2026-10-12", "12:00")) {
		t.Errorf("noon anchor: %v", noon)
	}
	// the gap between the strip and noon is filled with reruns up to the anchor
	before := slices.DeleteFunc(programs(rows), func(a Airing) bool { return !a.StartsAt.Before(local("2026-10-12", "12:00")) })
	if end := before[len(before)-1].EndsAt; end.Before(local("2026-10-12", "11:35")) {
		t.Errorf("the morning ends at %s, leaving a long break before noon", clock(end))
	}
}

// A flow block that would overrun its anchor is shortened: the episode that does not
// fit airs the next day.
func TestOverrunBeforeAnchor(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Drama", repeat(10, 50), 50, "Drama")
	l.series(2, "Short", repeat(20, 100), 8, "Comedy")
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 2)}},
		{Key: "block", Days: dayNames, Start: "17:00", Anchor: true, Fill: FillMarathon, Titles: []string{uid(2, 1)}, Count: 3},
		{Key: "after", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns},
	}}
	rows := planWeek(l.Library, "2026-10-12", 2, func(in *PlanInput) { in.Config = cfg })
	checkTimeline(t, cfg, l, rows)
	mon := l.labels(inSlot(onDay(cfg, rows, "2026-10-12"), "block"))
	tue := l.labels(inSlot(onDay(cfg, rows, "2026-10-13"), "block"))
	// 17:00-17:50, 17:55-18:45, a third would end at 19:40
	if !reflect.DeepEqual(mon, []string{"Drama S01E01", "Drama S01E02"}) || tue[0] != "Drama S01E03" {
		t.Errorf("monday %v, tuesday %v", mon, tue)
	}
	after := inSlot(onDay(cfg, rows, "2026-10-12"), "after")
	if len(after) == 0 || !after[0].StartsAt.Equal(local("2026-10-12", "19:00")) {
		t.Errorf("the 19:00 anchor moved: %v", after)
	}
	// the gap after the shortened block is filled up to the anchor
	gapFill := slices.DeleteFunc(programs(onDay(cfg, rows, "2026-10-12")), func(a Airing) bool { return a.Fill != FillFiller })
	if len(gapFill) == 0 {
		t.Errorf("the 14 minutes before 19:00 were left as one break")
	}
}

// Strips advance in episode order across seasons, wrap around at the end, and pass
// over an episode the slot could never hold.
func TestStripCursor(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Show", []int{3, 2}, 22, "Comedy")
	series := &l.Series[0]
	series.Episodes[3].Duration = 3 * time.Hour // S02E01, a feature-length special
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns},
		{Key: "strip", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillStrip, Count: 2},
		{Key: "late", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns},
	}}
	ep := func(i int) string { return series.Episodes[i].ID }
	tests := []struct {
		name    string
		history []Airing
		cursors []Cursor
		want    []string
	}{
		{"fresh", nil, nil, []string{"S01E01", "S01E02"}},
		{"after a strip airing", []Airing{{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(1), Fill: FillStrip,
			StartsAt: local("2026-10-11", "18:00"), EndsAt: local("2026-10-11", "18:22")}}, nil, []string{"S01E03", "S02E02"}},
		{"reruns do not move it", []Airing{
			{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(0), Fill: FillStrip, StartsAt: local("2026-10-10", "18:00"), EndsAt: local("2026-10-10", "18:22")},
			{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(4), Fill: FillReruns, StartsAt: local("2026-10-11", "07:00"), EndsAt: local("2026-10-11", "07:22")},
		}, nil, []string{"S01E02", "S01E03"}},
		{"wraps around", []Airing{{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(4), Fill: FillMarathon,
			StartsAt: local("2026-10-11", "18:00"), EndsAt: local("2026-10-11", "18:22")}}, nil, []string{"S01E01", "S01E02"}},
		{"an admin restart", []Airing{{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(0), Fill: FillStrip,
			StartsAt: local("2026-10-11", "18:00"), EndsAt: local("2026-10-11", "18:22")}},
			[]Cursor{{SeriesID: series.ID, EpisodeID: ep(2), From: local("2026-10-12", "00:00")}}, []string{"S01E03", "S02E02"}},
		{"a restart the strip has moved past", []Airing{{Kind: KindProgram, TitleID: series.ID, EpisodeID: ep(0), Fill: FillStrip,
			StartsAt: local("2026-10-11", "18:00"), EndsAt: local("2026-10-11", "18:22")}},
			[]Cursor{{SeriesID: series.ID, EpisodeID: ep(2), From: local("2026-10-01", "00:00")}}, []string{"S01E02", "S01E03"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) {
				in.Config, in.History, in.Cursors = cfg, tt.history, tt.cursors
			})
			var got []string
			for _, a := range inSlot(rows, "strip") {
				got = append(got, l.names[a.EpisodeID])
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("strip %v, want %v", got, tt.want)
			}
		})
	}
}

// Reruns prefer episodes that aired before and skip those within rerunGapDays.
func TestReruns(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Big", repeat(20, 400), 22, "Comedy")
	series := l.Series[0]
	cfg := DefaultConfig()
	var history []Airing
	airedAt := map[string]time.Time{}
	for i := range 60 {
		at := local("2026-09-01", "18:00").AddDate(0, 0, i/2)
		e := series.Episodes[i]
		history = append(history, Airing{Kind: KindProgram, TitleID: series.ID, EpisodeID: e.ID, Fill: FillStrip, StartsAt: at, EndsAt: at.Add(e.Duration)})
		airedAt[e.ID] = at
	}
	recent := series.Episodes[59].ID // aired on 2026-09-30, within 14 days of the plan
	rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) {
		in.History = history
		in.Config.Template = CouchTvTemplate{Slots: []CouchTvSlot{
			{Key: "all", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns},
		}}
	})
	checkTimeline(t, cfg, l, rows)
	reruns := inSlot(rows, "all")
	if len(reruns) < 40 {
		t.Fatalf("%d reruns fill the day", len(reruns))
	}
	seen := map[string]bool{}
	for _, a := range reruns {
		if seen[a.EpisodeID] {
			t.Errorf("%s reruns twice in a day", l.label(a))
		}
		if a.EpisodeID == recent {
			t.Errorf("%s reruns within the gap", l.label(a))
		}
		if _, aired := airedAt[a.EpisodeID]; !aired {
			gap := a.StartsAt.AddDate(0, 0, -cfg.Rotation.RerunGapDays)
			for id, at := range airedAt {
				if at.Before(gap) && !seen[id] {
					t.Errorf("%s never aired while %s (aired %s) was left", l.label(a), l.names[id], clock(at))
					break
				}
			}
		}
		seen[a.EpisodeID] = true
	}
}

// Prime time rotates the library: a movie returns only after max(5, 0.8 x count)
// days; matinees rerun recent prime-time movies.
func TestMovieRotation(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	rows := programs(planWeek(lib.Library, "2026-10-12", 28))
	window := time.Duration(0.8 * 12 * float64(24*time.Hour))
	lastPrime := map[string]time.Time{}
	for _, a := range rows {
		switch a.Fill {
		case FillMovie:
			if at, ok := lastPrime[a.TitleID]; ok && a.StartsAt.Sub(at) < window {
				t.Errorf("%s back in prime time on %s, %s after %s", lib.label(a), clock(a.StartsAt), a.StartsAt.Sub(at), clock(at))
			}
		case FillMatinee:
			if _, ok := lastPrime[a.TitleID]; !ok && a.StartsAt.After(local("2026-10-20", "06:00")) {
				t.Errorf("matinee %s on %s never aired in prime time", lib.label(a), clock(a.StartsAt))
			}
		}
		if a.Fill == FillMovie || a.Fill == FillPremiere || a.Fill == FillSaga {
			lastPrime[a.TitleID] = a.StartsAt
		}
	}
	if len(lastPrime) != 12 {
		t.Errorf("%d of 12 movies reached prime time in four weeks", len(lastPrime))
	}
	_ = cfg
}

// Saturday premieres the newest movie added in the last 28 days that never aired, and
// the week before it keeps the movie back (D24).
func TestSaturdayPremiere(t *testing.T) {
	lib := ownerLibrary()
	lib.movie(13, "New Arrival", 2026, 118, 0, "", "Drama")
	lib.Movies[12].AddedAt = time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	lib.movie(14, "Old Stock", 2020, 100, 0, "", "Drama") // added 2026-08-01: too long ago
	cfg := DefaultConfig()
	rows := programs(planWeek(lib.Library, "2026-10-12", 7))
	premiere := inSlot(onDay(cfg, rows, "2026-10-17"), "premiere")
	if len(premiere) != 1 || lib.label(premiere[0]) != "New Arrival" || premiere[0].Fill != FillPremiere {
		t.Fatalf("Saturday 20:15: %v", lib.labels(premiere))
	}
	for _, a := range rows {
		if a.TitleID == premiere[0].TitleID && a.StartsAt.Before(premiere[0].StartsAt) {
			t.Errorf("the premiere aired before Saturday, at %s (%s)", clock(a.StartsAt), a.Fill)
		}
	}

	// once it aired, the next Saturday has no premiere and runs the rotation
	next := programs(planWeek(lib.Library, "2026-10-24", 1, func(in *PlanInput) { in.History = rows }))
	if got := inSlot(next, "premiere"); len(got) != 1 || got[0].Fill != FillMovie {
		t.Errorf("a Saturday without a new movie: %v %v", lib.labels(got), got)
	}
}

// Saga Sunday walks a franchise in order, then the next franchise (D30): TMDB
// collections first, names with a shared prefix for movies without one.
func TestSagaSunday(t *testing.T) {
	lib := ownerLibrary()
	// Iron Man without collection data still groups by name
	for i := range lib.Movies {
		if strings.HasPrefix(lib.Movies[i].Name, "Iron Man") {
			lib.Movies[i].CollectionID, lib.Movies[i].CollectionName = 0, ""
		}
	}
	rows := programs(planWeek(lib.Library, "2026-10-12", 7*11))
	var got []string
	for _, a := range inSlot(rows, "saga") {
		if a.Fill != FillSaga {
			t.Errorf("a Sunday without a saga film: %s", lib.label(a))
		}
		got = append(got, lib.label(a))
	}
	want := []string{
		"Harry Potter a Kámen mudrců", "Harry Potter a Tajemná komnata", "Harry Potter a vězeň z Azkabanu",
		"Harry Potter a Ohnivý pohár", "Harry Potter a Fénixův řád", "Harry Potter a Princ dvojí krve",
		"Harry Potter a Relikvie smrti - část 1", "Harry Potter a Relikvie smrti - část 2",
		"Iron Man", "Iron Man 2", "Iron Man 3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("saga order\n got %q\nwant %q", got, want)
	}
}

func TestSagaGroups(t *testing.T) {
	names := func(groups []sagaGroup) [][]string {
		var out [][]string
		for _, g := range groups {
			var ns []string
			for _, m := range g.movies {
				ns = append(ns, m.Name)
			}
			out = append(out, ns)
		}
		return out
	}
	mk := func(name string, year, collection int) *Movie {
		return &Movie{ID: name, Name: name, Released: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), CollectionID: collection}
	}
	got := names(sagaGroups([]*Movie{
		mk("Alien 3", 1992, 0), mk("Alien", 1979, 0), mk("The Dark Knight", 2008, 0), mk("The Dark Tower", 2017, 0),
		mk("Toy Story 2", 1999, 0), mk("Toy Story", 1995, 0), mk("Pulp Fiction", 1994, 0),
		mk("Star Wars: Episode IV", 1977, 10), mk("Rogue One", 2016, 10), mk("Solo", 2018, 99),
	}))
	want := [][]string{{"Star Wars: Episode IV", "Rogue One"}, {"Alien", "Alien 3"}, {"Toy Story", "Toy Story 2"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groups\n got %q\nwant %q", got, want)
	}
}

// Within prepareAhead free choices prefer prepared media; a strip episode too close to
// air unprepared gives way to a prepared rerun and airs the next day.
func TestReadiness(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Show", repeat(10, 40), 22, "Comedy")
	series := &l.Series[0]
	for i := range series.Episodes {
		series.Episodes[i].Ready = i%2 == 0
	}
	series.Episodes[1].Ready = false // the strip's next episode
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns},
		{Key: "strip", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillStrip},
		{Key: "late", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns},
	}}
	history := []Airing{{Kind: KindProgram, TitleID: series.ID, EpisodeID: series.Episodes[0].ID, Fill: FillStrip,
		StartsAt: local("2026-10-11", "18:00"), EndsAt: local("2026-10-11", "18:22")}}
	plan := func(now time.Time) []Airing {
		return programs(planWeek(l.Library, "2026-10-12", 2, func(in *PlanInput) {
			in.Config, in.History, in.Now = cfg, history, now
		}))
	}

	// far ahead: the strip keeps its episode, whatever its state
	if got := inSlot(plan(longAgo), "strip"); l.label(got[0]) != "Show S01E02" {
		t.Errorf("far ahead the strip airs %s", l.label(got[0]))
	}

	// two hours before air: a prepared rerun stands in, the strip waits a day
	rows := plan(local("2026-10-12", "16:00"))
	strip := inSlot(rows, "strip")
	if strip[0].Fill != FillReruns || !l.Series[0].Episodes[indexOf(series, strip[0].EpisodeID)].Ready {
		t.Errorf("stand-in %s (%s)", l.label(strip[0]), strip[0].Fill)
	}
	if strip[1].Fill != FillStrip || l.label(strip[1]) != "Show S01E02" {
		t.Errorf("the next day the strip airs %s (%s)", l.label(strip[1]), strip[1].Fill)
	}
	// close to air a rerun is always a prepared one, even if it repeats; further out
	// the rotation comes first and preparation catches up
	now := local("2026-10-12", "16:00")
	unprepared := 0
	for _, a := range rows {
		if a.Fill != FillReruns || series.Episodes[indexOf(series, a.EpisodeID)].Ready {
			continue
		}
		if !a.StartsAt.Before(now) && a.StartsAt.Before(now.Add(swapAhead)) {
			t.Errorf("unprepared rerun %s at %s", l.label(a), clock(a.StartsAt))
		}
		unprepared++
	}
	if unprepared == 0 {
		t.Error("two days of reruns from 20 prepared episodes never reached an unprepared one")
	}
}

func indexOf(s *Series, episodeID string) int {
	return slices.IndexFunc(s.Episodes, func(e Episode) bool { return e.ID == episodeID })
}

// SwapUnready replaces unprepared auto programs within swapAhead, keeps locked and
// admin rows, and fits each stand-in before the next program.
func TestSwapUnready(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	rows := programs(planWeek(lib.Library, "2026-10-12", 1))
	now := local("2026-10-12", "17:30")
	// everything after now is unprepared except a few movies and the Simpsons
	for i := range lib.Movies {
		lib.Movies[i].Ready = i < 3
	}
	for s := range lib.Series {
		for e := range lib.Series[s].Episodes {
			lib.Series[s].Episodes[e].Ready = lib.Series[s].Name == "Simpsonovi"
		}
	}
	var upcoming []Airing
	for _, a := range rows {
		if !a.StartsAt.Before(now) {
			upcoming = append(upcoming, a)
		}
	}
	for i := range upcoming {
		upcoming[i].ID = fmt.Sprint(i)
	}
	upcoming[0].Locked = true
	swapped := SwapUnready(SwapInput{Config: cfg, Library: lib.Library, History: rows[:len(rows)-len(upcoming)], Rows: upcoming, Now: now})
	if len(swapped) == 0 {
		t.Fatal("nothing swapped")
	}
	byID := map[string]Airing{}
	for _, a := range upcoming {
		byID[a.ID] = a
	}
	for _, b := range swapped {
		orig := byID[b.ID]
		if b.ID == upcoming[0].ID {
			t.Errorf("the locked %s was swapped", lib.label(orig))
		}
		if !b.StartsAt.Before(now.Add(swapAhead)) {
			t.Errorf("%s at %s swapped beyond the horizon", lib.label(orig), clock(orig.StartsAt))
		}
		if b.EpisodeID == "" != (orig.EpisodeID == "") {
			t.Errorf("%s swapped for %s, another kind", lib.label(orig), lib.label(b))
		}
		if b.Fill == FillStrip || b.Fill == FillSaga || b.Fill == FillPremiere {
			t.Errorf("stand-in %s kept fill %s", lib.label(b), b.Fill)
		}
		i := slices.IndexFunc(upcoming, func(a Airing) bool { return a.ID == b.ID })
		if i+1 < len(upcoming) && b.EndsAt.Add(time.Duration(cfg.BreakMinSeconds)*time.Second).After(upcoming[i+1].StartsAt) {
			t.Errorf("stand-in %s overruns %s", lib.label(b), lib.label(upcoming[i+1]))
		}
		p := newPlanner(cfg, lib.Library, nil, nil, now)
		if !p.ready(b) {
			t.Errorf("stand-in %s is not prepared", lib.label(b))
		}
	}
}

// Admin and locked rows stay; the plan flows around them and a slot they fill does
// not run again.
func TestFixedRowsKept(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	pulp := lib.id("Pulp Fiction")
	dexter := lib.Series[2]
	fixed := []Airing{
		{ID: "admin-1", Kind: KindProgram, TitleID: dexter.ID, EpisodeID: dexter.Episodes[40].ID, Source: SourceAdmin,
			StartsAt: local("2026-10-13", "10:03"), EndsAt: local("2026-10-13", "10:56"), State: StatePlanned},
		{ID: "locked-1", Kind: KindProgram, TitleID: pulp, Slot: "prime-movie", Fill: FillMovie, Source: SourceAuto, Locked: true,
			StartsAt: local("2026-10-13", "20:15"), EndsAt: local("2026-10-13", "22:49"), State: StatePlanned},
	}
	rows := planWeek(lib.Library, "2026-10-12", 3, func(in *PlanInput) { in.Fixed = fixed })
	checkTimeline(t, cfg, lib, rows)
	for _, f := range fixed {
		i := slices.IndexFunc(rows, func(a Airing) bool { return a.ID == f.ID })
		if i < 0 || !reflect.DeepEqual(rows[i], f) {
			t.Errorf("fixed row %s changed or dropped", f.ID)
		}
	}
	tue := onDay(cfg, rows, "2026-10-13")
	if prime := inSlot(tue, "prime-movie"); len(prime) != 1 || prime[0].ID != "locked-1" {
		t.Errorf("Tuesday prime time: %v", lib.labels(prime))
	}
	if encore := inSlot(tue, "encore"); len(encore) != 1 || encore[0].TitleID != pulp {
		t.Errorf("the encore does not repeat the locked movie: %v", lib.labels(encore))
	}
}

// Planning is a pure function, and one day replanned on its own comes out the same.
func TestDeterminism(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	week := planWeek(lib.Library, "2026-10-12", 7)
	if again := planWeek(lib.Library, "2026-10-12", 7); !reflect.DeepEqual(week, again) {
		t.Fatal("two plans of the same week differ")
	}
	thursday := cfg.dayStart(day("2026-10-15"))
	var history []Airing
	for _, a := range programs(week) {
		if a.StartsAt.Before(thursday) {
			history = append(history, a)
		}
	}
	replanned := planWeek(lib.Library, "2026-10-15", 1, func(in *PlanInput) { in.History = history })
	want := onDay(cfg, week, "2026-10-15")
	if got := programs(onDay(cfg, replanned, "2026-10-15")); !reflect.DeepEqual(got, programs(want)) {
		t.Errorf("Thursday replanned differs: %d programs, want %d", len(got), len(programs(want)))
	}
	// the replanned day leads in with the break from Wednesday night
	if lead := replanned[0]; lead.Kind != KindBreak || !lead.EndsAt.Equal(thursday) || !lead.StartsAt.Before(thursday) {
		t.Errorf("no break leads into the replanned day: %+v", lead)
	}
	// a different day draws differently
	if reflect.DeepEqual(lib.labels(inSlot(onDay(cfg, week, "2026-10-13"), "morning")), lib.labels(inSlot(onDay(cfg, week, "2026-10-14"), "morning"))) {
		t.Error("two days drew the same morning")
	}
}

// The channel is on the air around the clock, the autumn clock change included.
func TestAroundTheClock(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	rows := planWeek(lib.Library, "2026-10-24", 2)
	checkTimeline(t, cfg, lib, rows)
	if got := rows[0].StartsAt; !got.Equal(local("2026-10-24", "06:00")) {
		t.Errorf("starts at %s", got)
	}
	sunday := cfg.dayStart(day("2026-10-25"))
	if sunday.Sub(cfg.dayStart(day("2026-10-24"))) != 25*time.Hour {
		t.Errorf("the night of the clock change is not an hour longer")
	}
	if i := slices.IndexFunc(rows, func(a Airing) bool { return a.StartsAt.Equal(sunday) }); i < 0 || rows[i].Kind != KindProgram {
		t.Errorf("Sunday does not open at 06:00 local")
	}
	var longest time.Duration
	for _, a := range rows {
		if a.Kind == KindBreak {
			longest = max(longest, a.EndsAt.Sub(a.StartsAt))
		}
	}
	// a gap is filled while a 22-minute episode fits it, flow starting on the 5-minute grid
	if longest > 28*time.Minute {
		t.Errorf("a break of %s", longest)
	}
}

// Small and lopsided libraries still make a schedule.
func TestSmallLibraries(t *testing.T) {
	tiny := testLibrary{names: map[string]string{}}
	tiny.movie(1, "Only Movie", 2020, 112, 0, "", "Drama")
	tiny.series(1, "Only Show", []int{2}, 43, "Drama")
	moviesOnly := testLibrary{names: map[string]string{}}
	moviesOnly.movie(1, "One", 2020, 95, 0, "")
	moviesOnly.movie(2, "Two", 2021, 101, 0, "")
	seriesOnly := testLibrary{names: map[string]string{}}
	seriesOnly.series(1, "Show", repeat(10, 30), 24, "Comedy")
	for _, c := range []struct {
		name string
		lib  testLibrary
	}{{"tiny", tiny}, {"movies only", moviesOnly}, {"series only", seriesOnly}, {"empty", testLibrary{names: map[string]string{}}}} {
		t.Run(c.name, func(t *testing.T) {
			cfg := DefaultConfig()
			rows := planWeek(c.lib.Library, "2026-10-12", 3)
			checkTimeline(t, cfg, c.lib, rows)
			if c.name != "empty" && len(programs(rows)) < 6 {
				t.Errorf("%d programs in three days", len(programs(rows)))
			}
		})
	}
}

func TestOnAirUsesTheMeasuredCut(t *testing.T) {
	m := Media{Duration: 22 * time.Minute, Cut: 21*time.Minute + 54*time.Second}
	if got := m.onAir(); got != m.Cut {
		t.Errorf("measured for its end: %s", got)
	}
	// credits found since: the cut was measured for the end, so the credits rule
	m.CreditsAt = 21 * time.Minute
	if got := m.onAir(); got != m.CreditsAt {
		t.Errorf("credits since: %s", got)
	}
	m.Cut, m.CutCredits = 21*time.Minute+3*time.Second, 21*time.Minute
	if got := m.onAir(); got != m.Cut {
		t.Errorf("measured for the credits: %s", got)
	}
}
