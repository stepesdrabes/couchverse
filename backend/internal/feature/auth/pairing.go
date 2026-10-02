package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/httpx"
)

// Pairing signs in a device without a keyboard (RFC 8628 style): the device
// shows a short user code, a signed-in user approves it on another device, and
// the device's poll then returns its token exactly once.
const (
	pairingTTL      = 10 * time.Minute
	pairingInterval = 5 * time.Second
	// consonants only, so a code can never spell a word (RFC 8628, section 6.1)
	userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"
	userCodeLength   = 8
)

// Pairing is a started pairing. The device keeps DeviceCode secret, shows
// UserCode and polls with DeviceCode every Interval seconds.
type Pairing struct {
	DeviceCode string `json:"deviceCode" doc:"Secret the device polls with; never shown to anyone."`
	UserCode   string `json:"userCode" doc:"Shown on the device as XXXX-XXXX for a signed-in user to approve."`
	VerifyPath string `json:"verifyPath" doc:"Where the code is approved on the server's web app; render the server URL plus this path as a QR code."`
	ExpiresIn  int    `json:"expiresIn" doc:"Seconds until both codes expire."`
	Interval   int    `json:"interval" doc:"Seconds to wait between polls."`
}

type startPairingInput struct {
	Body DeviceInfo

	remoteAddr string
}

func (in *startPairingInput) Resolve(ctx huma.Context) []error {
	in.remoteAddr = ctx.RemoteAddr()
	return nil
}

type pairingOutput struct{ Body Pairing }

func (a *Handlers) StartPairing(ctx context.Context, in *startPairingInput) (*pairingOutput, error) {
	if !a.pairingLimiter.allow(in.remoteAddr) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many pairing attempts, try again in a minute")
	}
	deviceCode, deviceCodeHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		userCode := newUserCode()
		err := a.store.CreatePairing(ctx, deviceCodeHash, userCode, in.Body, pairingTTL)
		if err == nil {
			return &pairingOutput{Body: Pairing{
				DeviceCode: deviceCode,
				UserCode:   formatUserCode(userCode),
				VerifyPath: "/pair?code=" + formatUserCode(userCode),
				ExpiresIn:  int(pairingTTL.Seconds()),
				Interval:   int(pairingInterval.Seconds()),
			}}, nil
		}
		// a user code collision is astronomically rare; retry once with a new one
		if attempt > 0 || !db.IsUniqueViolation(err) {
			return nil, err
		}
	}
}

// PairingStatus answers a poll. Device is set exactly once, on the poll that
// sees the approval.
type PairingStatus struct {
	Status string       `json:"status" enum:"pending,approved,denied,expired"`
	Device *DeviceToken `json:"device,omitempty"`
}

type PairingPoll struct {
	DeviceCode string `json:"deviceCode" minLength:"1"`
}

type pollPairingInput struct {
	Body PairingPoll

	userAgent string
}

func (in *pollPairingInput) Resolve(ctx huma.Context) []error {
	in.userAgent = ctx.Header("User-Agent")
	return nil
}

type pairingStatusOutput struct{ Body PairingStatus }

func (a *Handlers) PollPairing(ctx context.Context, in *pollPairingInput) (*pairingStatusOutput, error) {
	p, err := a.store.PollPairing(ctx, HashToken(in.Body.DeviceCode))
	if err != nil {
		return nil, err
	}
	switch {
	case p.tooSoon:
		return nil, httpx.Fail(http.StatusTooManyRequests, "slow_down", "polling faster than the interval")
	case p.expired:
		return &pairingStatusOutput{Body: PairingStatus{Status: "expired"}}, nil
	case p.denied:
		return &pairingStatusOutput{Body: PairingStatus{Status: "denied"}}, nil
	case p.approvedBy == 0:
		return &pairingStatusOutput{Body: PairingStatus{Status: "pending"}}, nil
	}
	user, err := a.store.UserByID(ctx, p.approvedBy)
	if err != nil {
		return nil, err
	}
	token, err := issueDeviceToken(ctx, a.store, user, p.device, in.userAgent)
	if err != nil {
		return nil, err
	}
	return &pairingStatusOutput{Body: PairingStatus{Status: "approved", Device: token}}, nil
}

// PairingRequest is what a user sees before approving a device.
type PairingRequest struct {
	UserCode   string    `json:"userCode"`
	DeviceName string    `json:"deviceName"`
	Platform   string    `json:"platform" enum:"ios,ipados,tvos,android,androidtv"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type userCodeInput struct {
	Code string `path:"code" doc:"The user code, with or without the dash, any case."`
}

type pairingRequestOutput struct{ Body PairingRequest }

func (p *Profile) PairingRequest(ctx context.Context, in *userCodeInput) (*pairingRequestOutput, error) {
	req, err := p.store.PendingPairing(ctx, normalizeUserCode(in.Code))
	if err != nil {
		return nil, err
	}
	return &pairingRequestOutput{Body: *req}, nil
}

type PairingApproval struct {
	DeviceName string `json:"deviceName,omitempty" maxLength:"60" doc:"Renames the device; it keeps its own name when empty."`
}

type approvePairingInput struct {
	Code string `path:"code" doc:"The user code, with or without the dash, any case."`
	Body *PairingApproval
}

func (p *Profile) ApprovePairing(ctx context.Context, in *approvePairingInput) (*struct{}, error) {
	name := ""
	if in.Body != nil {
		name = strings.TrimSpace(in.Body.DeviceName)
	}
	return nil, p.store.ResolvePairing(ctx, normalizeUserCode(in.Code), UserFrom(ctx).ID, name)
}

func (p *Profile) DenyPairing(ctx context.Context, in *userCodeInput) (*struct{}, error) {
	return nil, p.store.ResolvePairing(ctx, normalizeUserCode(in.Code), 0, "")
}

func newUserCode() string {
	b := make([]byte, userCodeLength)
	for i := range b {
		b[i] = userCodeAlphabet[randIndex(len(userCodeAlphabet))]
	}
	return string(b)
}

// randIndex draws uniformly from [0, n) by rejecting the bytes that would bias
// the modulo.
func randIndex(n int) int {
	limit := 256 - 256%n
	var b [1]byte
	for {
		_, _ = rand.Read(b[:])
		if int(b[0]) < limit {
			return int(b[0]) % n
		}
	}
}

func formatUserCode(code string) string {
	return code[:4] + "-" + code[4:]
}

// normalizeUserCode accepts what a person types: any case, spaces or a dash.
func normalizeUserCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Store

func (s *Store) CreatePairing(ctx context.Context, deviceCodeHash []byte, userCode string, d DeviceInfo, ttl time.Duration) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO device_pairings (device_code_hash, user_code, device_name, platform, expires_at)
		 VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5))`,
		deviceCodeHash, userCode, d.DeviceName, d.Platform, ttl.Seconds())
	return err
}

type polledPairing struct {
	device     DeviceInfo
	approvedBy int64
	denied     bool
	expired    bool
	tooSoon    bool
}

// PollPairing records a poll and reports the pairing's state. An approved,
// denied or expired pairing is consumed by the poll that sees it, so a token is
// issued at most once.
func (s *Store) PollPairing(ctx context.Context, deviceCodeHash []byte) (*polledPairing, error) {
	var p polledPairing
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var approvedBy *int64
		var expiresAt time.Time
		var polledAt *time.Time
		err := tx.QueryRow(ctx,
			`SELECT device_name, platform, approved_by, denied, expires_at, polled_at
			 FROM device_pairings WHERE device_code_hash = $1 FOR UPDATE`, deviceCodeHash).
			Scan(&p.device.DeviceName, &p.device.Platform, &approvedBy, &p.denied, &expiresAt, &polledAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ErrNotFound
		}
		if err != nil {
			return err
		}
		now := time.Now()
		if approvedBy != nil {
			p.approvedBy = *approvedBy
		}
		p.expired = !now.Before(expiresAt)
		pending := p.approvedBy == 0 && !p.denied && !p.expired
		// a second of slack absorbs network jitter between polls at the interval
		p.tooSoon = pending && polledAt != nil && now.Sub(*polledAt) < pairingInterval-time.Second
		if pending {
			_, err = tx.Exec(ctx, `UPDATE device_pairings SET polled_at = now() WHERE device_code_hash = $1`, deviceCodeHash)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM device_pairings WHERE device_code_hash = $1`, deviceCodeHash)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PendingPairing finds an unexpired pairing still waiting for a decision.
func (s *Store) PendingPairing(ctx context.Context, userCode string) (*PairingRequest, error) {
	var r PairingRequest
	err := s.db.QueryRow(ctx,
		`SELECT user_code, device_name, platform, expires_at FROM device_pairings
		 WHERE user_code = $1 AND approved_by IS NULL AND NOT denied AND expires_at > now()`, userCode).
		Scan(&r.UserCode, &r.DeviceName, &r.Platform, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.UserCode = formatUserCode(r.UserCode)
	return &r, nil
}

// ResolvePairing approves a pending pairing for userID (optionally renaming the
// device) or, with userID 0, denies it.
func (s *Store) ResolvePairing(ctx context.Context, userCode string, userID int64, deviceName string) error {
	var approvedBy *int64
	if userID != 0 {
		approvedBy = &userID
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE device_pairings
		 SET approved_by = $2, denied = $2::bigint IS NULL,
		     device_name = CASE WHEN $3 = '' THEN device_name ELSE $3 END
		 WHERE user_code = $1 AND approved_by IS NULL AND NOT denied AND expires_at > now()`,
		userCode, approvedBy, deviceName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredPairings(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM device_pairings WHERE expires_at <= now() - interval '1 hour'`)
	return tag.RowsAffected(), err
}
