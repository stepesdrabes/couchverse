package couch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/playback"
	"couchverse/internal/media"
)

func ptr(s string) *string { return &s }

type fakeMedia struct{ files map[string]*media.MediaFile }

func (f *fakeMedia) PrimaryMediaFileForTitle(_ context.Context, id string) (*media.MediaFile, error) {
	if mf := f.files["title:"+id]; mf != nil {
		return mf, nil
	}
	return nil, errors.New("no media")
}

func (f *fakeMedia) PrimaryMediaFileForEpisode(_ context.Context, id string) (*media.MediaFile, error) {
	if mf := f.files["episode:"+id]; mf != nil {
		return mf, nil
	}
	return nil, errors.New("no media")
}

func (f *fakeMedia) AudioSiblings(_ context.Context, _, _ *string, _ string) ([]media.MediaFile, error) {
	return nil, nil
}

type fakePlayback struct{}

func (fakePlayback) BuildPlayback(_ context.Context, _, _ string, _ playback.Viewer, _ playback.DeviceProfile) (*playback.PlaybackInfo, error) {
	return &playback.PlaybackInfo{}, nil
}

func newTestHub(t *testing.T, fm *fakeMedia) *Hub {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewHub(ctx, Deps{Media: fm, Playback: fakePlayback{}})
}

func host(id int64) *auth.User {
	return &auth.User{ID: id, Username: "host", DisplayName: "Host"}
}

// attachDevice opens a connection for a device token the way the socket handler does, without
// a socket: the frames it is sent queue in its send channel.
func attachDevice(t *testing.T, h *Hub, token string) *conn {
	t.Helper()
	ref, ok := h.lookup(token)
	if !ok {
		t.Fatal("the token unlocks nothing")
	}
	c := &conn{
		room:      ref.room,
		pid:       ref.pid,
		isHost:    !ref.remote && ref.room.isHostParticipant(ref.pid),
		remote:    ref.remote,
		tokenHash: hashToken(token),
		send:      make(chan []byte, sendBuffer),
		closed:    make(chan struct{}),
	}
	if !ref.room.attach(c) {
		t.Fatal("the session is over")
	}
	return c
}

func report(rm *room, c *conn, media CouchMediaRef, position float64) {
	rm.onClientMessage(c, Envelope{
		Type: msgHostState,
		Data: mustJSON(CouchHostStateCommand{Media: media, Playing: true, PositionSeconds: position}),
	})
}

// queued is every frame a socket-less connection has been sent so far.
func queued(c *conn) []Envelope {
	var frames []Envelope
	for {
		select {
		case raw := <-c.send:
			var env Envelope
			_ = json.Unmarshal(raw, &env)
			frames = append(frames, env)
		default:
			return frames
		}
	}
}

// AllowsAnon must scope a cookie to exactly the session's current media, move
// the allowance on a media switch, and revoke it on end.
func TestAllowsScopedToCurrentMedia(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{
		"title:t1": {ID: "mf1", TitleID: ptr("t1")},
		"title:t2": {ID: "mf2", TitleID: ptr("t2")},
	}}
	h := newTestHub(t, fm)

	rm, hostP, _, created, err := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Fatal("a first session must report itself as created")
	}
	if !h.allows(hostP.ID, "mf1") {
		t.Fatal("followers should be allowed to stream the current media")
	}
	if h.allows(hostP.ID, "mf2") {
		t.Fatal("followers must not stream unrelated media")
	}
	if h.allows("bogus-participant", "mf1") {
		t.Fatal("an unknown participant must never be allowed")
	}

	// hosting again (a refresh, another device) with new media moves the allowance once that
	// device reports what it plays: until then the media still playing stays allowed
	t2 := CouchMediaRef{Kind: "movie", TitleID: "t2"}
	_, _, token2, reclaimed, err := h.createOrReclaim(context.Background(), host(1), t2, "")
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed {
		t.Fatal("a reclaim must not report itself as created, or hosting is double-counted")
	}
	if h.byHost[1] != rm {
		t.Fatal("reclaim should reuse the same room, not spawn a duplicate")
	}
	if !h.allows(hostP.ID, "mf1") || h.allows(hostP.ID, "mf2") {
		t.Fatal("hosting again switched the media before the device played it")
	}
	report(rm, attachDevice(t, h, token2), t2, 0)
	if !h.allows(hostP.ID, "mf2") || h.allows(hostP.ID, "mf1") {
		t.Fatal("after a media switch only the new media is allowed")
	}

	if !h.endByHostToken(token2) {
		t.Fatal("the host should be able to end the session")
	}
	if h.allows(hostP.ID, "mf2") {
		t.Fatal("after end, nothing is allowed")
	}
}

// A follower (anonymous) joins, is scoped to the current media, and loses access
// on leave.
func TestJoinFollowerAndLeave(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	rm, _, _, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")

	p, ftoken, role, err := h.join(rm, nil, false) // anonymous
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if role != "follower" || !p.IsAnonymous || p.DisplayName == "" {
		t.Fatalf("expected an anonymous follower with a generated name, got %+v role=%s", p, role)
	}
	if !h.allows(p.ID, "mf1") {
		t.Fatal("follower should stream the current media")
	}
	h.leaveByToken(ftoken)
	if h.allows(p.ID, "mf1") {
		t.Fatal("after leave, the follower is denied")
	}
}

// The host's account joining by code is another device than the one playing: it becomes a
// remote even without asking, so it never broadcasts as a second host and its leaving keeps
// the session.
func TestHostAccountJoinsAsRemote(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	rm, hostP, hostToken, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")

	p, token, role, err := h.join(rm, host(1), false)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if role != roleRemote || p.ID != hostP.ID {
		t.Fatalf("the host's account joined as %q (participant %s), want a remote of %s", role, p.ID, hostP.ID)
	}
	if ref, ok := h.lookup(token); !ok || !ref.remote {
		t.Fatalf("the second device's token is not a remote's: %+v", ref)
	}
	if session := rm.snapshotFor(p.ID, role); len(session.Participants) != 1 {
		t.Fatalf("a remote takes a seat: %d participants", len(session.Participants))
	}

	h.leaveByToken(token)
	rm.mu.Lock()
	live := rm.live
	rm.mu.Unlock()
	if !live {
		t.Fatal("the second device leaving ended the session")
	}
	if _, ok := h.lookup(hostToken); !ok {
		t.Fatal("the playing device's token stopped working")
	}
}

// The host's account hosting on a second device hands the session over with that device's
// first report: until then the first device plays for everyone, then it is a remote (told by
// a fresh hello, its token too), its late reports are dropped, and a connection that looked
// its token up before the handover still comes in as a remote.
func TestHandoverMovesTheSeatWithTheNewDevicesFirstReport(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{
		"title:t1": {ID: "mf1", TitleID: ptr("t1")},
		"title:t2": {ID: "mf2", TitleID: ptr("t2")},
	}}
	h := newTestHub(t, fm)
	ctx := context.Background()
	t1, t2 := CouchMediaRef{Kind: "movie", TitleID: "t1"}, CouchMediaRef{Kind: "movie", TitleID: "t2"}

	rm, hostP, laptopToken, _, _ := h.createOrReclaim(ctx, host(1), t1, "")
	laptop := attachDevice(t, h, laptopToken)
	report(rm, laptop, t1, 100)

	_, tvHost, tvToken, created, err := h.createOrReclaim(ctx, host(1), t2, "")
	if err != nil || created || tvHost != hostP {
		t.Fatalf("hosting on a second device: created %v, same host %v, %v", created, tvHost == hostP, err)
	}
	// a connection that looked the laptop's token up before the handover and attaches after it
	late := &conn{room: rm, pid: hostP.ID, isHost: true, tokenHash: laptop.tokenHash,
		send: make(chan []byte, sendBuffer), closed: make(chan struct{})}

	report(rm, laptop, t1, 102)
	if ref, _ := h.lookup(laptopToken); ref.remote || rm.currentMedia() != t1 {
		t.Fatal("the first device stopped playing for everyone before the second one played")
	}

	tv := attachDevice(t, h, tvToken)
	report(rm, tv, t2, 5)
	if rm.roleOf(laptop) != roleRemote || rm.roleOf(tv) != roleHost {
		t.Fatalf("after the handover: laptop %s, tv %s", rm.roleOf(laptop), rm.roleOf(tv))
	}
	if ref, _ := h.lookup(laptopToken); !ref.remote {
		t.Fatal("the laptop's token would bring it back as a host")
	}
	var hello CouchHello
	frames := queued(laptop)
	if len(frames) == 0 || frames[0].Type != msgHello || json.Unmarshal(frames[0].Data, &hello) != nil || hello.Role != roleRemote {
		t.Fatalf("the laptop was not told it is a remote: %+v", frames)
	}

	report(rm, laptop, t1, 104)
	rm.mu.Lock()
	state, playing := rm.state, rm.playing
	connected := hostP.connCount
	rm.mu.Unlock()
	if state.Media != t2 || state.PositionSeconds != 5 || playing != tv {
		t.Fatalf("a remote's report moved the session: %+v", state)
	}

	if !rm.attach(late) || rm.roleOf(late) != roleRemote {
		t.Fatal("a connection attached after the handover with the laptop's token plays as a host")
	}
	if connected != 1 {
		t.Fatalf("the host is present on %d devices, want the tv only", connected)
	}
}

func TestParticipantCap(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	h.maxParticipants = 3 // host + 2 followers
	rm, _, _, _, _ := h.createOrReclaim(context.Background(), host(1), CouchMediaRef{Kind: "movie", TitleID: "t1"}, "")

	for i := 0; i < 2; i++ {
		if _, _, _, err := h.join(rm, nil, false); err != nil {
			t.Fatalf("follower %d should fit: %v", i, err)
		}
	}
	if _, _, _, err := h.join(rm, nil, false); !errors.Is(err, errRoomFull) {
		t.Fatalf("expected errRoomFull past the cap, got %v", err)
	}
}
