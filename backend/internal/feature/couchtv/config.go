package couchtv

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

// CouchTvGuestLink is the channel link for viewers without an account (D23). Only
// the code's hash is stored, so the code is shown once, by adminRotateCouchTvLink;
// that and adminSetCouchTvLink change the link, adminUpdateCouchTv keeps it.
type CouchTvGuestLink struct {
	Enabled   bool       `json:"enabled" doc:"Guests can join through the link."`
	CreatedAt *time.Time `json:"createdAt,omitempty" doc:"When the current link was made; absent before the first one."`
}

// CouchTvCity is a place the weather bulletin covers.
type CouchTvCity struct {
	Name string  `json:"name" minLength:"1" maxLength:"40"`
	Lat  float64 `json:"lat" minimum:"48.5" maximum:"51.1" doc:"Latitude; within Czechia, the map the weather is drawn on."`
	Lon  float64 `json:"lon" minimum:"12" maximum:"18.9" doc:"Longitude; within Czechia."`
}

// CouchTvBulletinsConfig sets up the bulletins (plan 6.10): the weather and the news
// headlines the couchtv_bulletins job keeps fresh for the breaks.
type CouchTvBulletinsConfig struct {
	Enabled bool          `json:"enabled" doc:"Fetch the weather and the news headlines through the day."`
	Cities  []CouchTvCity `json:"cities" minItems:"1" maxItems:"20" doc:"The places the weather covers, each with its own forecast, in the order they are shown."`
}

// CouchTvConfig is the channel's whole tunable surface (settings key "couchtv").
type CouchTvConfig struct {
	TimeZone        string                 `json:"timeZone" minLength:"1" doc:"IANA time zone the template's times are in."`
	BreakMinSeconds int                    `json:"breakMinSeconds" minimum:"5" maximum:"600" doc:"The shortest break between programs."`
	Template        CouchTvTemplate        `json:"template"`
	Rotation        CouchTvRotation        `json:"rotation"`
	GuestLink       CouchTvGuestLink       `json:"guestLink"`
	Bulletins       CouchTvBulletinsConfig `json:"bulletins"`
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
		Bulletins: CouchTvBulletinsConfig{Enabled: true, Cities: DefaultCities()},
	}
}

// DefaultCities are the regional capitals (Prague seats Středočeský kraj too), at
// GeoNames' coordinates, Prague by its English name (the weather is English, D13) and the
// rest by their own. Pardubice is left out: 19 km from Hradec Králové, its label
// would collide with that one on a map of Czechia.
func DefaultCities() []CouchTvCity {
	return []CouchTvCity{
		{Name: "Prague", Lat: 50.0880, Lon: 14.4208},
		{Name: "Brno", Lat: 49.1952, Lon: 16.6080},
		{Name: "Ostrava", Lat: 49.8347, Lon: 18.2820},
		{Name: "Plzeň", Lat: 49.7475, Lon: 13.3776},
		{Name: "Liberec", Lat: 50.7671, Lon: 15.0562},
		{Name: "Olomouc", Lat: 49.5955, Lon: 17.2518},
		{Name: "České Budějovice", Lat: 48.9745, Lon: 14.4743},
		{Name: "Hradec Králové", Lat: 50.2092, Lon: 15.8328},
		{Name: "Ústí nad Labem", Lat: 50.6607, Lon: 14.0323},
		{Name: "Zlín", Lat: 49.2265, Lon: 17.6707},
		{Name: "Jihlava", Lat: 49.3961, Lon: 15.5912},
		{Name: "Karlovy Vary", Lat: 50.2327, Lon: 12.8712},
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
	// a value stored before the bulletins existed has no cities, and gets them switched on
	if c.Bulletins.validate() != nil {
		c.Bulletins = def.Bulletins
	}
	return c
}

// ConfigError is why the planner could not follow a config. Code is stable for clients,
// which say it in the display language; Message (English) names the slot or day.
type ConfigError struct {
	Code    string
	Message string
}

func (e *ConfigError) Error() string { return e.Message }

// Codes of ConfigError.
const (
	ErrTimeZone    = "invalid_time_zone"
	ErrRotation    = "invalid_rotation"     // breakMinSeconds or a rotation knob out of range
	ErrSlot        = "invalid_slot"         // a slot's key, days, fill, pick, time, count or pinned ids
	ErrAnchorTime  = "anchor_needs_time"    // an anchor without a start
	ErrDayEmpty    = "day_without_slots"    // a weekday no slot airs on
	ErrDayOpen     = "day_needs_anchor"     // a day whose first slot is not an anchor
	ErrSlotOrder   = "slot_out_of_order"    // times that do not ascend through the day
	ErrRerunsEnd   = "reruns_without_end"   // a reruns slot with nothing timed after it
	ErrPinnedTitle = "invalid_pinned_title" // a pinned title that is gone or does not suit its slot
	ErrBulletins   = "invalid_bulletins"    // no city, too many, a nameless or repeated one, or one outside Czechia
)

func configError(code, format string, args ...any) error {
	return &ConfigError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Validate rejects a config the planner could not follow, with a *ConfigError.
func (c CouchTvConfig) Validate() error {
	if _, err := time.LoadLocation(c.TimeZone); err != nil || c.TimeZone == "" {
		return configError(ErrTimeZone, "unknown time zone %q", c.TimeZone)
	}
	if c.BreakMinSeconds < 5 || c.BreakMinSeconds > 600 {
		return configError(ErrRotation, "breakMinSeconds must be between 5 and 600")
	}
	if err := c.Rotation.validate(); err != nil {
		return err
	}
	if err := c.Bulletins.validate(); err != nil {
		return err
	}
	return c.Template.validate()
}

// The bulletins' limits: the weather is drawn on a map of Czechia, and each city is a
// location Open-Meteo counts against its free tier on every run.
const (
	maxCities      = 20
	maxCityName    = 40
	minLat, maxLat = 48.5, 51.1
	minLon, maxLon = 12.0, 18.9
)

func (b CouchTvBulletinsConfig) validate() error {
	if len(b.Cities) == 0 || len(b.Cities) > maxCities {
		return configError(ErrBulletins, "the weather needs 1 to %d cities", maxCities)
	}
	names := map[string]bool{}
	for _, c := range b.Cities {
		name := strings.TrimSpace(c.Name)
		if name == "" || utf8.RuneCountInString(name) > maxCityName {
			return configError(ErrBulletins, "every city needs a name of up to %d characters", maxCityName)
		}
		if names[name] {
			return configError(ErrBulletins, "city %q appears twice", name)
		}
		names[name] = true
		if c.Lat < minLat || c.Lat > maxLat || c.Lon < minLon || c.Lon > maxLon {
			return configError(ErrBulletins, "city %q lies outside Czechia", name)
		}
	}
	return nil
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
			return configError(ErrRotation, "%s must be between 0 and 365", v.name)
		}
	}
	if r.PrimeRepeatFactor < 0 || r.PrimeRepeatFactor > 5 {
		return configError(ErrRotation, "primeRepeatFactor must be between 0 and 5")
	}
	if r.FillerMinMinutes < 1 || r.FillerMinMinutes > 240 {
		return configError(ErrRotation, "fillerMinMinutes must be between 1 and 240")
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
		return configError(ErrDayEmpty, "the template has no slots")
	}
	keys := map[string]bool{}
	for _, s := range t.Slots {
		if s.Key == "" {
			return configError(ErrSlot, "every slot needs a key")
		}
		if keys[s.Key] {
			return configError(ErrSlot, "slot %q appears twice", s.Key)
		}
		keys[s.Key] = true
		if len(s.Days) == 0 {
			return configError(ErrSlot, "slot %q has no days", s.Key)
		}
		for _, d := range s.Days {
			if !slices.Contains(dayNames, d) {
				return configError(ErrSlot, "slot %q: unknown day %q", s.Key, d)
			}
		}
		if !slices.Contains(fills, s.Fill) {
			return configError(ErrSlot, "slot %q: unknown fill %q", s.Key, s.Fill)
		}
		if !slices.Contains(picks, s.Pick) {
			return configError(ErrSlot, "slot %q: unknown pick %q", s.Key, s.Pick)
		}
		if s.Start != "" {
			if _, err := clockMinutes(s.Start); err != nil {
				return configError(ErrSlot, "slot %q: %v", s.Key, err)
			}
		}
		if s.Anchor && s.Start == "" {
			return configError(ErrAnchorTime, "slot %q: an anchor needs a start time", s.Key)
		}
		if s.Count < 0 || s.Count > 20 {
			return configError(ErrSlot, "slot %q: count must be between 0 and 20", s.Key)
		}
		if (s.Fill == FillStrip || s.Fill == FillMarathon) && len(s.Titles) > 1 {
			return configError(ErrPinnedTitle, "slot %q: a %s pins one series", s.Key, s.Fill)
		}
		for _, id := range s.Titles {
			if _, err := uuid.Parse(id); err != nil {
				return configError(ErrPinnedTitle, "slot %q: %q is not a title id", s.Key, id)
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
		return configError(ErrDayEmpty, "%s has no slots", name)
	}
	if !day[0].Anchor {
		return configError(ErrDayOpen, "%s: the first slot (%q) must be an anchor", name, day[0].Key)
	}
	first, _ := clockMinutes(day[0].Start)
	last := -1
	for i, s := range day {
		if s.Start != "" {
			m, _ := clockMinutes(s.Start)
			at := (m - first + 24*60) % (24 * 60)
			if at <= last {
				return configError(ErrSlotOrder, "%s: slot %q at %s is not after the slot before it", name, s.Key, s.Start)
			}
			last = at
		}
		if s.Fill == FillReruns && i+1 < len(day) && !slices.ContainsFunc(day[i+1:], func(n CouchTvSlot) bool { return n.Start != "" }) {
			return configError(ErrRerunsEnd, "%s: reruns slot %q needs a timed slot after it, or to end the day", name, s.Key)
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
