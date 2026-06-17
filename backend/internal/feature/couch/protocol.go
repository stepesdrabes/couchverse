package couch

import "encoding/json"

// Envelope is the single WebSocket message shape in both directions; Type
// discriminates and Data carries the typed payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

const (
	// server -> client
	msgHello        = "hello"         // full snapshot on connect
	msgHostState    = "host_state"    // authoritative play-state (also the client->server command)
	msgParticipants = "participants"  // membership changed
	msgMediaChanged = "media_changed" // host switched title/episode (followers re-fetch their payload)
	msgHostAway     = "host_away"     // host lost all connections; grace countdown started
	msgHostReturned = "host_returned" // host reconnected within grace
	msgEmoji        = "emoji"         // relayed reaction
	msgPaused       = "paused"        // a follower paused/unpaused locally (client->server)
	msgSessionEnded = "session_ended" // terminal

	// client -> server (host_state and emoji reuse the strings above)
)

type helloData struct {
	SessionID       string        `json:"sessionId"`
	MyParticipantID string        `json:"myParticipantId"`
	Role            string        `json:"role"`
	State           hostState     `json:"state"`
	Participants    []participant `json:"participants"`
	ServerTimeMs    int64         `json:"serverTime"`
}

type participantsData struct {
	Participants []participant `json:"participants"`
}

type mediaChangedData struct {
	Media mediaRef `json:"media"`
	Seq   uint64   `json:"seq"`
}

type hostAwayData struct {
	GraceSeconds int `json:"graceSeconds"`
}

type emojiData struct {
	FromParticipantID string `json:"fromParticipantId"`
	Emoji             string `json:"emoji"`
}

type sessionEndedData struct {
	Reason string `json:"reason"`
}

// client -> server commands

type hostStateCmd struct {
	Media           mediaRef `json:"media"`
	Playing         bool     `json:"playing"`
	PositionSeconds float64  `json:"positionSeconds"`
}

type emojiCmd struct {
	Emoji string `json:"emoji"`
}

type pausedCmd struct {
	Paused bool `json:"paused"`
}

// mustEnvelope marshals a typed payload into a wire frame. Marshalling failures
// yield a frame with no data rather than panicking the fan-out.
func mustEnvelope(typ string, data any) []byte {
	env := Envelope{Type: typ}
	if data != nil {
		if raw, err := json.Marshal(data); err == nil {
			env.Data = raw
		}
	}
	b, _ := json.Marshal(env)
	return b
}
