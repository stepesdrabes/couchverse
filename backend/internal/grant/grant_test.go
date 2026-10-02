package grant

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testSigner(now time.Time) *Signer {
	s := NewSigner([]byte("0123456789abcdef0123456789abcdef"))
	s.now = func() time.Time { return now }
	return s
}

func TestGrantsRoundTrip(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	s := testSigner(now)
	file := uuid.MustParse("00000000-0000-4000-8000-000000000501")
	couch := uuid.MustParse("6ba7b810-9dad-41d1-80b4-00c04fd430c8")
	token := s.Issue(Grant{Scope: Media, Subject: 42, Resource: file, Couch: couch}, time.Hour)
	if len(token) != 83 || strings.ContainsAny(token, "+/=") {
		t.Fatalf("token %q is not an 83-character URL-safe string", token)
	}
	g, err := s.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	want := Grant{Scope: Media, Subject: 42, Resource: file, Couch: couch, Expires: now.Add(time.Hour)}
	if g != want {
		t.Fatalf("got %+v, want %+v", g, want)
	}
}

func TestTamperedGrantsAreRejected(t *testing.T) {
	s := testSigner(time.Unix(1_790_000_000, 0))
	token := s.Issue(Grant{Scope: Media, Subject: 42, Resource: uuid.New()}, time.Hour)
	for i := range len(token) {
		flipped := []byte(token)
		flipped[i] ^= 1
		if _, err := s.Verify(string(flipped)); err == nil {
			t.Fatalf("flipping byte %d still verified", i)
		}
	}
	other := NewSigner([]byte("another key, another server....."))
	if _, err := other.Verify(token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a grant from another server verified: %v", err)
	}
	for _, bad := range []string{"", "x", "not-base64!", strings.Repeat("A", 83)} {
		if _, err := s.Verify(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("Verify(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestGrantsExpire(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	token := testSigner(now).Issue(Grant{Scope: Artwork, Subject: 7}, time.Minute)
	if _, err := testSigner(now.Add(59 * time.Second)).Verify(token); err != nil {
		t.Fatalf("valid grant rejected: %v", err)
	}
	if _, err := testSigner(now.Add(time.Minute)).Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired grant: %v", err)
	}
}
