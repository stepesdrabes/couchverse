package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/couchtv"
)

// dialRoom opens the Couch TV room's socket with the cookies client holds for it.
func (e *testEnv) dialRoom(t *testing.T, client *http.Client) *websocket.Conn {
	t.Helper()
	u, _ := url.Parse(e.srv.URL + "/api/v1/tv/socket")
	header := http.Header{}
	if client.Jar != nil {
		for _, c := range client.Jar.Cookies(u) {
			header.Add("Cookie", c.Name+"="+c.Value)
		}
	}
	ws, res, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(u.String(), "http"), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		t.Fatalf("dial the room: %d %v", status, err)
	}
	return ws
}

func readFrame(t *testing.T, ws *websocket.Conn, typ string) json.RawMessage {
	t.Helper()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		var env couchtv.Envelope
		err := wsjson.Read(ctx, ws, &env)
		cancel()
		if err != nil {
			t.Fatalf("waiting for %s: %v", typ, err)
		}
		if env.Type == typ {
			return env.Data
		}
	}
}

// TestCouchTvGuests joins a guest through the channel link and follows them into
// the room and the stream, until the admin rotates the link: the guest's grant and
// socket end at once. A signed-in viewer shares the room with them.
func TestCouchTvGuests(t *testing.T) {
	env := newTestEnv(t)
	setFeatures(t, env, true)
	admin, guest := env.clients["admin"], env.clients["guest"]

	status, body, _ := env.request(t, admin, "POST", "/admin/couchtv/link", nil)
	var link couchtv.CouchTvLink
	if err := json.Unmarshal(body, &link); status != 200 || err != nil || link.Path != "/tv/join/"+link.Code || !link.Link.Enabled {
		t.Fatalf("rotate: %d %s", status, body)
	}
	status, body, _ = env.request(t, guest, "POST", "/tv/join/"+link.Code, map[string]string{"name": "Babička"})
	if status != 200 {
		t.Fatalf("join: %d %s", status, body)
	}
	u, _ := url.Parse(env.srv.URL + "/api/v1/tv/tune")
	if cookies := guest.Jar.Cookies(u); len(cookies) != 1 || cookies[0].Name != couchtv.GuestCookie {
		t.Fatalf("the channel cookie: %v", cookies)
	}
	if other, _ := url.Parse(env.srv.URL + "/api/v1/home"); len(guest.Jar.Cookies(other)) != 0 {
		t.Error("the channel cookie reaches the rest of the API")
	}

	status, body, _ = env.request(t, guest, "GET", "/tv/tune", nil)
	var tune couchtv.CouchTvTune
	if err := json.Unmarshal(body, &tune); status != 200 || err != nil || tune.Guest == nil || tune.Guest.Name != "Babička" ||
		tune.ArtworkGrant == "" {
		t.Fatalf("guest tune: %d %s", status, body)
	}
	if status, body := env.get(t, tune.StreamURL); status == http.StatusForbidden {
		t.Fatalf("the guest's stream: %d %s", status, body)
	}
	// a guest is no signed-in viewer
	if status, _, _ := env.request(t, guest, "GET", "/couchtv/tune", nil); status != 401 {
		t.Errorf("a guest on the signed-in tune: %d", status)
	}

	room := env.dialRoom(t, guest)
	defer room.CloseNow()
	var hello couchtv.CouchTvHello
	_ = json.Unmarshal(readFrame(t, room, "hello"), &hello)
	if !hello.Me.Guest || hello.Me.DisplayName != "Babička" || hello.Presence.Count != 1 ||
		time.Since(time.UnixMilli(hello.ServerTime)) > time.Minute {
		t.Fatalf("hello %+v", hello)
	}
	member := env.dialRoom(t, env.clients[memberName])
	defer member.CloseNow()
	_ = json.Unmarshal(readFrame(t, member, "hello"), &hello)
	if hello.Me.Guest || hello.Me.Seed != memberName || hello.Presence.Count != 2 {
		t.Fatalf("Nora's hello %+v", hello)
	}
	readFrame(t, room, "presence")
	if err := wsjson.Write(context.Background(), room, couchtv.Envelope{Type: "reaction", Data: json.RawMessage(`{"emoji":"🍿"}`)}); err != nil {
		t.Fatal(err)
	}
	var reaction couchtv.CouchTvReaction
	_ = json.Unmarshal(readFrame(t, member, "reaction"), &reaction)
	if reaction.Emoji != "🍿" || !strings.HasPrefix(reaction.ViewerID, "guest-") {
		t.Errorf("reaction %+v", reaction)
	}

	// a new link ends the old one's guests: their stream, their socket, their tune
	if status, body, _ := env.request(t, admin, "POST", "/admin/couchtv/link", nil); status != 200 {
		t.Fatalf("rotate again: %d %s", status, body)
	}
	if status, body := env.get(t, tune.StreamURL); status != 403 || !strings.Contains(string(body), "grant_revoked") {
		t.Errorf("the old guest's stream: %d %s", status, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		var frame couchtv.Envelope
		if err := wsjson.Read(ctx, room, &frame); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Errorf("the guest's socket ended with %v", err)
			}
			break
		}
	}
	if status, _, _ := env.request(t, guest, "GET", "/tv/tune", nil); status != 401 {
		t.Errorf("the old guest tunes in: %d", status)
	}
	if _, res, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(env.srv.URL, "http")+"/api/v1/tv/socket", nil); err == nil || res.StatusCode != 401 {
		t.Errorf("an unknown viewer opened the room: %v", err)
	}

	// what the room records lands in the channel's own watch-time bucket
	store := analytics.NewStore(env.pool)
	if err := store.RecordTvWatch(context.Background(), 0, "", 30); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTvWatch(context.Background(), 0, "", 15); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordTvWatch(context.Background(), 2, movieID, 60); err != nil {
		t.Fatal(err)
	}
	overview, err := store.Overview(context.Background(), 7)
	if err != nil || overview.Totals.TvSeconds != 105 || overview.Daily[len(overview.Daily)-1].TvSeconds != 105 {
		t.Errorf("overview %+v %v", overview, err)
	}
	var rows int
	_ = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM tv_watch_time_daily`).Scan(&rows)
	if rows != 2 {
		t.Errorf("%d watch-time rows, want a guests' one and Nora's", rows)
	}
}
