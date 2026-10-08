package couch

import (
	"context"
	"encoding/json"
	"errors"
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

	rm, hostP, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, followerToken, _, err := h.join(rm, nil, false)
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
		Data: mustJSON(CouchHostStateCommand{Media: CouchMediaRef{Kind: "movie", TitleID: "t1"}, Playing: true, PositionSeconds: 42}),
	}); err != nil {
		t.Fatalf("host write: %v", err)
	}

	got := waitForType(t, fc, msgHostState)
	var st CouchHostState
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
	if err := wsjson.Write(context.Background(), hc, Envelope{Type: msgEmoji, Data: mustJSON(CouchEmojiCommand{Emoji: "🎉"})}); err != nil {
		t.Fatalf("emoji write: %v", err)
	}
	ge := waitForType(t, fc, msgEmoji)
	var ed CouchEmoji
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

	rm, _, hostToken, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"})
	follower, followerToken, _, _ := h.join(rm, nil, false)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/" + rm.shareToken + "/ws"

	hc := dialWS(t, wsURL, hostToken)
	defer hc.Close(websocket.StatusNormalClosure, "")
	waitForType(t, hc, msgHello)
	fc := dialWS(t, wsURL, followerToken)
	defer fc.Close(websocket.StatusNormalClosure, "")
	waitForType(t, fc, msgHello)

	if err := wsjson.Write(context.Background(), fc, Envelope{Type: msgPaused, Data: mustJSON(CouchPausedCommand{Paused: true})}); err != nil {
		t.Fatalf("paused write: %v", err)
	}

	env := waitForType(t, hc, msgParticipants)
	var pd CouchParticipants
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

// The host's account on a second device joins as a remote: the host's own token keeps
// working, the remote sees the play state, its commands reach the playing device only,
// followers cannot steer, and a remote leaving keeps the session.
func TestWSRemoteSteersTheHostsPlayer(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}

	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/x/ws"

	rm, _, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, _, err := h.join(rm, nil, true); !errors.Is(err, errNotHost) {
		t.Fatalf("a guest joined as a remote: %v", err)
	}
	_, remoteToken, role, err := h.join(rm, host(1), true)
	if err != nil || role != roleRemote {
		t.Fatalf("remote join: role %q, %v", role, err)
	}
	_, followerToken, _, _ := h.join(rm, nil, false)

	tv := dialWS(t, wsURL, hostToken) // still valid after the remote joined
	defer tv.CloseNow()
	waitForType(t, tv, msgHello)

	hdr := http.Header{}
	hdr.Set(CouchTokenHeader, remoteToken)
	phone, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatalf("dial remote: %v", err)
	}
	defer phone.CloseNow()
	var hello CouchHello
	if err := json.Unmarshal(waitForType(t, phone, msgHello).Data, &hello); err != nil || hello.Role != roleRemote {
		t.Fatalf("remote hello role %q, %v", hello.Role, err)
	}

	ctx := context.Background()
	state := CouchHostStateCommand{Media: CouchMediaRef{Kind: "movie", TitleID: "t1"}, Playing: true, PositionSeconds: 30}
	if err := wsjson.Write(ctx, tv, Envelope{Type: msgHostState, Data: mustJSON(state)}); err != nil {
		t.Fatal(err)
	}
	waitForType(t, phone, msgHostState)

	seek := 120.0
	cmd := CouchRemoteCommand{Action: "seek", PositionSeconds: &seek}
	if err := wsjson.Write(ctx, phone, Envelope{Type: msgRemote, Data: mustJSON(cmd)}); err != nil {
		t.Fatal(err)
	}
	var got CouchRemoteCommand
	if err := json.Unmarshal(waitForType(t, tv, msgRemote).Data, &got); err != nil || got.Action != "seek" || *got.PositionSeconds != 120 {
		t.Fatalf("host got %+v, %v", got, err)
	}

	guest := dialWS(t, wsURL, followerToken)
	defer guest.CloseNow()
	waitForType(t, guest, msgHello)
	if err := wsjson.Write(ctx, guest, Envelope{Type: msgRemote, Data: mustJSON(cmd)}); err != nil {
		t.Fatal(err)
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		var env Envelope
		if err := wsjson.Read(readCtx, guest, &env); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("a follower steering was not refused: %v", err)
			}
			break
		}
	}

	h.leaveByToken(remoteToken)
	rm.mu.Lock()
	live := rm.live
	rm.mu.Unlock()
	if !live {
		t.Fatal("a remote leaving ended the session")
	}
	if _, ok := h.lookup(remoteToken); ok {
		t.Fatal("the remote's token still works after leaving")
	}
	if _, ok := h.lookup(hostToken); !ok {
		t.Fatal("the host's token stopped working")
	}
}

// A browser keeps one couch cookie: joining is refused only while the host's token in it has
// a socket open (another tab plays for the session), not for a host device gone quiet, a
// remote's or a follower's token.
func TestPlaysForNeedsTheHostsOpenSocket(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}

	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/x/ws"

	rm, _, hostToken, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"})
	_, remoteToken, _, _ := h.join(rm, host(1), true)
	_, followerToken, _, _ := h.join(rm, nil, false)
	if h.playsFor(rm, hostToken) {
		t.Fatal("a host token without a socket counts as playing")
	}

	tv := dialWS(t, wsURL, hostToken)
	waitForType(t, tv, msgHello)
	phone := dialWS(t, wsURL, remoteToken)
	defer phone.CloseNow()
	waitForType(t, phone, msgHello)
	guest := dialWS(t, wsURL, followerToken)
	defer guest.CloseNow()
	waitForType(t, guest, msgHello)

	if !h.playsFor(rm, hostToken) {
		t.Fatal("the playing device's token was not recognised")
	}
	if h.playsFor(rm, remoteToken) || h.playsFor(rm, followerToken) || h.playsFor(rm, "") {
		t.Fatal("a remote's, a follower's or no token counts as playing")
	}

	tv.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(3 * time.Second)
	for h.playsFor(rm, hostToken) {
		if time.Now().After(deadline) {
			t.Fatal("a closed socket still counts as playing")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Ending a session must deliver session_ended to every attached follower before the
// socket closes; a follower that only sees the close waits for the host forever.
func TestWSEndDeliversSessionEnded(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}

	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// the teardown races the final write; rounds keep a lucky ordering from hiding a drop
	for range 50 {
		rm, _, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		_, followerToken, _, err := h.join(rm, nil, false)
		if err != nil {
			t.Fatalf("join: %v", err)
		}
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/" + rm.shareToken + "/ws"
		fc := dialWS(t, wsURL, followerToken)
		waitForType(t, fc, msgHello)

		if !h.endByHostToken(hostToken) {
			t.Fatal("the host token should end the session")
		}
		waitForType(t, fc, msgSessionEnded)
		fc.CloseNow()
	}
}
