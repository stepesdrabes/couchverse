// Package couchtv runs Couch TV, the live channel (docs/couch-tv-plan.md): the weekly
// template and its config, the pure schedule generator, the plan and prepare jobs,
// the per-title analysis for promos, the live stream and the viewer and admin
// operations. It reads catalog and library tables (media markers included) by SQL
// join and imports library to queue marker analysis and program preparation;
// nothing imports it.
package couchtv

import "time"

// Kinds of schedule rows.
const (
	KindProgram = "program"
	KindBreak   = "break"
)

// Sources of schedule rows: the planner's own, or an admin's edit that replanning keeps.
const (
	SourceAuto  = "auto"
	SourceAdmin = "admin"
)

// StatePlanned is a new row's state: couchtv_prepare marks a program ready once its
// media can air, and an airing is aired once it went on air. StateReplaced is a program
// an admin took off: every read skips it, and planning its broadcast day leaves its
// movie or episode out, so the planner's own choice does not come back.
const (
	StatePlanned  = "planned"
	StateReplaced = "replaced"
)

// Airing is one schedule row: a program, or the break between two programs.
type Airing struct {
	ID          string // empty until stored
	StartsAt    time.Time
	EndsAt      time.Time
	Kind        string
	TitleID     string
	EpisodeID   string // empty for a movie
	MediaFileID string
	Slot        string // the template slot it airs in; empty for fillers and inserted programs
	Fill        string
	Source      string
	Locked      bool
	State       string
	CreditsAt   time.Duration // where the squeeze begins, from the start; 0 while unknown
}

// Media is what airs for a movie or an episode.
type Media struct {
	FileID   string
	Duration time.Duration
	// CreditsAt is where the end credits start (the file's credits marker); 0 without one.
	CreditsAt time.Duration
	// Ready is whether the file is prepared for the channel.
	Ready bool
	// Cut is the exact time on air couchtv_prepare measured in the file's package
	// for credits at CutCredits (0: the end); 0 until it did.
	Cut, CutCredits time.Duration
}

// onAir is how long a program holds the screen: up to its credits when it has a credits
// marker (the rest of the credits plays inside the next break's squeeze), else in full,
// cut at its package's segment boundary nearest that once couchtv_prepare measured the
// package for these credits.
func (m Media) onAir() time.Duration {
	if m.Cut > 0 && m.CutCredits == m.CreditsAt {
		return m.Cut
	}
	if m.CreditsAt > 0 && m.CreditsAt < m.Duration {
		return m.CreditsAt
	}
	return m.Duration
}

// Movie is a published movie the channel can air.
type Movie struct {
	ID             string
	Name           string
	Genres         []string
	Released       time.Time // zero when unknown
	AddedAt        time.Time
	CollectionID   int // TMDB collection, 0 for none
	CollectionName string
	Media
}

// Series is a published series with its airable episodes in broadcast order: seasons
// ascending (specials left out), episodes ascending.
type Series struct {
	ID       string
	Name     string
	Genres   []string
	Episodes []Episode
}

// Episode is one airable episode of a series.
type Episode struct {
	ID     string
	Season int
	Number int
	Media
}

// Library is what the channel can air.
type Library struct {
	Movies []Movie
	Series []Series
}

// Cursor restarts a series' strip at an episode from a moment on (couchtv_strips).
type Cursor struct {
	SeriesID  string
	EpisodeID string
	From      time.Time
}
