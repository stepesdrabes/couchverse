package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/settings"
)

// settingsKey is where the channel's config lives.
const settingsKey = "couchtv"

// Fills say what a template slot airs. Programs carry the fill that chose them, so
// the planner reads strips, the movie rotation and the saga back from history.
const (
	FillStrip    = "strip"    // a series in episode order, count per day
	FillMarathon = "marathon" // the same, a longer block (weekends)
	FillReruns   = "reruns"   // episodes that already aired, up to the next slot's time
	FillMovie    = "movie"    // the prime-time rotation
	FillMatinee  = "matinee"  // a rerun of a movie that aired in prime time
	FillPremiere = "premiere" // the newest movie that never aired, else a movie
	FillSaga     = "saga"     // the next film of a franchise, else a movie
	FillEncore   = "encore"   // tonight's prime-time movie again, else a matinee
	// FillFiller is no slot's fill: the planner fills a long gap before a fixed time
	// with it.
	FillFiller = "filler"
)

// Picks prefer titles by genre; a slot falls back to every title when none matches.
const (
	PickAny       = "any"
	PickComedy    = "comedy"
	PickAnimation = "animation"
	PickDrama     = "drama"
	PickFamily    = "family"
)

// dayNames are the template's day names, indexed by time.Weekday.
var dayNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// CouchTvSlot is one entry of the weekly template. A day's slots run in template
// order; the first one starts the broadcast day and must be an anchor.
type CouchTvSlot struct {
	Key    string   `json:"key" pattern:"^[a-z0-9][a-z0-9-]*$" maxLength:"40" doc:"Stable name; airings record it."`
	Days   []string `json:"days" minItems:"1" maxItems:"7" enum:"mon,tue,wed,thu,fri,sat,sun"`
	Start  string   `json:"start,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" doc:"Local time (HH:MM). An anchor starts exactly then; for a flow slot it is the time the block before it fills up to. Times before the day's first anchor belong to the night after it."`
	Anchor bool     `json:"anchor,omitempty" doc:"Starts exactly at start; programs before it are shortened or swapped to make room."`
	Fill   string   `json:"fill" enum:"strip,marathon,reruns,movie,matinee,premiere,saga,encore"`
	Pick   string   `json:"pick,omitempty" enum:"any,comedy,animation,drama,family" doc:"Genre the slot prefers; any when omitted."`
	Count  int      `json:"count,omitempty" minimum:"0" maximum:"20" doc:"Episodes a strip or marathon airs per day; 1 when omitted."`
	Titles []string `json:"titles,omitempty" format:"uuid" maxItems:"50" doc:"Pinned titles: a strip or marathon airs the first (a series), reruns and movie slots draw only from these. Without them the pick chooses."`
}

// CouchTvTemplate is the channel's week.
type CouchTvTemplate struct {
	Slots []CouchTvSlot `json:"slots" minItems:"1" maxItems:"200"`
}

// CouchTvRotation are the knobs of the repetition rules.
type CouchTvRotation struct {
	PrimeRepeatMinDays int     `json:"primeRepeatMinDays" minimum:"0" maximum:"365" doc:"A movie returns to prime time after max(this, primeRepeatFactor x the movie count) days."`
	PrimeRepeatFactor  float64 `json:"primeRepeatFactor" minimum:"0" maximum:"5"`
	MovieGapDays       int     `json:"movieGapDays" minimum:"0" maximum:"365" doc:"Days before a movie reruns outside prime time."`
	RerunGapDays       int     `json:"rerunGapDays" minimum:"0" maximum:"365" doc:"Days before an episode reruns."`
	PremiereWindowDays int     `json:"premiereWindowDays" minimum:"0" maximum:"365" doc:"How recently a movie must have been added to premiere."`
	FillerMinMinutes   int     `json:"fillerMinMinutes" minimum:"1" maximum:"240" doc:"A gap this long before a fixed time is filled with reruns where one fits; shorter gaps grow the break."`
}

// CouchTvGuestLink is the channel link for viewers without an account (D23). The
// code itself is created by a later phase; only its hash is ever stored.
type CouchTvGuestLink struct {
	Enabled bool `json:"enabled"`
}

// CouchTvConfig is the channel's whole tunable surface (settings key "couchtv").
type CouchTvConfig struct {
	TimeZone        string           `json:"timeZone" minLength:"1" doc:"IANA time zone the template's times are in."`
	BreakMinSeconds int              `json:"breakMinSeconds" minimum:"5" maximum:"600" doc:"The shortest break between programs."`
	Template        CouchTvTemplate  `json:"template"`
	Rotation        CouchTvRotation  `json:"rotation"`
	GuestLink       CouchTvGuestLink `json:"guestLink"`
}

// storedConfig is the settings value: the config plus what the API never shows.
type storedConfig struct {
	CouchTvConfig
	GuestLinkHash string `json:"guestLinkHash,omitempty"`
}

// DefaultConfig is the shipped channel, and the fallback for any part of the stored
// value that is absent or no longer valid.
func DefaultConfig() CouchTvConfig {
	return CouchTvConfig{
		TimeZone:        "Europe/Prague",
		BreakMinSeconds: 23,
		Template:        DefaultTemplate(),
		Rotation: CouchTvRotation{
			PrimeRepeatMinDays: 5,
			PrimeRepeatFactor:  0.8,
			MovieGapDays:       2,
			RerunGapDays:       14,
			PremiereWindowDays: 28,
			FillerMinMinutes:   5,
		},
	}
}

// DefaultTemplate is plan 5.1 written as rules, so it suits any library: strips go to
// the series with the most episodes of their genre (each series once, specific genres
// before "any"), the marathon to the biggest drama, reruns draw by genre. Pinning
// titles on a slot turns it into the owner's own lineup.
func DefaultTemplate() CouchTvTemplate {
	all := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	weekdays := []string{"mon", "tue", "wed", "thu", "fri"}
	return CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "morning", Days: all, Start: "06:00", Anchor: true, Fill: FillReruns, Pick: PickComedy},
		{Key: "daytime", Days: all, Start: "09:00", Fill: FillReruns, Pick: PickAny},
		{Key: "matinee", Days: []string{"mon", "tue", "wed", "thu", "fri", "sat"}, Start: "12:00", Anchor: true, Fill: FillMatinee, Pick: PickAny},
		{Key: "family-movie", Days: []string{"sun"}, Start: "12:00", Anchor: true, Fill: FillMatinee, Pick: PickFamily},
		{Key: "marathon", Days: []string{"sat", "sun"}, Fill: FillMarathon, Pick: PickDrama, Count: 4},
		{Key: "afternoon", Days: all, Fill: FillReruns, Pick: PickAny},
		{Key: "early-strip", Days: all, Start: "17:00", Anchor: true, Fill: FillStrip, Pick: PickAny, Count: 1},
		{Key: "sitcom-strip", Days: all, Start: "18:00", Anchor: true, Fill: FillStrip, Pick: PickComedy, Count: 2},
		{Key: "second-sitcom-strip", Days: all, Fill: FillStrip, Pick: PickComedy, Count: 2},
		{Key: "prime-movie", Days: weekdays, Start: "20:15", Anchor: true, Fill: FillMovie, Pick: PickAny},
		{Key: "premiere", Days: []string{"sat"}, Start: "20:15", Anchor: true, Fill: FillPremiere, Pick: PickAny},
		{Key: "saga", Days: []string{"sun"}, Start: "20:15", Anchor: true, Fill: FillSaga, Pick: PickAny},
		{Key: "drama-strip", Days: []string{"mon", "tue", "wed", "thu"}, Fill: FillStrip, Pick: PickDrama, Count: 1},
		{Key: "late-strip", Days: []string{"fri", "sun"}, Fill: FillStrip, Pick: PickAny, Count: 1},
		{Key: "late-movie", Days: []string{"sat"}, Fill: FillMatinee, Pick: PickAny},
		{Key: "late", Days: all, Fill: FillReruns, Pick: PickAnimation},
		{Key: "encore", Days: all, Start: "01:00", Fill: FillEncore, Pick: PickAny},
		{Key: "overnight", Days: all, Fill: FillReruns, Pick: PickAny},
	}}
}

// LoadConfig reads the stored config, falling back to the defaults part by part so a
// partial or outdated value cannot take the channel off the air.
func LoadConfig(ctx context.Context, st *settings.Store) CouchTvConfig {
	return loadStored(ctx, st).CouchTvConfig
}

func loadStored(ctx context.Context, st *settings.Store) storedConfig {
	out := storedConfig{CouchTvConfig: DefaultConfig()}
	if st == nil {
		return out
	}
	raw, err := st.Get(ctx, settingsKey)
	if err != nil || raw == nil {
		return out
	}
	var stored storedConfig
	if json.Unmarshal(raw, &stored) != nil {
		return out
	}
	return storedConfig{CouchTvConfig: stored.withDefaults(), GuestLinkHash: stored.GuestLinkHash}
}

// withDefaults replaces every part that is unset or invalid with the shipped one.
func (c CouchTvConfig) withDefaults() CouchTvConfig {
	def := DefaultConfig()
	if _, err := time.LoadLocation(c.TimeZone); err != nil || c.TimeZone == "" {
		c.TimeZone = def.TimeZone
	}
	if c.BreakMinSeconds < 5 || c.BreakMinSeconds > 600 {
		c.BreakMinSeconds = def.BreakMinSeconds
	}
	if c.Rotation == (CouchTvRotation{}) || c.Rotation.validate() != nil {
		c.Rotation = def.Rotation
	}
	if c.Template.validate() != nil {
		c.Template = def.Template
	}
	return c
}

// Validate rejects a config the planner could not follow.
func (c CouchTvConfig) Validate() error {
	if _, err := time.LoadLocation(c.TimeZone); err != nil || c.TimeZone == "" {
		return fmt.Errorf("unknown time zone %q", c.TimeZone)
	}
	if c.BreakMinSeconds < 5 || c.BreakMinSeconds > 600 {
		return errors.New("breakMinSeconds must be between 5 and 600")
	}
	if err := c.Rotation.validate(); err != nil {
		return err
	}
	return c.Template.validate()
}

func (r CouchTvRotation) validate() error {
	for _, v := range []struct {
		name  string
		value int
	}{
		{"primeRepeatMinDays", r.PrimeRepeatMinDays}, {"movieGapDays", r.MovieGapDays},
		{"rerunGapDays", r.RerunGapDays}, {"premiereWindowDays", r.PremiereWindowDays},
	} {
		if v.value < 0 || v.value > 365 {
			return fmt.Errorf("%s must be between 0 and 365", v.name)
		}
	}
	if r.PrimeRepeatFactor < 0 || r.PrimeRepeatFactor > 5 {
		return errors.New("primeRepeatFactor must be between 0 and 5")
	}
	if r.FillerMinMinutes < 1 || r.FillerMinMinutes > 240 {
		return errors.New("fillerMinMinutes must be between 1 and 240")
	}
	return nil
}

var fills = []string{FillStrip, FillMarathon, FillReruns, FillMovie, FillMatinee, FillPremiere, FillSaga, FillEncore}
var picks = []string{"", PickAny, PickComedy, PickAnimation, PickDrama, PickFamily}

// validate checks the slots and that every day of the week is a well-formed
// broadcast day: it opens with an anchor, its times ascend through the night, and
// every reruns block has a time to fill up to.
func (t CouchTvTemplate) validate() error {
	if len(t.Slots) == 0 {
		return errors.New("the template has no slots")
	}
	keys := map[string]bool{}
	for _, s := range t.Slots {
		if s.Key == "" {
			return errors.New("every slot needs a key")
		}
		if keys[s.Key] {
			return fmt.Errorf("slot %q appears twice", s.Key)
		}
		keys[s.Key] = true
		if len(s.Days) == 0 {
			return fmt.Errorf("slot %q has no days", s.Key)
		}
		for _, d := range s.Days {
			if !slices.Contains(dayNames, d) {
				return fmt.Errorf("slot %q: unknown day %q", s.Key, d)
			}
		}
		if !slices.Contains(fills, s.Fill) {
			return fmt.Errorf("slot %q: unknown fill %q", s.Key, s.Fill)
		}
		if !slices.Contains(picks, s.Pick) {
			return fmt.Errorf("slot %q: unknown pick %q", s.Key, s.Pick)
		}
		if s.Start != "" {
			if _, err := clockMinutes(s.Start); err != nil {
				return fmt.Errorf("slot %q: %w", s.Key, err)
			}
		}
		if s.Anchor && s.Start == "" {
			return fmt.Errorf("slot %q: an anchor needs a start time", s.Key)
		}
		if s.Count < 0 || s.Count > 20 {
			return fmt.Errorf("slot %q: count must be between 0 and 20", s.Key)
		}
		if (s.Fill == FillStrip || s.Fill == FillMarathon) && len(s.Titles) > 1 {
			return fmt.Errorf("slot %q: a %s pins one series", s.Key, s.Fill)
		}
		for _, id := range s.Titles {
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("slot %q: %q is not a title id", s.Key, id)
			}
		}
	}
	for wd := range dayNames {
		if err := t.validateDay(time.Weekday(wd)); err != nil {
			return err
		}
	}
	return nil
}

func (t CouchTvTemplate) validateDay(wd time.Weekday) error {
	day := t.slotsOn(wd)
	name := dayNames[wd]
	if len(day) == 0 {
		return fmt.Errorf("%s has no slots", name)
	}
	if !day[0].Anchor {
		return fmt.Errorf("%s: the first slot (%q) must be an anchor", name, day[0].Key)
	}
	first, _ := clockMinutes(day[0].Start)
	last := -1
	for i, s := range day {
		if s.Start != "" {
			m, _ := clockMinutes(s.Start)
			at := (m - first + 24*60) % (24 * 60)
			if at <= last {
				return fmt.Errorf("%s: slot %q at %s is not after the slot before it", name, s.Key, s.Start)
			}
			last = at
		}
		if s.Fill == FillReruns && i+1 < len(day) && !slices.ContainsFunc(day[i+1:], func(n CouchTvSlot) bool { return n.Start != "" }) {
			return fmt.Errorf("%s: reruns slot %q needs a timed slot after it, or to end the day", name, s.Key)
		}
	}
	return nil
}

// slotsOn lists the slots that air on a weekday, in template order.
func (t CouchTvTemplate) slotsOn(wd time.Weekday) []CouchTvSlot {
	var out []CouchTvSlot
	for _, s := range t.Slots {
		if slices.Contains(s.Days, dayNames[wd]) {
			out = append(out, s)
		}
	}
	return out
}

// clockMinutes parses "HH:MM" into minutes after midnight.
func clockMinutes(hhmm string) (int, error) {
	if len(hhmm) != 5 || hhmm[2] != ':' {
		return 0, fmt.Errorf("time %q is not HH:MM", hhmm)
	}
	h, err1 := strconv.Atoi(hhmm[:2])
	m, err2 := strconv.Atoi(hhmm[3:])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q is not HH:MM", hhmm)
	}
	return h*60 + m, nil
}

// location is the channel's time zone; Validate guarantees it loads.
func (c CouchTvConfig) location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}
