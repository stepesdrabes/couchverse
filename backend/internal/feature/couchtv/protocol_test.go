package couchtv

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden frames in contract/fixtures/couchtv")

const goldenDir = "../../../../contract/fixtures/couchtv"

type golden struct {
	server bool
	typ    string
	data   any
}

// goldenFrames holds one representative frame per message type. The core's
// couchtv tests decode the same files, so both sides agree on the wire format.
func goldenFrames() map[string]golden {
	avatar := "00000000-0000-4000-8000-000000000405"
	nora := CouchTvViewer{ID: "user-2", DisplayName: "Nora", AvatarID: &avatar, Seed: "nora"}
	guest := CouchTvViewer{ID: "guest-6ba7b810-9dad-41d1-80b4-00c04fd430c8", DisplayName: "Babička",
		Seed: "6ba7b810-9dad-41d1-80b4-00c04fd430c8", Guest: true}
	presence := CouchTvPresence{Count: 2, Viewers: []CouchTvViewer{nora, guest}}
	return map[string]golden{
		"server-hello":            {true, msgHello, CouchTvHello{ServerTime: 1_791_043_200_000, Me: guest, Presence: presence}},
		"server-presence":         {true, msgPresence, presence},
		"server-reaction":         {true, msgReaction, CouchTvReaction{ViewerID: nora.ID, Emoji: "🍿"}},
		"server-schedule-changed": {true, msgScheduleChanged, nil},
		"client-reaction":         {false, msgReaction, CouchTvReactionCommand{Emoji: "😂"}},
		"client-report":           {false, msgReport, CouchTvReport{ProgramDate: 1_791_043_170_040}},
	}
}

func TestGoldenFramesMatchTheWireFormat(t *testing.T) {
	_, registry := protocolSchema()
	frames := goldenFrames()
	for name, g := range frames {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, envelope(g.typ, g.data), "", "  "); err != nil {
			t.Fatal(err)
		}
		pretty.WriteByte('\n')
		path := filepath.Join(goldenDir, name+".json")
		if *updateGolden {
			if err := os.MkdirAll(goldenDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run go test ./internal/feature/couchtv -update)", name, err)
		}
		if !bytes.Equal(want, pretty.Bytes()) {
			t.Errorf("%s: frame changed; rerun with -update if intended\nwant %s\ngot  %s", name, want, pretty.Bytes())
		}
		if g.data != nil {
			var value any
			_ = json.Unmarshal(envelope(g.typ, g.data), &value)
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
		frames []httpx.Frame
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
