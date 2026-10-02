package couch

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden frames in contract/fixtures/couch")

const goldenDir = "../../../../contract/fixtures/couch"

type golden struct {
	server bool
	typ    string
	data   any
}

// goldenFrames holds one representative frame per message type. The core's
// couch tests decode the same files, so both sides agree on the wire format.
func goldenFrames() map[string]golden {
	avatar := "00000000-0000-4000-8000-000000000405"
	movie := CouchMediaRef{Kind: "movie", TitleID: "00000000-0000-4000-8000-000000000101"}
	state := CouchHostState{Media: movie, Playing: true, PositionSeconds: 1834.5, ServerTimestampMs: 912345, Seq: 42}
	host := CouchParticipant{ID: "p-host", DisplayName: "Nora", AvatarID: &avatar, Seed: "nora", IsHost: true}
	guest := CouchParticipant{ID: "p-guest", DisplayName: "Sleepy Otter", Seed: "x7Qe2LmA", IsAnonymous: true, Paused: true}
	return map[string]golden{
		"server-hello": {true, msgHello, CouchHello{
			SessionID: "s-1", MyParticipantID: "p-guest", Role: "follower", State: state,
			Participants: []CouchParticipant{host, guest}, ServerTimeMs: 912400,
		}},
		"server-host-state":    {true, msgHostState, state},
		"server-participants":  {true, msgParticipants, CouchParticipants{Participants: []CouchParticipant{host, guest}}},
		"server-media-changed": {true, msgMediaChanged, CouchMediaChanged{Media: CouchMediaRef{Kind: "episode", TitleID: "00000000-0000-4000-8000-000000000102", EpisodeID: "00000000-0000-4000-8000-000000000301"}, Seq: 43}},
		"server-host-away":     {true, msgHostAway, CouchHostAway{GraceSeconds: 60}},
		"server-host-returned": {true, msgHostReturned, nil},
		"server-emoji":         {true, msgEmoji, CouchEmoji{FromParticipantID: "p-guest", Emoji: "🍿"}},
		"server-session-ended": {true, msgSessionEnded, CouchSessionEnded{Reason: "host_ended"}},
		"client-host-state":    {false, msgHostState, CouchHostStateCommand{Media: movie, Playing: false, PositionSeconds: 1840}},
		"client-emoji":         {false, msgEmoji, CouchEmojiCommand{Emoji: "😂"}},
		"client-paused":        {false, msgPaused, CouchPausedCommand{Paused: true}},
	}
}

func TestGoldenFramesMatchTheWireFormat(t *testing.T) {
	_, registry := protocolSchema()
	frames := goldenFrames()
	for name, g := range frames {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, mustEnvelope(g.typ, g.data), "", "  "); err != nil {
			t.Fatal(err)
		}
		pretty.WriteByte('\n')
		path := filepath.Join(goldenDir, name+".json")
		if *updateGolden {
			if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run go test ./internal/feature/couch -update)", name, err)
		}
		if !bytes.Equal(want, pretty.Bytes()) {
			t.Errorf("%s: frame changed; rerun with -update if intended\nwant %s\ngot  %s", name, want, pretty.Bytes())
		}
		if g.data != nil {
			var value any
			_ = json.Unmarshal(mustEnvelope(g.typ, g.data), &value)
			schema := registry.Schema(reflect.TypeOf(g.data), true, "")
			res := &huma.ValidateResult{}
			huma.Validate(registry, schema, huma.NewPathBuffer([]byte{}, 0), huma.ModeReadFromServer, value.(map[string]any)["data"], res)
			for _, err := range res.Errors {
				t.Errorf("%s: payload does not match the schema: %v", name, err)
			}
		}
	}

	for _, table := range []struct {
		server bool
		frames []Frame
	}{{true, ServerFrames}, {false, ClientFrames}} {
		for _, f := range table.frames {
			found := false
			for _, g := range frames {
				found = found || (g.server == table.server && g.typ == f.Type)
			}
			if !found {
				t.Errorf("frame %q (server=%v) has no golden file", f.Type, table.server)
			}
		}
	}
}
