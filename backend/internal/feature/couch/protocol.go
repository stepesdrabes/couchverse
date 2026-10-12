package couch

import (
	"encoding/json"

	"couchverse/internal/httpx"
)

// The couch WebSocket protocol (GET /couch/{token}/ws). Every frame is an
// Envelope in both directions; the frame tables below are the protocol's single
// source - `couchverse couch-schema` publishes them as
// contract/couch-protocol.schema.json for the clients.

// Envelope is the single WebSocket message shape in both directions; Type
// discriminates and Data carries the typed payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

const (
	msgHello        = "hello"
	msgHostState    = "host_state"
	msgParticipants = "participants"
	msgMediaChanged = "media_changed"
	msgHostAway     = "host_away"
	msgHostReturned = "host_returned"
	msgEmoji        = "emoji"
	msgPaused       = "paused"
	msgSessionEnded = "session_ended"
	msgRemote       = "remote_command"
)

// Connection roles: the host's playing device, a follower, or the host's account on another
// device steering the host's player.
const (
	roleHost     = "host"
	roleFollower = "follower"
	roleRemote   = "remote"
)

// ServerFrames are the messages the server sends.
var ServerFrames = []httpx.Frame{
	{Type: msgHello, Payload: CouchHello{}, Doc: "Full snapshot, sent on connect and again when the connection's role changes: the host's account started hosting on another device, so this one is its remote now."},
	{Type: msgHostState, Payload: CouchHostState{}, Doc: "The authoritative play state, stamped by the server."},
	{Type: msgParticipants, Payload: CouchParticipants{}, Doc: "Membership or a follower's local pause changed."},
	{Type: msgMediaChanged, Payload: CouchMediaChanged{}, Doc: "The host switched title or episode; followers fetch their playback payload again."},
	{Type: msgHostAway, Payload: CouchHostAway{}, Doc: "The host lost every connection; the grace countdown started."},
	{Type: msgHostReturned, Payload: nil, Doc: "The host reconnected within the grace period."},
	{Type: msgEmoji, Payload: CouchEmoji{}, Doc: "A relayed reaction."},
	{Type: msgSessionEnded, Payload: CouchSessionEnded{}, Doc: "Terminal: the session is over and the socket closes."},
	{Type: msgRemote, Payload: CouchRemoteCommand{}, Doc: "To the host's playing connection only: a remote asks for a change, which the host applies and broadcasts as host_state."},
}

// ClientFrames are the messages clients send.
var ClientFrames = []httpx.Frame{
	{Type: msgHostState, Payload: CouchHostStateCommand{}, Doc: "Host only: the host's play state. The first one from the device the host's account last started hosting on hands the session to it: the host's other devices become remotes."},
	{Type: msgEmoji, Payload: CouchEmojiCommand{}, Doc: "Send a reaction."},
	{Type: msgPaused, Payload: CouchPausedCommand{}, Doc: "Follower only: paused or resumed locally."},
	{Type: msgRemote, Payload: CouchRemoteCommand{}, Doc: "Remote only: steer the host's player."},
}

// CouchMediaRef identifies what the host is watching. An empty Kind means the
// host is on the browse screen ("choosing what to watch").
type CouchMediaRef struct {
	Kind      string `json:"kind" enum:",movie,episode" doc:"Empty while the host is choosing what to watch."`
	TitleID   string `json:"titleId,omitempty"`
	EpisodeID string `json:"episodeId,omitempty"`
}

// CouchHostState is the single authoritative play state, set by the host and
// relayed to followers. ServerTimestampMs and Seq are stamped by the Hub so
// followers can drop stale frames and extrapolate the position without
// trusting client clocks.
type CouchHostState struct {
	Media             CouchMediaRef `json:"media"`
	Playing           bool          `json:"playing"`
	PositionSeconds   float64       `json:"positionSeconds"`
	ServerTimestampMs int64         `json:"serverTimestamp" doc:"When the state was stamped, in milliseconds on the server's monotonic clock (the hello frame's serverTime)."`
	Seq               uint64        `json:"seq" doc:"Increases with every state change; drop frames older than the last seen."`
	Away              bool          `json:"away" doc:"The host lost its connection and the grace countdown is running."`
}

// CouchParticipant is what every viewer sees of one participant.
type CouchParticipant struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	AvatarID    *string `json:"avatarId,omitempty"`
	Seed        string  `json:"seed,omitempty" doc:"Identicon seed: the username, or a random seed for an anonymous viewer."`
	IsHost      bool    `json:"isHost"`
	IsAnonymous bool    `json:"isAnonymous"`
	Paused      bool    `json:"paused" doc:"A follower paused their own playback locally."`
}

type CouchHello struct {
	SessionID       string             `json:"sessionId"`
	MyParticipantID string             `json:"myParticipantId"`
	Role            string             `json:"role" enum:"host,follower,remote" doc:"A remote receives host_state like a follower but plays nothing; it sends remote_command. The host's playing device becomes a remote when the host's account starts hosting on another device, and its host_state is ignored from then on."`
	State           CouchHostState     `json:"state"`
	Participants    []CouchParticipant `json:"participants"`
	ServerTimeMs    int64              `json:"serverTime" doc:"The server's monotonic clock in milliseconds when the snapshot was taken."`
}

type CouchParticipants struct {
	Participants []CouchParticipant `json:"participants"`
}

type CouchMediaChanged struct {
	Media CouchMediaRef `json:"media"`
	Seq   uint64        `json:"seq"`
}

type CouchHostAway struct {
	GraceSeconds int `json:"graceSeconds"`
}

type CouchEmoji struct {
	FromParticipantID string `json:"fromParticipantId"`
	Emoji             string `json:"emoji"`
}

type CouchSessionEnded struct {
	Reason string `json:"reason" enum:"host_ended,host_left,host_timeout,idle,server_shutdown"`
}

type CouchHostStateCommand struct {
	Media           CouchMediaRef `json:"media"`
	Playing         bool          `json:"playing"`
	PositionSeconds float64       `json:"positionSeconds"`
}

type CouchEmojiCommand struct {
	Emoji string `json:"emoji"`
}

type CouchPausedCommand struct {
	Paused bool `json:"paused"`
}

// CouchRemoteCommand is a remote's request to the host's player.
type CouchRemoteCommand struct {
	Action          string   `json:"action" enum:"play,pause,seek,next,previous"`
	PositionSeconds *float64 `json:"positionSeconds,omitempty" doc:"Where to seek; required for seek." minimum:"0"`
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
