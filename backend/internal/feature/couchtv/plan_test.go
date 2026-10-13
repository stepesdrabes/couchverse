package couchtv

import (
	"errors"
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
		name     string
		mutate   func(*CouchTvConfig)
		wantErr  string
		wantCode string
	}{
		{"defaults", func(*CouchTvConfig) {}, "", ""},
		{"other zone", func(c *CouchTvConfig) { c.TimeZone = "America/New_York" }, "", ""},
		{"unknown zone", func(c *CouchTvConfig) { c.TimeZone = "Mars/Olympus" }, "time zone", ErrTimeZone},
		{"break too short", func(c *CouchTvConfig) { c.BreakMinSeconds = 2 }, "breakMinSeconds", ErrRotation},
		{"negative rotation", func(c *CouchTvConfig) { c.Rotation.RerunGapDays = -1 }, "rerunGapDays", ErrRotation},
		{"no filler minimum", func(c *CouchTvConfig) { c.Rotation.FillerMinMinutes = 0 }, "fillerMinMinutes", ErrRotation},
		{"duplicate key", func(c *CouchTvConfig) { c.Template.Slots[1].Key = "morning" }, "twice", ErrSlot},
		{"day must open with an anchor", func(c *CouchTvConfig) { c.Template.Slots[0].Anchor = false }, "must be an anchor", ErrDayOpen},
		{"anchor needs a time", func(c *CouchTvConfig) { c.Template.Slots[6].Start = "" }, "needs a start", ErrAnchorTime},
		{"times ascend", func(c *CouchTvConfig) { c.Template.Slots[7].Start = "16:00" }, "not after", ErrSlotOrder},
		{"bad time", func(c *CouchTvConfig) { c.Template.Slots[7].Start = "25:00" }, "HH:MM", ErrSlot},
		{"a day without slots", func(c *CouchTvConfig) {
			for i := range c.Template.Slots {
				c.Template.Slots[i].Days = slices.DeleteFunc(slices.Clone(c.Template.Slots[i].Days), func(d string) bool { return d == "wed" })
			}
		}, "wed has no slots", ErrDayEmpty},
		{"reruns need an end", func(c *CouchTvConfig) {
			c.Template.Slots = slices.DeleteFunc(c.Template.Slots, func(s CouchTvSlot) bool { return s.Key == "encore" })
			c.Template.Slots = append(c.Template.Slots, CouchTvSlot{Key: "tail", Days: []string{"mon"}, Fill: FillMovie})
		}, "needs a timed slot", ErrRerunsEnd},
		{"a strip pins one series", func(c *CouchTvConfig) { c.Template.Slots[7].Titles = []string{uid(2, 1), uid(2, 2)} }, "pins one", ErrPinnedTitle},
		{"pinned ids", func(c *CouchTvConfig) { c.Template.Slots[9].Titles = []string{"iron-man"} }, "not a title id", ErrPinnedTitle},
		{"unknown fill", func(c *CouchTvConfig) { c.Template.Slots[9].Fill = "news" }, "unknown fill", ErrSlot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			var invalid *ConfigError
			if tt.wantCode != "" && (!errors.As(err, &invalid) || invalid.Code != tt.wantCode) {
				t.Errorf("err %v has no code %q", err, tt.wantCode)
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

// Slack no filler fits before an anchor is spread over the flow since the last program
// that cannot move, as evenly as the 5-minute grid allows; anchors and fixed rows keep
// their times, and a replan after the block started keeps the spread.
func TestSpreadSlack(t *testing.T) {
	const date = "2026-10-12"
	tests := []struct {
		name    string
		block   string // the block's programs from 12:00 on
		fixed   string // an admin row of 25 minutes at this time
		want    string // the starts from 12:00 to 17:00
		longest time.Duration
	}{
		// an afternoon measured on production: 14:15, 15:05, 16:00, 16:25, then 12:24 of break
		{"a mixed afternoon", "2h11m 45m 53m 22m 22m36s", "", "12:00 14:15 15:05 16:05 16:30", 7*time.Minute + 24*time.Second},
		// otherwise every 25 minutes from 14:10, then 20:00 of break
		{"sitcoms of 22 to 25 minutes", "2h8m 24m 24m30s 22m 23m30s 24m 25m", "", "12:00 14:10 14:40 15:10 15:35 16:00 16:30", 6 * time.Minute},
		// otherwise every 25 minutes from 14:20, then 13:20 of break; a 22-minute episode
		// leaves about 3 minutes to the next mark, so a break that grows makes 8
		{"22-minute sitcoms", "2h16m40s 21m40s 21m50s 21m40s 21m50s 21m40s 21m40s", "", "12:00 14:20 14:45 15:15 15:40 16:10 16:35", 8*time.Minute + 10*time.Second},
		// otherwise 3:20 before it and 13:36 after: the program goes in between
		{"one program after the anchor", "4h21m40s 21m24s", "", "12:00 16:30", 8*time.Minute + 36*time.Second},
		// nothing can move, so the gap stays
		{"only the anchored program", "4h40m", "", "12:00", 20 * time.Minute},
		// the flow before the admin row ends 3 minutes before it; after it, otherwise
		// 15:30 and 16:25, then 13:00 of break
		{"a fixed row", "2h11m 45m 53m 22m 22m36s", "15:03", "12:00 14:15 15:03 15:35 16:35", 7 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := testLibrary{names: map[string]string{}}
			lengths := strings.Fields(tt.block)
			l.series(1, "Block", []int{len(lengths)}, 22, "Comedy")
			for i, s := range lengths {
				d, err := time.ParseDuration(s)
				if err != nil {
					t.Fatal(err)
				}
				l.Series[0].Episodes[i].Duration = d
			}
			l.series(2, "Strip", []int{10}, 25, "Comedy")
			l.series(3, "Long", repeat(10, 100), 50, "Drama")
			cfg := DefaultConfig()
			cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
				{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
				{Key: "block", Days: dayNames, Start: "12:00", Anchor: true, Fill: FillStrip, Titles: []string{uid(2, 1)}, Count: len(lengths)},
				{Key: "strip", Days: dayNames, Start: "17:00", Anchor: true, Fill: FillStrip, Titles: []string{uid(2, 2)}, Count: 1},
				{Key: "late", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
			}}
			var fixed []Airing
			if tt.fixed != "" {
				at := local(date, tt.fixed)
				fixed = append(fixed, Airing{ID: "admin-1", Kind: KindProgram, TitleID: uid(2, 2), EpisodeID: l.Series[1].Episodes[4].ID,
					Source: SourceAdmin, StartsAt: at, EndsAt: at.Add(25 * time.Minute), State: StatePlanned})
			}
			plan := func(keep []Airing, now time.Time) []Airing {
				return planWeek(l.Library, date, 1, func(in *PlanInput) { in.Config, in.Fixed, in.Now = cfg, keep, now })
			}
			from, until := local(date, "12:00"), local(date, "17:00")
			block := func(rows []Airing) (starts string, longest time.Duration) {
				var out []string
				for _, a := range rows {
					if a.StartsAt.Before(from) || !a.StartsAt.Before(until) {
						continue
					}
					if a.Kind == KindBreak {
						longest = max(longest, a.EndsAt.Sub(a.StartsAt))
						continue
					}
					out = append(out, a.StartsAt.In(prague).Format("15:04"))
				}
				return strings.Join(out, " "), longest
			}

			rows := plan(fixed, longAgo)
			checkTimeline(t, cfg, l, rows)
			starts, longest := block(rows)
			if starts != tt.want || longest != tt.longest {
				t.Errorf("starts %s with a longest break of %s, want %s and %s", starts, longest, tt.want, tt.longest)
			}
			if strip := inSlot(rows, "strip"); len(strip) == 0 || !strip[0].StartsAt.Equal(until) {
				t.Errorf("the 17:00 anchor moved: %v", strip)
			}
			for _, f := range fixed {
				if i := slices.IndexFunc(rows, func(a Airing) bool { return a.ID == f.ID }); i < 0 || !reflect.DeepEqual(rows[i], f) {
					t.Errorf("fixed row %s changed or dropped", f.ID)
				}
			}

			// at 15:00 the programs on air so far are fixed: the rest keeps its times
			now := local(date, "15:00")
			started := slices.Clone(fixed)
			for _, a := range programs(rows) {
				if a.StartsAt.Before(now) {
					started = append(started, a)
				}
			}
			if again, _ := block(plan(started, now)); again != starts {
				t.Errorf("replanned at 15:00: %s, want %s", again, starts)
			}
		})
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

// randomLibrary has Toons, a series whose episodes may air in any order, beside Saga, one
// that airs in order, and Filler for the reruns around them. Every episode runs about 22
// minutes, so no filler fits the gap a block leaves before an anchor.
func randomLibrary() testLibrary {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Toons", repeat(20, 60), 22, "Animation", "Comedy")
	l.Series[0].Random = true
	l.series(2, "Saga", repeat(10, 40), 22, "Drama")
	l.series(3, "Filler", repeat(20, 200), 22, "Comedy")
	return l
}

// randomConfig airs two Toons at 17:00 and two Saga at 18:00, between reruns of Filler.
func randomConfig() CouchTvConfig {
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
		{Key: "toons", Days: dayNames, Start: "17:00", Anchor: true, Fill: FillStrip, Titles: []string{uid(2, 1)}, Count: 2},
		{Key: "saga", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillStrip, Titles: []string{uid(2, 2)}, Count: 2},
		{Key: "late", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
	}}
	return cfg
}

// aired is a past airing of an episode.
func aired(ser Series, i int, at time.Time, fill string) Airing {
	e := ser.Episodes[i]
	return Airing{Kind: KindProgram, TitleID: ser.ID, EpisodeID: e.ID, MediaFileID: e.FileID, Fill: fill, Source: SourceAuto,
		StartsAt: at, EndsAt: at.Add(e.Duration), State: StatePlanned}
}

// A strip of a series that may air in any order draws its episodes at random: those
// that never aired first, none twice within rerunGapDays, the same rows again for an
// unchanged day, while a strip in order goes on in order.
func TestRandomStrips(t *testing.T) {
	l := randomLibrary()
	cfg := randomConfig()
	toons := &l.Series[0]
	// five episodes aired within the gap of the planned week, five long before it
	var history []Airing
	for i := range 10 {
		at := local("2026-10-05", "17:00")
		if i >= 5 {
			at = local("2026-09-01", "17:00")
		}
		history = append(history, aired(*toons, i, at.Add(time.Duration(i%5)*25*time.Minute), FillStrip))
	}
	plan := func(from string, days int, history []Airing) []Airing {
		return planWeek(l.Library, from, days, func(in *PlanInput) { in.Config, in.History = cfg, history })
	}
	week := plan("2026-10-12", 7, history)
	checkTimeline(t, cfg, l, week)

	strip := inSlot(week, "toons")
	if len(strip) != 14 {
		t.Fatalf("%d Toons in a week of two a day: %v", len(strip), l.labels(strip))
	}
	var order []int
	for _, a := range strip {
		i := indexOf(toons, a.EpisodeID)
		if a.TitleID != toons.ID || a.Fill != FillStrip || i < 0 {
			t.Fatalf("the Toons strip airs %s (%s)", l.label(a), a.Fill)
		}
		if i < 10 {
			t.Errorf("%s at %s aired before, while 50 episodes never did", l.label(a), clock(a.StartsAt))
		}
		order = append(order, i)
	}
	if slices.IsSorted(order) {
		t.Errorf("the strip airs in order: %v", l.labels(strip))
	}
	// whichever slot airs a Toons episode, it does not come back within the gap
	last := map[string]time.Time{}
	for _, a := range append(slices.Clone(history), programs(week)...) {
		if a.TitleID != toons.ID {
			continue
		}
		if at, ok := last[a.EpisodeID]; ok && a.StartsAt.Before(at.AddDate(0, 0, cfg.Rotation.RerunGapDays)) {
			t.Errorf("%s at %s again, after %s", l.label(a), clock(a.StartsAt), at.Format(time.DateOnly))
		}
		last[a.EpisodeID] = a.StartsAt
	}

	// the strip in order is untouched
	var saga []string
	for _, a := range inSlot(week, "saga") {
		saga = append(saga, l.label(a))
	}
	want := []string{"Saga S01E01", "Saga S01E02", "Saga S01E03", "Saga S01E04", "Saga S01E05", "Saga S01E06", "Saga S01E07",
		"Saga S01E08", "Saga S01E09", "Saga S01E10", "Saga S02E01", "Saga S02E02", "Saga S02E03", "Saga S02E04"}
	if !reflect.DeepEqual(saga, want) {
		t.Errorf("Saga strip %v", saga)
	}

	// an unchanged week plans the same, and the days after the ones planned already come
	// out the same when replanned on their own
	if again := plan("2026-10-12", 7, history); !reflect.DeepEqual(again, week) {
		t.Error("two plans of the same week differ")
	}
	thursday := cfg.dayStart(day("2026-10-15"))
	past := slices.Clone(history)
	for _, a := range programs(week) {
		if a.StartsAt.Before(thursday) {
			past = append(past, a)
		}
	}
	later := slices.DeleteFunc(programs(week), func(a Airing) bool { return a.StartsAt.Before(thursday) })
	if got := programs(plan("2026-10-15", 4, past)); !reflect.DeepEqual(got, later) {
		t.Errorf("Thursday to Sunday replanned differ: %v, want %v", l.labels(inSlot(got, "toons")), l.labels(inSlot(later, "toons")))
	}

	// a draw changes only with its own episodes: ten more that never aired had aired
	// long ago, and Monday airs the same two
	monday := inSlot(onDay(cfg, week, "2026-10-12"), "toons")
	more := slices.Clone(history)
	for i := 10; len(more) < len(history)+10; i++ {
		if !slices.ContainsFunc(monday, func(a Airing) bool { return a.EpisodeID == toons.Episodes[i].ID }) {
			more = append(more, aired(*toons, i, local("2026-08-01", "17:00").Add(time.Duration(i)*25*time.Minute), FillReruns))
		}
	}
	if got := inSlot(plan("2026-10-12", 1, more), "toons"); !reflect.DeepEqual(l.labels(got), l.labels(monday)) {
		t.Errorf("with ten fewer episodes to draw from Monday airs %v, want %v", l.labels(got), l.labels(monday))
	}
}

// On a library shaped like production's, the series the owner lets play at random keep
// their strips (plan 5.1's lineup) and air them out of order without a repeat all week;
// Dexter stays in order.
func TestRandomOwnerWeek(t *testing.T) {
	lib := ownerLibrary()
	for i := range lib.Series {
		switch lib.Series[i].Name {
		case "Simpsonovi", "Městečko South Park", "Věřte nevěřte":
			lib.Series[i].Random = true
		}
	}
	cfg := DefaultConfig()
	rows := planWeek(lib.Library, "2026-10-12", 7)
	checkTimeline(t, cfg, lib, rows)
	for slot, name := range map[string]string{"early-strip": "Věřte nevěřte", "sitcom-strip": "Simpsonovi",
		"second-sitcom-strip": "Městečko South Park"} {
		strip := inSlot(rows, slot)
		var order []int
		for _, a := range strip {
			i := slices.IndexFunc(lib.Series, func(s Series) bool { return s.ID == a.TitleID })
			if i < 0 || lib.Series[i].Name != name || a.Fill != FillStrip {
				t.Fatalf("%s airs %s (%s)", slot, lib.label(a), a.Fill)
			}
			order = append(order, indexOf(&lib.Series[i], a.EpisodeID))
		}
		if len(strip) < 7 || slices.IsSorted(order) {
			t.Errorf("%s: %v", slot, lib.labels(strip))
		}
	}
	seen := map[string]string{}
	for _, a := range programs(rows) {
		i := slices.IndexFunc(lib.Series, func(s Series) bool { return s.ID == a.TitleID })
		if i < 0 || !lib.Series[i].Random {
			continue
		}
		if at, ok := seen[a.EpisodeID]; ok {
			t.Errorf("%s at %s again, after %s", lib.label(a), clock(a.StartsAt), at)
		}
		seen[a.EpisodeID] = clock(a.StartsAt)
	}
	var dexter []string
	for _, a := range programs(rows) {
		if (a.Fill == FillStrip || a.Fill == FillMarathon) && lib.names[a.TitleID] == "Dexter" {
			dexter = append(dexter, lib.names[a.EpisodeID])
		}
	}
	if len(dexter) < 6 || !slices.IsSorted(dexter) || dexter[0] != "S01E01" {
		t.Errorf("Dexter: %v", dexter)
	}
}

// Once every episode aired, a strip at random airs the one aired longest ago, outside
// rerunGapDays first, and never the same episode twice in a day.
func TestRandomStripRepeats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
		{Key: "strip", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillStrip, Titles: []string{uid(2, 1)}, Count: 2},
		{Key: "late", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
	}}
	tests := []struct {
		name     string
		episodes int
		history  func(Series) []Airing
		check    func(t *testing.T, days [][]int)
	}{
		{"never aired first, then the one aired longest ago", 6, nil, func(t *testing.T, days [][]int) {
			if first := slices.Concat(days[0], days[1], days[2]); len(slices.Compact(slices.Sorted(slices.Values(first)))) != 6 {
				t.Errorf("the first three days air %v, not all six episodes", first)
			}
			for d := 3; d < len(days); d++ {
				if !slices.Equal(days[d], days[d-3]) {
					t.Errorf("day %d airs %v, want %v as aired three days before", d, days[d], days[d-3])
				}
			}
		}},
		{"outside the gap first", 6, func(ser Series) []Airing {
			return []Airing{
				aired(ser, 0, local("2026-10-09", "18:00"), FillStrip), aired(ser, 1, local("2026-10-09", "18:25"), FillStrip),
				aired(ser, 2, local("2026-09-20", "18:00"), FillStrip), aired(ser, 3, local("2026-09-20", "18:25"), FillStrip),
				aired(ser, 4, local("2026-09-21", "18:00"), FillStrip), aired(ser, 5, local("2026-09-21", "18:25"), FillStrip),
			}
		}, func(t *testing.T, days [][]int) {
			if want := [][]int{{2, 3}, {4, 5}, {0, 1}, {2, 3}}; !reflect.DeepEqual(days[:4], want) {
				t.Errorf("days %v, want %v", days[:4], want)
			}
		}},
		{"the strip airs an episode once a day", 1, nil, func(t *testing.T, days [][]int) {
			for d, eps := range days {
				if !slices.Equal(eps, []int{0}) {
					t.Errorf("day %d airs %v", d, eps)
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := testLibrary{names: map[string]string{}}
			l.series(1, "Few", []int{tt.episodes}, 22, "Comedy")
			l.Series[0].Random = true
			l.series(3, "Filler", repeat(20, 200), 22, "Comedy")
			few := &l.Series[0]
			var history []Airing
			if tt.history != nil {
				history = tt.history(*few)
			}
			rows := planWeek(l.Library, "2026-10-12", 7, func(in *PlanInput) { in.Config, in.History = cfg, history })
			checkTimeline(t, cfg, l, rows)
			var days [][]int
			for d := range 7 {
				var eps []int
				for _, a := range inSlot(onDay(cfg, rows, day("2026-10-12").AddDate(0, 0, d).Format(time.DateOnly)), "strip") {
					if a.TitleID != few.ID || a.Fill != FillStrip {
						t.Errorf("the strip airs %s (%s)", l.label(a), a.Fill)
					}
					eps = append(eps, indexOf(few, a.EpisodeID))
				}
				days = append(days, eps)
			}
			tt.check(t, days)
		})
	}
}

// Close to air a strip at random draws prepared episodes, and with none of the series
// prepared a prepared rerun stands in; further out it draws from them all.
func TestRandomStripReadiness(t *testing.T) {
	l := randomLibrary()
	cfg := randomConfig()
	toons := &l.Series[0]
	for i := range toons.Episodes {
		toons.Episodes[i].Ready = i%3 == 0
	}
	now := local("2026-10-12", "16:00")
	plan := func() []Airing {
		return programs(planWeek(l.Library, "2026-10-12", 5, func(in *PlanInput) { in.Config, in.Now = cfg, now }))
	}
	unprepared := 0
	for _, a := range inSlot(plan(), "toons") {
		ready := toons.Episodes[indexOf(toons, a.EpisodeID)].Ready
		switch {
		case a.TitleID != toons.ID || a.Fill != FillStrip:
			t.Errorf("the strip airs %s (%s) at %s", l.label(a), a.Fill, clock(a.StartsAt))
		case !ready && a.StartsAt.Before(now.Add(prepareAhead)):
			t.Errorf("unprepared %s at %s, while prepared episodes never aired", l.label(a), clock(a.StartsAt))
		case !ready:
			unprepared++
		}
	}
	if unprepared == 0 {
		t.Error("beyond 48 hours the strip drew prepared episodes only")
	}

	for i := range toons.Episodes {
		toons.Episodes[i].Ready = false
	}
	for _, a := range inSlot(onDay(cfg, plan(), "2026-10-12"), "toons") {
		if a.TitleID == toons.ID || a.Fill != FillReruns {
			t.Errorf("an hour before air the strip airs %s (%s) with nothing of it prepared", l.label(a), a.Fill)
		}
	}
}

// A strip at random leaves out what an admin took off its day, counts a fixed program
// toward itself, and keeps clear of episodes fixed later that day or within the gap after.
func TestRandomStripRemovedAndFixed(t *testing.T) {
	l := randomLibrary()
	cfg := randomConfig()
	toons := l.Series[0]
	plan := func(set func(*PlanInput)) []Airing {
		return planWeek(l.Library, "2026-10-12", 3, func(in *PlanInput) { in.Config = cfg; set(in) })
	}
	week := plan(func(*PlanInput) {})
	mon := inSlot(onDay(cfg, week, "2026-10-12"), "toons")
	tue := inSlot(onDay(cfg, week, "2026-10-13"), "toons")

	removed := mon[0]
	removed.State = StateReplaced
	rows := plan(func(in *PlanInput) { in.Removed = []Airing{removed} })
	checkTimeline(t, cfg, l, rows)
	if got := inSlot(onDay(cfg, rows, "2026-10-12"), "toons"); len(got) != 2 ||
		slices.ContainsFunc(programs(onDay(cfg, rows, "2026-10-12")), func(a Airing) bool { return a.EpisodeID == removed.EpisodeID }) {
		t.Errorf("Monday's strip %v after removing %s", l.labels(got), l.label(removed))
	}

	// Monday's second episode is locked, the first fixed at 22:00 and Tuesday's first on
	// Wednesday morning
	locked := mon[1]
	locked.ID, locked.Locked = "locked-1", true
	late := aired(toons, indexOf(&toons, mon[0].EpisodeID), local("2026-10-12", "22:00"), "")
	late.ID, late.Source = "admin-1", SourceAdmin
	ahead := aired(toons, indexOf(&toons, tue[0].EpisodeID), local("2026-10-14", "10:00"), "")
	ahead.ID, ahead.Source = "admin-2", SourceAdmin
	rows = plan(func(in *PlanInput) { in.Fixed = []Airing{locked, late, ahead} })
	checkTimeline(t, cfg, l, rows)
	got := inSlot(onDay(cfg, rows, "2026-10-12"), "toons")
	if len(got) != 2 || got[1].ID != locked.ID || got[0].EpisodeID == locked.EpisodeID || got[0].EpisodeID == late.EpisodeID {
		t.Errorf("Monday's strip %v around the locked %s and %s at 22:00", l.labels(got), l.label(locked), l.label(late))
	}
	for _, f := range []Airing{late, ahead} {
		if n := len(slices.DeleteFunc(programs(rows), func(a Airing) bool { return a.EpisodeID != f.EpisodeID })); n != 1 {
			t.Errorf("%s airs %d times, though fixed once at %s", l.label(f), n, clock(f.StartsAt))
		}
	}
}

// Reruns of a series airing at random draw from all its episodes; those of a series in
// order keep to the episodes that aired before.
func TestRandomReruns(t *testing.T) {
	l := randomLibrary()
	toons, saga := l.Series[0], l.Series[1]
	var history []Airing
	for k, ser := range []Series{toons, saga} {
		for i := range 20 {
			history = append(history, aired(ser, i, local("2026-09-01", "06:00").Add(time.Duration(k*20+i)*25*time.Minute), FillReruns))
		}
	}
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns, Titles: []string{toons.ID, saga.ID}},
		{Key: "late", Days: dayNames, Start: "12:00", Anchor: true, Fill: FillReruns, Titles: []string{uid(2, 3)}},
	}}
	rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) { in.Config, in.History = cfg, history })
	checkTimeline(t, cfg, l, rows)
	fresh := 0
	for _, a := range inSlot(rows, "open") {
		switch {
		case a.TitleID == saga.ID && indexOf(&saga, a.EpisodeID) >= 20:
			t.Errorf("Saga reruns %s, which never aired, while aired episodes were left", l.label(a))
		case a.TitleID == toons.ID && indexOf(&toons, a.EpisodeID) >= 20:
			fresh++
		}
	}
	if fresh == 0 {
		t.Errorf("Toons reruns only episodes that aired before: %v", l.labels(inSlot(rows, "open")))
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

// A fixed program counts toward its slot: a locked second strip episode keeps the
// strip's first one in place.
func TestFixedRowsCountTowardTheirSlot(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Show", []int{6}, 22, "Comedy")
	series := l.Series[0]
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillReruns},
		{Key: "strip", Days: dayNames, Start: "18:00", Anchor: true, Fill: FillStrip, Count: 2},
		{Key: "late", Days: dayNames, Start: "19:00", Anchor: true, Fill: FillReruns},
	}}
	locked := Airing{ID: "locked-1", Kind: KindProgram, TitleID: series.ID, EpisodeID: series.Episodes[1].ID,
		MediaFileID: series.Episodes[1].FileID, Slot: "strip", Fill: FillStrip, Source: SourceAuto, Locked: true,
		StartsAt: local("2026-10-12", "18:25"), EndsAt: local("2026-10-12", "18:47"), State: StatePlanned}
	rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) {
		in.Config, in.Fixed = cfg, []Airing{locked}
	})
	checkTimeline(t, cfg, l, rows)
	strip := inSlot(rows, "strip")
	if got := l.labels(strip); !reflect.DeepEqual(got, []string{"Show S01E01", "Show S01E02"}) || strip[1].ID != "locked-1" {
		t.Errorf("strip %v, want the first episode before the locked second", got)
	}
}

// A program an admin took off stays off its day: the slot airs something else, and a
// strip passes the episode over.
func TestRemovedPrograms(t *testing.T) {
	lib := ownerLibrary()
	cfg := DefaultConfig()
	week := planWeek(lib.Library, "2026-10-12", 2)
	mon := onDay(cfg, week, "2026-10-12")
	prime := inSlot(mon, "prime-movie")[0]
	strip := inSlot(mon, "sitcom-strip")
	rerun := inSlot(mon, "morning")[1]
	removed := []Airing{prime, strip[0], rerun}
	for i := range removed {
		removed[i].State = StateReplaced
	}
	rows := planWeek(lib.Library, "2026-10-12", 2, func(in *PlanInput) { in.Removed = removed })
	checkTimeline(t, cfg, lib, rows)
	again := onDay(cfg, rows, "2026-10-12")
	for _, r := range removed {
		if slices.ContainsFunc(programs(again), func(a Airing) bool { return programKey(a) == programKey(r) }) {
			t.Errorf("%s still airs on Monday", lib.label(r))
		}
	}
	if got := inSlot(again, "prime-movie"); len(got) != 1 || got[0].TitleID == prime.TitleID {
		t.Errorf("Monday prime time: %v", lib.labels(got))
	}
	// the strip airs the two episodes after the removed one, and goes on from there
	got := inSlot(again, "sitcom-strip")
	if len(got) != 2 || got[0].EpisodeID != strip[1].EpisodeID {
		t.Fatalf("Monday's strip %v after removing %s", lib.labels(got), lib.label(strip[0]))
	}
	i := slices.IndexFunc(lib.Series, func(s Series) bool { return s.ID == got[1].TitleID })
	eps := lib.Series[i].Episodes
	next := eps[slices.IndexFunc(eps, func(e Episode) bool { return e.ID == got[1].EpisodeID })+1]
	if tue := inSlot(onDay(cfg, rows, "2026-10-13"), "sitcom-strip"); len(tue) == 0 || tue[0].EpisodeID != next.ID {
		t.Errorf("Tuesday's strip %v does not go on after Monday's", lib.labels(tue))
	}
}

func TestAdminFill(t *testing.T) {
	const show, other = "show", "other"
	tests := []struct {
		rowFill, rowTitle, title string
		movie                    bool
		want                     string
	}{
		{FillStrip, show, show, false, FillStrip},       // another episode restarts the strip there
		{FillStrip, show, other, false, FillReruns},     // another series must not move its own strip
		{FillStrip, show, other, true, FillMatinee},     // a movie in a strip's place
		{FillSaga, show, show, true, FillSaga},          // the same film stays the saga's
		{FillSaga, show, other, true, FillMovie},        // another film is not the franchise's next
		{FillPremiere, show, other, true, FillMovie},    // nor a premiere
		{FillMovie, show, other, false, FillReruns},     // an episode in prime time
		{FillFiller, show, other, true, FillFiller},     // a gap filler stays one
		{FillEncore, show, other, true, FillMatinee},    // a rerun of another movie
		{"", "", other, true, FillMatinee},              // an inserted movie
		{"", "", other, false, FillReruns},              // an inserted episode
		{FillReruns, show, other, false, FillReruns},    // another rerun
		{FillMarathon, show, show, false, FillMarathon}, // the marathon goes on from it
	}
	for _, tt := range tests {
		if got := adminFill(tt.rowFill, tt.rowTitle, tt.title, tt.movie); got != tt.want {
			t.Errorf("adminFill(%q, %q, %q, movie %v) = %q, want %q", tt.rowFill, tt.rowTitle, tt.title, tt.movie, got, tt.want)
		}
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
	for i, a := range rows {
		if a.Kind != KindBreak {
			continue
		}
		length := a.EndsAt.Sub(a.StartsAt)
		longest = max(longest, length)
		// what no filler fits is spread over the flow before the next anchor, so a break
		// runs past ten minutes only after a program at its anchor, which cannot move
		if prev := rows[i-1]; length > breakTarget+flowStep && !atAnchor(cfg, prev) {
			t.Errorf("a break of %s after %s at %s", length, lib.label(prev), clock(prev.StartsAt))
		}
	}
	// a gap is filled while a 22-minute episode fits it, flow starting on the 5-minute grid
	if longest > 28*time.Minute {
		t.Errorf("a break of %s", longest)
	}
}

// atAnchor reports whether a program starts at its anchored slot's time.
func atAnchor(cfg CouchTvConfig, a Airing) bool {
	i := slices.IndexFunc(cfg.Template.Slots, func(s CouchTvSlot) bool { return s.Key == a.Slot })
	return i >= 0 && cfg.Template.Slots[i].Anchor && a.StartsAt.Equal(cfg.slotTime(cfg.dayOf(a.StartsAt), cfg.Template.Slots[i].Start))
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
