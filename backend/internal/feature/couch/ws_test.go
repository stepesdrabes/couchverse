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

	rm, hostP, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
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

	rm, _, hostToken, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
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

	rm, _, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
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

	rm, _, hostToken, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
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
		rm, _, hostToken, _, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
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

// dialDevice opens a socket the way a native client does, with the token in a header.
func dialDevice(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	hdr.Set(CouchTokenHeader, token)
	c, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

// waitForAny is the next frame on a socket.
func waitForAny(t *testing.T, c *websocket.Conn) Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var env Envelope
	if err := wsjson.Read(ctx, c, &env); err != nil {
		t.Fatalf("read: %v", err)
	}
	return env
}

func decode[T any](t *testing.T, env Envelope) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(env.Data, &v); err != nil {
		t.Fatalf("decode %s: %v", env.Type, err)
	}
	return v
}

// The host's account starting a session on another device hands it over with nobody
// rejoining: the old device plays for the follower until the new one reports, then hears it is
// a remote and steers the new device, its late reports reach no one, its token brings it back
// as a remote, and the follower follows the new device on the socket it had.
func TestWSHandoverKeepsTheFollowers(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{
		"title:t1": {ID: "mf1", TitleID: ptr("t1")},
		"title:t2": {ID: "mf2", TitleID: ptr("t2")},
	}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}
	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/x/ws"
	ctx := context.Background()
	t1, t2 := CouchMediaRef{Kind: "movie", TitleID: "t1"}, CouchMediaRef{Kind: "movie", TitleID: "t2"}
	send := func(c *websocket.Conn, typ string, data any) {
		t.Helper()
		if err := wsjson.Write(ctx, c, Envelope{Type: typ, Data: mustJSON(data)}); err != nil {
			t.Fatal(err)
		}
	}
	playingAt := func(media CouchMediaRef, position float64) CouchHostStateCommand {
		return CouchHostStateCommand{Media: media, Playing: true, PositionSeconds: position}
	}

	rm, _, laptopToken, _, err := h.createOrReclaim(ctx, host(1), t1, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, followerToken, _, _ := h.join(rm, nil, false)
	laptop := dialWS(t, wsURL, laptopToken)
	defer laptop.CloseNow()
	waitForType(t, laptop, msgHello)
	guest := dialWS(t, wsURL, followerToken)
	defer guest.CloseNow()
	waitForType(t, guest, msgHello)
	send(laptop, msgHostState, playingAt(t1, 100))
	waitForType(t, guest, msgHostState)

	_, _, tvToken, _, err := h.createOrReclaim(ctx, host(1), t2, "")
	if err != nil {
		t.Fatalf("host on the tv: %v", err)
	}
	tv := dialDevice(t, wsURL, tvToken)
	defer tv.CloseNow()
	if hello := decode[CouchHello](t, waitForType(t, tv, msgHello)); hello.Role != roleHost {
		t.Fatalf("the tv joined its own session as %q", hello.Role)
	}
	send(laptop, msgHostState, playingAt(t1, 102))
	if st := decode[CouchHostState](t, waitForType(t, guest, msgHostState)); st.PositionSeconds != 102 {
		t.Fatalf("before the tv played the follower heard %+v", st)
	}

	send(tv, msgHostState, playingAt(t2, 5))
	if hello := decode[CouchHello](t, waitForType(t, laptop, msgHello)); hello.Role != roleRemote {
		t.Fatalf("the laptop is a %q after the handover", hello.Role)
	}
	if changed := decode[CouchMediaChanged](t, waitForType(t, guest, msgMediaChanged)); changed.Media != t2 {
		t.Fatalf("the follower switched to %+v", changed.Media)
	}
	if st := decode[CouchHostState](t, waitForType(t, guest, msgHostState)); st.Media != t2 || st.PositionSeconds != 5 {
		t.Fatalf("the follower heard %+v from the tv", st)
	}

	// a report the laptop sent before it heard is dropped and its socket stays open: the
	// reaction it sends next arrives, with nothing in between
	send(laptop, msgHostState, playingAt(t1, 104))
	send(laptop, msgEmoji, CouchEmojiCommand{Emoji: "👋"})
	for env := waitForAny(t, guest); env.Type != msgEmoji; env = waitForAny(t, guest) {
		if env.Type == msgHostState || env.Type == msgMediaChanged {
			t.Fatalf("the laptop's report reached the follower: %s", env.Data)
		}
	}

	send(laptop, msgRemote, CouchRemoteCommand{Action: "pause"})
	if got := decode[CouchRemoteCommand](t, waitForType(t, tv, msgRemote)); got.Action != "pause" {
		t.Fatalf("the tv got %+v from its remote", got)
	}

	again := dialWS(t, wsURL, laptopToken)
	defer again.CloseNow()
	if hello := decode[CouchHello](t, waitForType(t, again, msgHello)); hello.Role != roleRemote {
		t.Fatalf("the laptop came back as a %q", hello.Role)
	}
	h.leaveByToken(laptopToken)
	rm.mu.Lock()
	live := rm.live
	rm.mu.Unlock()
	if _, ok := h.lookup(tvToken); !live || !ok {
		t.Fatal("the laptop leaving ended the session for the tv")
	}
}

// A browser keeps one couch cookie for all its tabs: a tab hosting the session another tab
// plays for is refused, while another device, or the tab after a refresh (no socket left on
// the cookie's token), hosts it.
func TestCreateRefusesAnotherTabOfThePlayingBrowser(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	hand := &Handlers{hub: h, joinRate: newRateLimiter(1000, time.Minute)}
	r := chi.NewRouter()
	r.Get("/api/v1/couch/{token}/ws", hand.WS)
	srv := httptest.NewServer(r)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/couch/x/ws"
	ctx := context.Background()
	t1 := CouchMediaRef{Kind: "movie", TitleID: "t1"}

	rm, _, tabToken, _, _ := h.createOrReclaim(ctx, host(1), t1, "")
	tab := dialWS(t, wsURL, tabToken)
	waitForType(t, tab, msgHello)

	if _, _, _, _, err := h.createOrReclaim(ctx, host(1), t1, tabToken); !errors.Is(err, errAlreadyHosting) {
		t.Fatalf("another tab of the playing browser hosted: %v", err)
	}
	if _, _, _, _, err := h.createOrReclaim(ctx, host(1), t1, ""); err != nil {
		t.Fatalf("another device could not host: %v", err)
	}

	tab.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(3 * time.Second)
	for h.playsFor(rm, tabToken) {
		if time.Now().After(deadline) {
			t.Fatal("a closed tab still plays for the session")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, _, _, _, err := h.createOrReclaim(ctx, host(1), t1, tabToken); err != nil {
		t.Fatalf("the refreshed tab could not host: %v", err)
	}
}
