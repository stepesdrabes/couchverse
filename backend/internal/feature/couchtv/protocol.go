package couchtv

import (
	"encoding/json"

	"couchverse/internal/httpx"
)

// The Couch TV WebSocket protocol (GET /tv/socket): presence, reactions and the
// viewers' reports of what they watch. Every frame is an Envelope in both
// directions; the frame tables below are the protocol's single source -
// `couchverse couchtv-schema` publishes them as contract/couchtv-protocol.schema.json.

// Envelope is the single WebSocket message shape in both directions; Type
// discriminates and Data carries the typed payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

const (
	msgHello           = "hello"
	msgPresence        = "presence"
	msgReaction        = "reaction"
	msgScheduleChanged = "schedule_changed"
	msgReport          = "report"
)

// ServerFrames are the messages the server sends.
var ServerFrames = []httpx.Frame{
	{Type: msgHello, Payload: CouchTvHello{}, Doc: "Sent on connect: the server clock, who this viewer is and who is watching."},
	{Type: msgPresence, Payload: CouchTvPresence{}, Doc: "Who is watching changed."},
	{Type: msgReaction, Payload: CouchTvReaction{}, Doc: "A relayed reaction, the sender's own included."},
	{Type: msgScheduleChanged, Payload: nil, Doc: "The schedule changed (a replan, an admin's edit or a swap): tune in again for what is on and next, and read the guide again."},
}

// ClientFrames are the messages clients send.
var ClientFrames = []httpx.Frame{
	{Type: msgReaction, Payload: CouchTvReactionCommand{}, Doc: "Send a reaction; a viewer sending them faster than about three a second has the extra ones dropped."},
	{Type: msgReport, Payload: CouchTvReport{}, Doc: "About every 10 s while the picture plays: the program date on screen, which counts toward progress and watch time."},
}

// CouchTvViewer is what everyone sees of one viewer.
type CouchTvViewer struct {
	ID          string  `json:"id" doc:"Stable while the viewer is signed in or holds their guest cookie."`
	DisplayName string  `json:"displayName" doc:"The account's name, or the name a guest typed."`
	AvatarID    *string `json:"avatarId,omitempty"`
	Seed        string  `json:"seed" doc:"Identicon seed: the username, or the guest's id."`
	Guest       bool    `json:"guest" doc:"Joined through the channel link without an account."`
}

// CouchTvPresence is who is watching.
type CouchTvPresence struct {
	Count   int             `json:"count" doc:"Everyone watching, each viewer once however many devices they watch on."`
	Viewers []CouchTvViewer `json:"viewers" doc:"The first viewers to tune in, at most 24."`
}

type CouchTvHello struct {
	ServerTime int64           `json:"serverTime" doc:"The server clock when the frame was made, in Unix milliseconds: players hold the program date three target durations behind it."`
	Me         CouchTvViewer   `json:"me"`
	Presence   CouchTvPresence `json:"presence"`
}

type CouchTvReaction struct {
	ViewerID string `json:"viewerId"`
	Emoji    string `json:"emoji"`
}

type CouchTvReactionCommand struct {
	Emoji string `json:"emoji"`
}

type CouchTvReport struct {
	ProgramDate int64 `json:"programDate" doc:"The program date of the picture on screen, in Unix milliseconds."`
}

// envelope marshals a typed payload into a wire frame. Marshalling failures yield
// a frame with no data rather than panicking the fan-out.
func envelope(typ string, data any) []byte {
	env := Envelope{Type: typ}
	if data != nil {
		if raw, err := json.Marshal(data); err == nil {
			env.Data = raw
		}
	}
	b, _ := json.Marshal(env)
	return b
}
