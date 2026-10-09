package auth

import (
	"context"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// DeviceInfo names a native client when it signs in; it labels the session in
// every devices list.
type DeviceInfo struct {
	DeviceName string `json:"deviceName" minLength:"1" maxLength:"60" doc:"Shown in the account's devices list, e.g. \"Living room Apple TV\"."`
	Platform   string `json:"platform" enum:"ios,ipados,tvos,android,androidtv,desktop" doc:"desktop is an app on a computer, such as the CouchPush uploader."`
}

// DeviceSignIn is a password sign-in from a native client.
type DeviceSignIn struct {
	Credentials
	DeviceInfo
}

// DeviceToken is a new device session. The token is sent as
// `Authorization: Bearer <token>`; it is shown once and only its hash is stored.
type DeviceToken struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId" format:"uuid"`
	User     *User  `json:"user"`
}

type deviceTokenOutput struct{ Body DeviceToken }

type tokenInput struct {
	Body DeviceSignIn

	remoteAddr string
	userAgent  string
}

// Resolve captures the caller for throttling and the session row.
func (in *tokenInput) Resolve(ctx huma.Context) []error {
	in.remoteAddr = ctx.RemoteAddr()
	in.userAgent = ctx.Header("User-Agent")
	return nil
}

// Token signs a native client in with a password and returns a device token.
func (a *Handlers) Token(ctx context.Context, in *tokenInput) (*deviceTokenOutput, error) {
	user, err := a.authenticate(ctx, in.remoteAddr, in.Body.Credentials)
	if err != nil {
		return nil, err
	}
	token, err := issueDeviceToken(ctx, a.store, user, in.Body.DeviceInfo, in.userAgent)
	if err != nil {
		return nil, err
	}
	return &deviceTokenOutput{Body: *token}, nil
}

func issueDeviceToken(ctx context.Context, st *Store, user *User, d DeviceInfo, userAgent string) (*DeviceToken, error) {
	token, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	id, err := st.CreateSession(ctx, NewSession{
		TokenHash:  tokenHash,
		UserID:     user.ID,
		Kind:       "device",
		DeviceName: d.DeviceName,
		Platform:   d.Platform,
		UserAgent:  userAgent,
	})
	if err != nil {
		return nil, err
	}
	return &DeviceToken{Token: token, DeviceID: id, User: user}, nil
}

// Device is one signed-in browser or app of the account.
type Device struct {
	ID         string    `json:"id" format:"uuid"`
	Kind       string    `json:"kind" enum:"browser,device"`
	Name       string    `json:"name" doc:"The app's device name, or the browser and OS read from a browser's user agent."`
	Platform   string    `json:"platform" enum:"ios,ipados,tvos,android,androidtv,desktop,web"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	Current    bool      `json:"current" doc:"The session making this request."`
}

type devicesOutput struct{ Body []Device }

func (p *Profile) Devices(ctx context.Context, _ *struct{}) (*devicesOutput, error) {
	rows, err := p.store.UserSessions(ctx, UserFrom(ctx).ID)
	if err != nil {
		return nil, err
	}
	current := SessionFrom(ctx)
	devices := make([]Device, 0, len(rows))
	for _, r := range rows {
		d := Device{
			ID:         r.ID,
			Kind:       r.Kind,
			Name:       r.DeviceName,
			Platform:   r.Platform,
			CreatedAt:  r.CreatedAt,
			LastSeenAt: r.LastSeenAt,
			Current:    current != nil && current.ID == r.ID,
		}
		if r.Kind == "browser" {
			d.Name, d.Platform = browserName(r.UserAgent), "web"
		}
		devices = append(devices, d)
	}
	return &devicesOutput{Body: devices}, nil
}

type deviceIDInput struct {
	ID string `path:"id" format:"uuid"`
}

// RevokeDevice signs one of the account's browsers or apps out.
func (p *Profile) RevokeDevice(ctx context.Context, in *deviceIDInput) (*struct{}, error) {
	return nil, p.store.DeleteUserSession(ctx, UserFrom(ctx).ID, in.ID)
}

// browserName summarizes a user agent as "Browser on OS" for the devices list;
// it only needs to tell a person's own browsers apart.
func browserName(ua string) string {
	browser := "Browser"
	for _, b := range []struct{ token, name string }{
		{"Edg/", "Edge"},
		{"OPR/", "Opera"},
		{"Firefox/", "Firefox"},
		{"Chrome/", "Chrome"},
		{"Safari/", "Safari"},
	} {
		if strings.Contains(ua, b.token) {
			browser = b.name
			break
		}
	}
	for _, o := range []struct{ token, name string }{
		{"TitanOS/", "Titan OS"},
		{"Android", "Android"},
		{"iPhone", "iOS"},
		{"iPad", "iPadOS"},
		{"Mac OS X", "macOS"},
		{"Windows", "Windows"},
		{"CrOS", "ChromeOS"},
		{"Linux", "Linux"},
	} {
		if strings.Contains(ua, o.token) {
			return browser + " on " + o.name
		}
	}
	return browser
}
