package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/httpx"
)

// "Connect a device": a signed-in web user shows a one-time code as a QR
// (couchverse://connect?server=...&code=...) and a phone scans it to sign in.
const connectCodeTTL = 10 * time.Minute

// ConnectCode is a one-time sign-in code for the account that created it.
type ConnectCode struct {
	Code      string `json:"code"`
	ExpiresIn int    `json:"expiresIn" doc:"Seconds until the code expires."`
}

type connectCodeOutput struct{ Body ConnectCode }

func (p *Profile) CreateConnectCode(ctx context.Context, _ *struct{}) (*connectCodeOutput, error) {
	code, codeHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	if err := p.store.CreateConnectCode(ctx, codeHash, UserFrom(ctx).ID, connectCodeTTL); err != nil {
		return nil, err
	}
	return &connectCodeOutput{Body: ConnectCode{Code: code, ExpiresIn: int(connectCodeTTL.Seconds())}}, nil
}

// ConnectRedemption trades a connect code for a device token.
type ConnectRedemption struct {
	Code string `json:"code" minLength:"1"`
	DeviceInfo
}

type connectInput struct {
	Body ConnectRedemption

	remoteAddr string
	userAgent  string
}

func (in *connectInput) Resolve(ctx huma.Context) []error {
	in.remoteAddr = ctx.RemoteAddr()
	in.userAgent = ctx.Header("User-Agent")
	return nil
}

func (a *Handlers) Connect(ctx context.Context, in *connectInput) (*deviceTokenOutput, error) {
	if !a.pairingLimiter.allow(in.remoteAddr) {
		return nil, httpx.Fail(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
	}
	userID, err := a.store.RedeemConnectCode(ctx, HashToken(in.Body.Code))
	if errors.Is(err, db.ErrNotFound) {
		return nil, httpx.Fail(http.StatusUnauthorized, "invalid_code", "the code is unknown, used or expired")
	}
	if err != nil {
		return nil, err
	}
	user, err := a.store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, httpx.Fail(http.StatusUnauthorized, "invalid_code", "the code is unknown, used or expired")
	}
	token, err := issueDeviceToken(ctx, a.store, user, in.Body.DeviceInfo, in.userAgent)
	if err != nil {
		return nil, err
	}
	return &deviceTokenOutput{Body: *token}, nil
}

func (s *Store) CreateConnectCode(ctx context.Context, codeHash []byte, userID int64, ttl time.Duration) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO connect_codes (code_hash, user_id, expires_at) VALUES ($1, $2, now() + make_interval(secs => $3))`,
		codeHash, userID, ttl.Seconds())
	return err
}

// RedeemConnectCode consumes an unexpired code and returns its user; a code works
// once.
func (s *Store) RedeemConnectCode(ctx context.Context, codeHash []byte) (int64, error) {
	var userID int64
	err := s.db.QueryRow(ctx,
		`DELETE FROM connect_codes WHERE code_hash = $1 AND expires_at > now() RETURNING user_id`,
		codeHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, db.ErrNotFound
	}
	return userID, err
}

func (s *Store) DeleteExpiredConnectCodes(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM connect_codes WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
