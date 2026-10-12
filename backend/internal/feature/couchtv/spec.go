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

// SpecTitle is a title as the templates show it.
type SpecTitle struct {
	Name           string   `json:"name"`
	Year           int      `json:"year,omitempty"`
	RuntimeMinutes int      `json:"runtimeMinutes,omitempty"`
	Genres         []string `json:"genres"`
	Accents        Accents  `json:"accents"`
	Backdrop       string   `json:"backdrop"`
	Poster         string   `json:"poster"`
	Taglines       []string `json:"taglines,omitempty"`
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

// Spec is one clip's spec (spec.ts ClipSpec): Template says which fields apply.
type Spec struct {
	Template string         `json:"template"`
	Cut      string         `json:"cut,omitempty"`
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
	Airing   *SpecAiring    `json:"airing,omitempty"`
	Foot     *StripFoot     `json:"foot,omitempty"`
	Items    []LineupItem   `json:"items,omitempty"`
	Ending   *SpecTitle     `json:"ending,omitempty"`
	Program  *ProgramSource `json:"program,omitempty"`
	Next     *SqueezeNext   `json:"next,omitempty"`
	Handover string         `json:"handover,omitempty"`
	Bed      *Bed           `json:"bed,omitempty"`
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
