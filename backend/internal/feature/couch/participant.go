package couch

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/google/uuid"

	"couchverse/internal/feature/auth"
)

// participant is one viewer in a couch session. A logged-in viewer has userID
// set and uses their real display name + avatar; an anonymous viewer gets a
// generated name and identicon seed and no real account. The unexported fields
// are never serialized to other clients.
type participant struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	AvatarID    *string `json:"avatarId,omitempty"`
	Seed        string  `json:"seed,omitempty"` // identicon seed (username, or a random seed for anon)
	IsHost      bool    `json:"isHost"`
	IsAnonymous bool    `json:"isAnonymous"`

	tokenHash string // hex SHA-256 of the current cookie token
	userID    int64  // 0 when anonymous
}

// newParticipant builds a participant from an optional logged-in user. A nil
// user yields an anonymous participant with a generated name + avatar seed.
func newParticipant(user *auth.User, isHost bool) *participant {
	p := &participant{ID: uuid.NewString(), IsHost: isHost}
	if user != nil {
		p.DisplayName = user.DisplayName
		p.AvatarID = user.AvatarID
		p.Seed = user.Username
		p.userID = user.ID
	} else {
		p.DisplayName = anonDisplayName()
		p.Seed = randToken(8)
		p.IsAnonymous = true
	}
	return p
}

func anonDisplayName() string {
	return anonAdjectives[randIndex(len(anonAdjectives))] + " " + anonNouns[randIndex(len(anonNouns))]
}

func randIndex(n int) int {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint32(b[:]) % uint32(n))
}
