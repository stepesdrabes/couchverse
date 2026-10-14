package couchtv

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The broadcast specs the server writes. clients/broadcast/src/spec.ts is the contract and
// broadcast-spec.schema.json (copied from contract/ by `make contract`) its schema, which
// every spec is checked against before it is stored. Only what the composer sets is here.

// FPS is the channel's frame rate (D16); every clip and break is a whole number of frames.
const FPS = 25

// frameSeconds converts frames to the seconds a spec carries.
func frameSeconds(frames int) float64 { return float64(frames) / FPS }

// framesOf converts seconds to whole frames.
func framesOf(s float64) int { return int(math.Round(s * FPS)) }

// Crop is a normalized `[x, y, width, height]` of an image.
type Crop [4]float64

// Bed is an excerpt of a soundtrack a template plays under itself.
type Bed struct {
	URL string  `json:"url"`
	In  float64 `json:"in"`
	Out float64 `json:"out"`
}

// Accents are the artwork accents that colour a template.
type Accents struct {
	Backdrop string `json:"backdrop,omitempty"`
	Poster   string `json:"poster,omitempty"`
}

// SpecLogo is a title's logo: a PNG trimmed to its letters, and its width over its height.
type SpecLogo struct {
	URL    string  `json:"url"`
	Aspect float64 `json:"aspect"`
}

// SpecTitle is a title as the templates show it. Footage is its analysed shots, for the
// screens that show it in motion.
type SpecTitle struct {
	Name           string    `json:"name"`
	Logo           *SpecLogo `json:"logo,omitempty"`
	Footage        []Shot    `json:"footage,omitempty"`
	Year           int       `json:"year,omitempty"`
	RuntimeMinutes int       `json:"runtimeMinutes,omitempty"`
	Genres         []string  `json:"genres"`
	Accents        Accents   `json:"accents"`
	Backdrop       string    `json:"backdrop"`
	Poster         string    `json:"poster"`
	Taglines       []string  `json:"taglines,omitempty"`
}

// SpecEpisode is an episode line (`S01E02`, its name).
type SpecEpisode struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	RuntimeMinutes int    `json:"runtimeMinutes,omitempty"`
}

// Shot is a promo shot: footage frames, or a still crop of the title's artwork.
type Shot struct {
	Kind   string   `json:"kind"`
	Frames []string `json:"frames,omitempty"`
	Image  string   `json:"image,omitempty"`
	From   *Crop    `json:"from,omitempty"`
	To     *Crop    `json:"to,omitempty"`
}

// SpecAiring is when something airs, as an English phrase. Days is only for a strip promo.
type SpecAiring struct {
	Eyebrow string `json:"eyebrow"`
	Day     string `json:"day,omitempty"`
	Time    string `json:"time"`
	Days    []int  `json:"days,omitempty"`
}

// StripFoot is the line under a strip promo's days.
type StripFoot struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// LineupItem is one row of the lineup card.
type LineupItem struct {
	Time  string    `json:"time"`
	Title SpecTitle `json:"title"`
	Sub   string    `json:"sub"`
	State string    `json:"state,omitempty"`
	Badge string    `json:"badge,omitempty"`
}

// ProgramSource fills the squeeze's program panel: frames, or the ending title's backdrop
// (standIn) with an empty credits roll.
type ProgramSource struct {
	Kind    string      `json:"kind"`
	Frames  []string    `json:"frames,omitempty"`
	Credits *[][]string `json:"credits,omitempty"`
}

// SqueezeNext is what the squeeze announces.
type SqueezeNext struct {
	Title   SpecTitle    `json:"title"`
	Episode *SpecEpisode `json:"episode,omitempty"`
	Time    string       `json:"time"`
}

// SpecTime is a moment a bulletin names: a clock time, with the English weekday when it is
// another day than the bulletin's.
type SpecTime struct {
	Day  string `json:"day,omitempty"`
	Time string `json:"time"`
}

// SpecCity is one city of the weather bulletin.
type SpecCity struct {
	Name string           `json:"name"`
	Lat  float64          `json:"lat"`
	Lon  float64          `json:"lon"`
	Now  SpecWeatherNow   `json:"now"`
	Days []SpecWeatherDay `json:"days"`
}

// SpecWeatherNow is a city's weather at the bulletin's time, in whole degrees.
type SpecWeatherNow struct {
	Temperature int  `json:"temperature"`
	Code        int  `json:"code"`
	Day         bool `json:"day"`
}

// SpecWeatherDay is one day of a city's forecast.
type SpecWeatherDay struct {
	Label         string `json:"label"`
	Weekday       string `json:"weekday"`
	Short         string `json:"short"`
	Min           int    `json:"min"`
	Max           int    `json:"max"`
	Code          int    `json:"code"`
	Precipitation *int   `json:"precipitation,omitempty"`
}

// SpecWarning is one of ČHMÚ's warnings, in English.
type SpecWarning struct {
	Level   string       `json:"level"`
	Event   string       `json:"event"`
	Active  bool         `json:"active"`
	From    SpecTime     `json:"from"`
	Until   *SpecTime    `json:"until,omitempty"`
	Regions []SpecRegion `json:"regions"`
}

// SpecRegion is a kraj a warning covers.
type SpecRegion struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Partial bool   `json:"partial"`
}

// SpecCredits are the weather's credits, each shown with its data.
type SpecCredits struct {
	Forecast string `json:"forecast"`
	Warnings string `json:"warnings"`
	Borders  string `json:"borders"`
}

// SpecStory is one story of the news bulletin.
type SpecStory struct {
	Headline string `json:"headline"`
	Outlet   string `json:"outlet"`
	Outlets  int    `json:"outlets"`
	Trending bool   `json:"trending,omitempty"`
}

// Spec is one clip's spec (spec.ts ClipSpec): Template says which fields apply.
type Spec struct {
	Template string         `json:"template"`
	Cut      string         `json:"cut,omitempty"`
	Design   string         `json:"design,omitempty"`
	Duration float64        `json:"duration,omitempty"`
	Part     string         `json:"part,omitempty"`
	Mood     string         `json:"mood,omitempty"`
	Premiere bool           `json:"premiere,omitempty"`
	Title    *SpecTitle     `json:"title,omitempty"`
	Episode  *SpecEpisode   `json:"episode,omitempty"`
	Time     string         `json:"time,omitempty"`
	Clock    string         `json:"clock,omitempty"`
	Enter    string         `json:"enter,omitempty"`
	Shots    []Shot         `json:"shots,omitempty"`
	Scenes   []SpecScene    `json:"scenes,omitempty"`
	Words    []string       `json:"words,omitempty"`
	Airing   *SpecAiring    `json:"airing,omitempty"`
	Foot     *StripFoot     `json:"foot,omitempty"`
	Items    []LineupItem   `json:"items,omitempty"`
	Ending   *SpecTitle     `json:"ending,omitempty"`
	Program  *ProgramSource `json:"program,omitempty"`
	Next     *SqueezeNext   `json:"next,omitempty"`
	Handover string         `json:"handover,omitempty"`
	Bed      *Bed           `json:"bed,omitempty"`
	// the bulletins': when the data is of, the weather's cities, warnings (empty on a quiet
	// day, nil while ČHMÚ is unread) and credits, the news' stories and credit line
	AsOf     string         `json:"asOf,omitempty"`
	Cities   []SpecCity     `json:"cities,omitempty"`
	Warnings *[]SpecWarning `json:"warnings,omitempty"`
	Credits  *SpecCredits   `json:"credits,omitempty"`
	Stories  []SpecStory    `json:"stories,omitempty"`
	Credit   string         `json:"credit,omitempty"`
}

// BreakItem places a clip's sound in a break.
type BreakItem struct {
	At   float64 `json:"at"`
	Spec Spec    `json:"spec"`
}

// BreakSpec is a whole break's sound in one render.
type BreakSpec struct {
	Template string      `json:"template"`
	Loudness float64     `json:"loudness"`
	Items    []BreakItem `json:"items"`
}

//go:embed broadcast-spec.schema.json
var specSchemaJSON []byte

var specSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(specSchemaJSON))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("broadcast-spec.schema.json", doc); err != nil {
		return nil, err
	}
	return c.Compile("broadcast-spec.schema.json")
})

// validateSpec checks a clip or break spec against the broadcast schema.
func validateSpec(spec any) error {
	schema, err := specSchema()
	if err != nil {
		return err
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return schema.Validate(inst)
}

// clipKey is a clip's cache key: its template's version and its spec, whose media URLs
// carry their inputs' versions (artwork changes, the extraction recipe). Equal keys
// render the same picture and sound.
func clipKey(version int, spec Spec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s:%d:%s", spec.Template, version, data))
	return hex.EncodeToString(sum[:16]), nil
}
