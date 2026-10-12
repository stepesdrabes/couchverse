package couchtv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// fakeLineup airs a 30 min episode from tuned, then a break, then a movie.
type fakeLineup struct{ parts []LineupPart }

func (f fakeLineup) Lineup(_ context.Context, from, to time.Time) ([]LineupPart, error) {
	var out []LineupPart
	for _, p := range f.parts {
		if p.End.After(from) && p.Start.Before(to) {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeDurations map[string]time.Duration

func (f fakeDurations) FileDurations(_ context.Context, ids []string) (map[string]time.Duration, error) {
	return f, nil
}

type watchRow struct {
	userID  int64
	titleID string
}

// recorder takes everything the room records.
type recorder struct {
	mu        sync.Mutex
	watched   map[watchRow]int
	progress  []string
	viewings  []viewing
	reactions map[int64]int
	audience  map[int64]int
}

func newRecorder() *recorder {
	return &recorder{watched: map[watchRow]int{}, reactions: map[int64]int{}, audience: map[int64]int{}}
}

func (r *recorder) RecordTvWatch(_ context.Context, userID int64, titleID string, seconds int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watched[watchRow{userID, titleID}] += seconds
	return nil
}

func (r *recorder) UpsertProgress(_ context.Context, userID int64, titleID, episodeID *string, position, duration int, _ *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := ""
	if titleID != nil {
		id = "title " + *titleID
	} else {
		id = "episode " + *episodeID
	}
	r.progress = append(r.progress, strings.Join([]string{id, itoa(position), itoa(duration)}, " "))
	return nil
}

func (r *recorder) RecordTvViewing(_ context.Context, userID int64, airingID, kind string, at time.Time, seconds int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewings = append(r.viewings, viewing{userID: userID, airingID: airingID, kind: kind, at: at, seconds: seconds})
	return nil
}

func (r *recorder) RecordTvReactions(_ context.Context, userID int64, count int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reactions[userID] += count
	return nil
}

func (r *recorder) RecordTvAudience(_ context.Context, userID int64, viewers int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audience[userID] = viewers
	return nil
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

var tuned = time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)

// testRoom is a room over a fake lineup and recorders, on a clock the test moves.
func testRoom(t *testing.T) (*Hub, *recorder, *time.Time) {
	t.Helper()
	now := tuned
	rec := newRecorder()
	lineup := fakeLineup{parts: []LineupPart{
		{AiringID: "a-episode", Kind: PartProgram, Start: tuned, End: tuned.Add(22 * time.Minute), File: "f-episode",
			TitleID: "t-series", EpisodeID: "e-pilot", Fill: FillStrip},
		{AiringID: "a-break", Kind: PartBreak, Start: tuned.Add(22 * time.Minute), End: tuned.Add(25 * time.Minute)},
		{AiringID: "a-movie", Kind: PartProgram, Start: tuned.Add(25 * time.Minute), End: tuned.Add(140 * time.Minute),
			File: "f-movie", TitleID: "t-movie", Fill: FillPremiere},
	}}
	h := newHub(context.Background(), HubDeps{Lineup: lineup,
		Durations: fakeDurations{"f-episode": 23 * time.Minute, "f-movie": 2 * time.Hour},
		Watch:     rec, Progress: rec, Stats: rec, Now: func() time.Time { return now }})
	return h, rec, &now
}

// socket is a viewer's socket without a network: what the room queues for it.
func socket(h *Hub, me CouchTvViewer, userID int64) *conn {
	c := &conn{send: make(chan []byte, 64), closed: make(chan struct{})}
	h.attach(c, me, userID)
	return c
}

func frames(c *conn) []Envelope {
	var out []Envelope
	for {
		select {
		case raw := <-c.send:
			var env Envelope
			_ = json.Unmarshal(raw, &env)
			out = append(out, env)
		default:
			return out
		}
	}
}

func types(envs []Envelope) []string {
	var out []string
	for _, e := range envs {
		out = append(out, e.Type)
	}
	return out
}

var (
	nora  = CouchTvViewer{ID: "user-2", DisplayName: "Nora", Seed: "nora"}
	guest = CouchTvViewer{ID: "guest-g1", DisplayName: "Babička", Seed: "g1", Guest: true}
)

func TestTheRoomGreetsAndCountsEachViewerOnce(t *testing.T) {
	h, _, _ := testRoom(t)
	phone := socket(h, nora, 2)
	got := frames(phone)
	if len(got) != 1 || got[0].Type != msgHello {
		t.Fatalf("first frames %v", types(got))
	}
	var hello CouchTvHello
	_ = json.Unmarshal(got[0].Data, &hello)
	if hello.ServerTime != tuned.UnixMilli() || hello.Me.ID != "user-2" || hello.Presence.Count != 1 {
		t.Fatalf("hello %+v", hello)
	}

	// Nora's second device is no one new
	tv := socket(h, nora, 2)
	if got := types(frames(tv)); len(got) != 1 || got[0] != msgHello {
		t.Fatalf("second device: %v", got)
	}
	if got := frames(phone); len(got) != 0 {
		t.Fatalf("the first device heard %v", types(got))
	}

	g := socket(h, guest, 0)
	frames(g)
	got = frames(phone)
	var presence CouchTvPresence
	_ = json.Unmarshal(got[0].Data, &presence)
	if len(got) != 1 || got[0].Type != msgPresence || presence.Count != 2 || presence.Viewers[1].DisplayName != "Babička" {
		t.Fatalf("presence %v %+v", types(got), presence)
	}

	h.detach(tv)
	if got := frames(phone); len(got) != 0 {
		t.Fatalf("Nora still watches on her phone, yet %v", types(got))
	}
	h.detach(g)
	_ = json.Unmarshal(frames(phone)[0].Data, &presence)
	if presence.Count != 1 || h.Viewers() != 1 {
		t.Fatalf("after the guest left: %+v", presence)
	}
}

func TestReactionsAreRelayedAndLimited(t *testing.T) {
	h, rec, now := testRoom(t)
	phone := socket(h, nora, 2)
	g := socket(h, guest, 0)
	frames(phone)
	frames(g)
	for _, emoji := range []string{"🍿", "😂", "  ", "🎉", "🔥"} {
		h.react(g, emoji)
	}
	got := frames(phone)
	if len(got) != 3 {
		t.Fatalf("relayed %v, want three", types(got))
	}
	var r CouchTvReaction
	_ = json.Unmarshal(got[2].Data, &r)
	if r.ViewerID != guest.ID || r.Emoji != "🎉" {
		t.Fatalf("reaction %+v", r)
	}
	*now = now.Add(time.Second)
	h.react(g, "🔥")
	h.react(phone, "👏")
	if got := frames(g); len(got) != 5 {
		t.Fatalf("the guest heard %v: their three, the fourth a second later and Nora's", types(got))
	}
	h.accrue(context.Background())
	if rec.reactions[2] != 1 || len(rec.reactions) != 1 {
		t.Fatalf("reaction counts %v: only signed-in viewers count", rec.reactions)
	}
}

func TestReportsBecomeWatchTimeAndProgress(t *testing.T) {
	h, rec, now := testRoom(t)
	phone := socket(h, nora, 2)
	tv := socket(h, nora, 2)
	g := socket(h, guest, 0)
	// three minutes of the episode on Nora's phone and TV (the same picture,
	// 9 s behind the clock), the guest from a minute in
	for s := 0; s <= 180; s += 10 {
		*now = tuned.Add(time.Duration(s)*time.Second + 9*time.Second)
		pd := tuned.Add(time.Duration(s) * time.Second).UnixMilli()
		h.report(phone, pd)
		h.report(tv, pd+40)
		if s >= 60 {
			h.report(g, pd)
		}
	}
	h.accrue(context.Background())
	if got := rec.watched[watchRow{2, "t-series"}]; got != 180 {
		t.Errorf("Nora watched %d s of the series, want 180 (two devices count once)", got)
	}
	if got := rec.watched[watchRow{0, "t-series"}]; got != 120 {
		t.Errorf("the guest watched %d s, want 120", got)
	}
	if len(rec.progress) != 1 || rec.progress[0] != "episode e-pilot 180 1380" {
		t.Errorf("progress %v", rec.progress)
	}
	if rec.audience[2] != 2 || len(rec.audience) != 1 {
		t.Errorf("audience %v", rec.audience)
	}
	// on the channel's clock (Prague's summer time here): 18:00 UTC is its 20:00
	if v := rec.viewings[0]; v.userID != 2 || v.airingID != "a-episode" || v.kind != PartProgram ||
		!v.at.Equal(tuned) || v.at.Hour() != 20 || v.seconds != 180 {
		t.Errorf("viewing %+v", v)
	}

	// a pause (no reports), a jump back and a report from the future count nothing
	rec.watched = map[watchRow]int{}
	*now = now.Add(5 * time.Minute)
	h.report(phone, tuned.Add(185*time.Second).UnixMilli())
	h.report(phone, tuned.Add(175*time.Second).UnixMilli())
	h.report(phone, now.Add(2*time.Minute).UnixMilli())
	h.accrue(context.Background())
	if len(rec.watched) != 0 {
		t.Errorf("counted %v", rec.watched)
	}

	// watched across the end of the episode: finished, and the break counts as
	// time on the channel without a title
	rec.progress = nil
	for s := 21*60 + 50; s <= 22*60+30; s += 10 {
		*now = tuned.Add(time.Duration(s)*time.Second + 9*time.Second)
		h.report(phone, tuned.Add(time.Duration(s)*time.Second).UnixMilli())
	}
	h.accrue(context.Background())
	if len(rec.progress) != 1 || rec.progress[0] != "episode e-pilot 1380 1380" {
		t.Errorf("progress at the end %v", rec.progress)
	}
	if rec.watched[watchRow{2, ""}] != 30 || rec.watched[watchRow{2, "t-series"}] != 10 {
		t.Errorf("across the boundary %v", rec.watched)
	}

	// the movie needs two minutes of its own before it shows in Continue watching
	rec.progress = nil
	for s := 25 * 60; s <= 26*60; s += 10 {
		*now = tuned.Add(time.Duration(s)*time.Second + 9*time.Second)
		h.report(phone, tuned.Add(time.Duration(s)*time.Second).UnixMilli())
	}
	h.accrue(context.Background())
	if len(rec.progress) != 0 {
		t.Errorf("a minute of the movie saved %v", rec.progress)
	}
}

func TestTheRoomForgetsViewersWhoLeft(t *testing.T) {
	h, rec, now := testRoom(t)
	phone := socket(h, nora, 2)
	*now = tuned.Add(19 * time.Second)
	h.report(phone, tuned.Add(10*time.Second).UnixMilli())
	*now = tuned.Add(29 * time.Second)
	h.report(phone, tuned.Add(20*time.Second).UnixMilli())
	h.detach(phone)
	h.accrue(context.Background())
	if rec.watched[watchRow{2, "t-series"}] != 10 {
		t.Errorf("what Nora watched before leaving: %v", rec.watched)
	}
	if len(h.viewers) != 0 {
		t.Errorf("viewers %v", h.viewers)
	}
}

func TestScheduleChangesAreGathered(t *testing.T) {
	h, _, _ := testRoom(t)
	phone := socket(h, nora, 2)
	frames(phone)
	h.ScheduleChanged()
	h.ScheduleChanged()
	time.Sleep(scheduleSettle + 300*time.Millisecond)
	if got := types(frames(phone)); len(got) != 1 || got[0] != msgScheduleChanged {
		t.Fatalf("frames %v", got)
	}
}

// Over a real socket: hello, a reaction relayed, and a guest closed when the
// channel link ends.
func TestTheRoomOverASocket(t *testing.T) {
	h, _, _ := testRoom(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		if r.URL.Query().Get("as") == "guest" {
			h.serve(ws, guest, 0)
		} else {
			h.serve(ws, nora, 2)
		}
	}))
	defer srv.Close()
	dial := func(as string) *websocket.Conn {
		c, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"?as="+as, nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	read := func(c *websocket.Conn, typ string) Envelope {
		t.Helper()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			var env Envelope
			err := wsjson.Read(ctx, c, &env)
			cancel()
			if err != nil {
				t.Fatalf("waiting for %s: %v", typ, err)
			}
			if env.Type == typ {
				return env
			}
		}
	}
	member := dial("member")
	defer member.CloseNow()
	read(member, msgHello)
	g := dial("guest")
	defer g.CloseNow()
	read(g, msgHello)
	read(member, msgPresence)

	if err := wsjson.Write(context.Background(), g, Envelope{Type: msgReaction, Data: json.RawMessage(`{"emoji":"🍿"}`)}); err != nil {
		t.Fatal(err)
	}
	var r CouchTvReaction
	_ = json.Unmarshal(read(member, msgReaction).Data, &r)
	if r.ViewerID != guest.ID || r.Emoji != "🍿" {
		t.Fatalf("reaction %+v", r)
	}

	h.EndGuests()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		var env Envelope
		if err := wsjson.Read(ctx, g, &env); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("the guest's socket ended with %v", err)
			}
			break
		}
	}
	var presence CouchTvPresence
	_ = json.Unmarshal(read(member, msgPresence).Data, &presence)
	if presence.Count != 1 {
		t.Fatalf("presence after the guests ended %+v", presence)
	}
}
