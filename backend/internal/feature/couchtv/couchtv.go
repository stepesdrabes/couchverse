// Package couchtv runs Couch TV, the live channel (docs/couch-tv-plan.md): the weekly
// template and its config, the pure schedule generator, the plan job and the admin
// operations. It reads catalog and library tables by SQL join; nothing imports it.
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

// StatePlanned is a new row's state; later phases move it on (ready, aired, failed).
const StatePlanned = "planned"

// Airing is one schedule row: a program, or the break between two programs.
type Airing struct {
	ID          string // empty until stored
	StartsAt    time.Time
	EndsAt      time.Time
	Kind        string
	TitleID     string
	EpisodeID   string // empty for a movie
	MediaFileID string
	Slot        string // the template slot that placed it; empty for fillers and admin rows
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
	// CreditsAt is where the end credits start; 0 until the credits markers exist.
	CreditsAt time.Duration
	// Ready is whether the file is prepared for the channel.
	Ready bool
}

// onAir is how long a program holds the screen. Once credits markers exist (a later
// phase) a program airs up to its credits, rounded to its package's segment boundary,
// and the rest of the credits plays inside the next break's squeeze; until then it
// airs in full.
func (m Media) onAir() time.Duration {
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
