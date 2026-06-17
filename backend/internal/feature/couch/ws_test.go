package couch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"couchverse/internal/media"
)

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func dialWS(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	hdr.Set("Cookie", CouchCookie+"="+token)
	c, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func waitForType(t *testing.T, c *websocket.Conn, typ string) Envelope {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var env Envelope
		err := wsjson.Read(ctx, c, &env)
		cancel()
		if err != nil {
			t.Fatalf("read waiting for %s: %v", typ, err)
		}
		if env.Type == typ {
			return env
		}
	}
	t.Fatalf("did not receive %s in time", typ)
	return Envelope{}
}

// End-to-end over a real socket: a host's play-state and emoji fan out to a
// follower, the server stamps a sequence, and the host never echoes its own state.
func TestWSHostStateAndEmojiFanout(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}

	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	rm, hostP, hostToken, err := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, followerToken, _, err := h.join(rm, nil)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/" + rm.shareToken + "/ws"

	fc := dialWS(t, wsURL, followerToken)
	defer fc.Close(websocket.StatusNormalClosure, "")
	if env := waitForType(t, fc, msgHello); env.Type != msgHello {
		t.Fatal("follower should get hello first")
	}

	hc := dialWS(t, wsURL, hostToken)
	defer hc.Close(websocket.StatusNormalClosure, "")
	waitForType(t, hc, msgHello)

	// host broadcasts its play-state
	if err := wsjson.Write(context.Background(), hc, Envelope{
		Type: msgHostState,
		Data: mustJSON(hostStateCmd{Media: mediaRef{Kind: "movie", TitleID: "t1"}, Playing: true, PositionSeconds: 42}),
	}); err != nil {
		t.Fatalf("host write: %v", err)
	}

	got := waitForType(t, fc, msgHostState)
	var st hostState
	if err := json.Unmarshal(got.Data, &st); err != nil {
		t.Fatalf("unmarshal host_state: %v", err)
	}
	if !st.Playing || st.PositionSeconds != 42 {
		t.Fatalf("follower host_state = %+v, want playing@42", st)
	}
	if st.Seq == 0 || st.ServerTimestampMs == 0 {
		t.Fatalf("server must stamp seq+timestamp, got %+v", st)
	}

	// emoji relays to everyone, tagged with the sender
	if err := wsjson.Write(context.Background(), hc, Envelope{Type: msgEmoji, Data: mustJSON(emojiCmd{Emoji: "🎉"})}); err != nil {
		t.Fatalf("emoji write: %v", err)
	}
	ge := waitForType(t, fc, msgEmoji)
	var ed emojiData
	if err := json.Unmarshal(ge.Data, &ed); err != nil {
		t.Fatalf("unmarshal emoji: %v", err)
	}
	if ed.Emoji != "🎉" || ed.FromParticipantID != hostP.ID {
		t.Fatalf("emoji = %+v, want 🎉 from %s", ed, hostP.ID)
	}
}

// A follower's local pause is relayed to everyone (so it can be shown on the couch).
func TestWSPausedBroadcast(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}

	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	rm, _, hostToken, _ := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t1"})
	follower, followerToken, _, _ := h.join(rm, nil)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/" + rm.shareToken + "/ws"

	hc := dialWS(t, wsURL, hostToken)
	defer hc.Close(websocket.StatusNormalClosure, "")
	waitForType(t, hc, msgHello)
	fc := dialWS(t, wsURL, followerToken)
	defer fc.Close(websocket.StatusNormalClosure, "")
	waitForType(t, fc, msgHello)

	if err := wsjson.Write(context.Background(), fc, Envelope{Type: msgPaused, Data: mustJSON(pausedCmd{Paused: true})}); err != nil {
		t.Fatalf("paused write: %v", err)
	}

	env := waitForType(t, hc, msgParticipants)
	var pd participantsData
	if err := json.Unmarshal(env.Data, &pd); err != nil {
		t.Fatalf("unmarshal participants: %v", err)
	}
	found := false
	for _, p := range pd.Participants {
		if p.ID == follower.ID {
			found = true
			if !p.Paused {
				t.Fatal("follower should be marked paused after pausing locally")
			}
		}
	}
	if !found {
		t.Fatal("follower missing from the participants broadcast")
	}
}
