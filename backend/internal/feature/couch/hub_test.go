package couch

import (
	"context"
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

func (fakePlayback) BuildPlayback(_ context.Context, _, _ string, _ *int64, _ []string) (*playback.PlaybackInfo, error) {
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

// AllowsAnon must scope a cookie to exactly the session's current media, move
// the allowance on a media switch, and revoke it on end.
func TestAllowsScopedToCurrentMedia(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{
		"title:t1": {ID: "mf1", TitleID: ptr("t1")},
		"title:t2": {ID: "mf2", TitleID: ptr("t2")},
	}}
	h := newTestHub(t, fm)

	rm, _, token, created, err := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Fatal("a first session must report itself as created")
	}
	if !h.allows(token, "mf1") {
		t.Fatal("host should be allowed to stream the current media")
	}
	if h.allows(token, "mf2") {
		t.Fatal("host must not stream unrelated media")
	}
	if h.allows("bogus-token", "mf1") {
		t.Fatal("an unknown token must never be allowed")
	}

	// reclaim (refresh/second tab) with new media moves the allowance
	_, _, token2, reclaimed, err := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t2"})
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed {
		t.Fatal("a reclaim must not report itself as created, or hosting is double-counted")
	}
	if h.byHost[1] != rm {
		t.Fatal("reclaim should reuse the same room, not spawn a duplicate")
	}
	if !h.allows(token2, "mf2") || h.allows(token2, "mf1") {
		t.Fatal("after a media switch only the new media is allowed")
	}

	if !h.endByHostToken(token2) {
		t.Fatal("the host should be able to end the session")
	}
	if h.allows(token2, "mf2") {
		t.Fatal("after end, nothing is allowed")
	}
}

// A follower (anonymous) joins, is scoped to the current media, and loses access
// on leave.
func TestJoinFollowerAndLeave(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	rm, _, _, _, _ := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t1"})

	p, ftoken, role, err := h.join(rm, nil) // anonymous
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if role != "follower" || !p.IsAnonymous || p.DisplayName == "" {
		t.Fatalf("expected an anonymous follower with a generated name, got %+v role=%s", p, role)
	}
	if !h.allows(ftoken, "mf1") {
		t.Fatal("follower should stream the current media")
	}
	h.leaveByToken(ftoken)
	if h.allows(ftoken, "mf1") {
		t.Fatal("after leave, the follower is denied")
	}
}

func TestParticipantCap(t *testing.T) {
	fm := &fakeMedia{files: map[string]*media.MediaFile{"title:t1": {ID: "mf1", TitleID: ptr("t1")}}}
	h := newTestHub(t, fm)
	h.maxParticipants = 3 // host + 2 followers
	rm, _, _, _, _ := h.createOrReclaim(context.Background(), host(1), mediaRef{Kind: "movie", TitleID: "t1"})

	for i := 0; i < 2; i++ {
		if _, _, _, err := h.join(rm, nil); err != nil {
			t.Fatalf("follower %d should fit: %v", i, err)
		}
	}
	if _, _, _, err := h.join(rm, nil); !errors.Is(err, errRoomFull) {
		t.Fatalf("expected errRoomFull past the cap, got %v", err)
	}
}
