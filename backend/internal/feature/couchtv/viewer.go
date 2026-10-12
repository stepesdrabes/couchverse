package couchtv

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
)

// Viewer serves the channel to its viewers, signed in or guests: tuning in, the
// guide and the socket.
type Viewer struct {
	scheduler *Scheduler
	live      *Live
	grants    *grant.Signer
	hub       *Hub
	guests    *Guests
}

// CouchTvListing is an airing as viewers see it: a program with its title, or a break.
type CouchTvListing struct {
	ID            string    `json:"id" format:"uuid"`
	Kind          string    `json:"kind" enum:"program,break"`
	StartsAt      time.Time `json:"startsAt"`
	EndsAt        time.Time `json:"endsAt"`
	Fill          string    `json:"fill,omitempty" enum:"strip,marathon,reruns,movie,matinee,premiere,saga,encore,filler" doc:"The rule that chose the program (premiere, saga...); absent for a break."`
	TitleID       string    `json:"titleId,omitempty" format:"uuid"`
	TitleSlug     string    `json:"titleSlug,omitempty"`
	TitleKind     string    `json:"titleKind,omitempty" enum:"movie,series"`
	TitleName     string    `json:"titleName,omitempty"`
	EpisodeID     string    `json:"episodeId,omitempty" format:"uuid"`
	SeasonNumber  int       `json:"seasonNumber,omitempty"`
	EpisodeNumber int       `json:"episodeNumber,omitempty"`
	EpisodeName   string    `json:"episodeName,omitempty"`
	Overview      string    `json:"overview,omitempty" doc:"The episode's overview for an episode, else the title's."`
	PosterID      string    `json:"posterId,omitempty"`
	PosterVer     int64     `json:"posterVer,omitempty"`
	BackdropID    string    `json:"backdropId,omitempty"`
	BackdropVer   int64     `json:"backdropVer,omitempty"`
}

// CouchTvTune is everything a player needs to tune in to the channel.
type CouchTvTune struct {
	Grant          string           `json:"grant" doc:"The channel grant every channel URL carries; it expires, so tune in again on grant_expired."`
	StreamURL      string           `json:"streamUrl" doc:"The live multivariant playlist; its default audio follows the display language."`
	ServerTime     int64            `json:"serverTime" doc:"The server clock when the answer was made, in Unix milliseconds. Players hold the program date at server time minus three target durations."`
	TargetDuration int              `json:"targetDuration" doc:"The channel's EXT-X-TARGETDURATION, in seconds."`
	Languages      []string         `json:"languages" doc:"The audio and subtitle languages of the stream, in its playlists' order."`
	Now            *CouchTvListing  `json:"now,omitempty" doc:"What is on air, a program or a break; absent while nothing is."`
	Next           *CouchTvListing  `json:"next,omitempty" doc:"The next program; absent when nothing is planned."`
	Later          []CouchTvListing `json:"later" doc:"The programs after the next one, up to four."`
	ArtworkGrant   string           `json:"artworkGrant,omitempty" doc:"A guest's: lets them load artwork without an account (append it to artwork URLs as ?g=)."`
	Guest          *CouchTvGuest    `json:"guest,omitempty" doc:"The guest tuning in; absent for a signed-in viewer."`
}

// CouchTvGuide is a stretch of the channel's programs.
type CouchTvGuide struct {
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	TimeZone string           `json:"timeZone" doc:"The channel's time zone, in which its days run."`
	Days     []CouchTvDay     `json:"days" doc:"The broadcast days the stretch overlaps, in order."`
	Programs []CouchTvListing `json:"programs" doc:"The programs overlapping the stretch, in order."`
}

// CouchTvDay is a broadcast day: from its first anchor (say 06:00) to the next day's.
type CouchTvDay struct {
	Date     string    `json:"date" format:"date" doc:"The day's date in the channel's time zone."`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}

// ChannelPath is the URL of a route under a channel grant: /api/v1/tv/{grant}/<rest>.
func ChannelPath(g, rest string) string {
	return "/api/v1/tv/" + g + "/" + rest
}

type tuneOutput struct{ Body CouchTvTune }

// tuneLookahead is how far ahead tuning in looks for the next programs.
const tuneLookahead = 24 * time.Hour

// Tune hands a signed-in viewer the channel: a grant, the stream and what is on.
func (v *Viewer) Tune(ctx context.Context, _ *struct{}) (*tuneOutput, error) {
	g := grant.Grant{Scope: grant.Channel, Resource: grant.CouchTv}
	if u := auth.UserFrom(ctx); u != nil {
		g.Subject = u.ID
	}
	out, err := v.tune(ctx, g)
	if err != nil {
		return nil, err
	}
	return &tuneOutput{Body: out}, nil
}

type guestTuneInput struct {
	Cookie string `cookie:"couchverse_tv" doc:"The channel cookie set by joinCouchTv."`
	Token  string `header:"X-Couch-Tv-Token" doc:"The guest token from joinCouchTv with delivery=body."`
}

func errNoGuestTune() error {
	return httpx.Fail(http.StatusUnauthorized, "no_channel_guest", "join through the channel link first")
}

// TuneGuest hands a guest the live channel, under a grant that names them, so it
// stops working when the channel link does; their artwork comes with a grant too.
func (v *Viewer) TuneGuest(ctx context.Context, in *guestTuneInput) (*tuneOutput, error) {
	guest, err := v.guests.Lookup(ctx, guestToken(in.Token, in.Cookie))
	if errors.Is(err, errNoGuest) {
		return nil, errNoGuestTune()
	}
	if err != nil {
		return nil, err
	}
	out, err := v.tune(ctx, grant.Grant{Scope: grant.Channel, Resource: grant.CouchTv, Couch: uuid.MustParse(guest.ID)})
	if err != nil {
		return nil, err
	}
	out.ArtworkGrant = artwork.IssueGrant(v.grants, 0).Grant
	out.Guest = &CouchTvGuest{ID: guest.ID, Name: guest.Name}
	return &tuneOutput{Body: out}, nil
}

// tune is the channel under grant g and what is on.
func (v *Viewer) tune(ctx context.Context, g grant.Grant) (CouchTvTune, error) {
	token := v.grants.Issue(g, grant.ChannelTTL)
	stream := ChannelPath(token, "live/master.m3u8")
	if lang := httpx.LangFrom(ctx); lang != "" {
		stream += "?lang=" + url.QueryEscape(lang)
	}
	now := v.live.Channel.now()
	listings, err := v.scheduler.Store.Listings(ctx, now, now.Add(tuneLookahead), true)
	if err != nil {
		return CouchTvTune{}, err
	}
	out := CouchTvTune{Grant: token, StreamURL: stream, ServerTime: now.UnixMilli(), TargetDuration: TargetDuration,
		Languages: Languages, Later: []CouchTvListing{}}
	for _, l := range listings {
		switch {
		case out.Now == nil && !l.StartsAt.After(now):
			out.Now = &l
		case l.Kind != KindProgram:
		case out.Next == nil:
			out.Next = &l
		case len(out.Later) < 4:
			out.Later = append(out.Later, l)
		}
	}
	return out, nil
}

// Socket upgrades a viewer's connection to the channel's room: a signed-in
// viewer by their session, a guest by their guest token. coder/websocket refuses
// a cross-origin browser upgrade by default (the auth CSRF posture); native
// clients send no Origin.
func (v *Viewer) Socket(w http.ResponseWriter, r *http.Request) {
	var me CouchTvViewer
	var userID int64
	if u := auth.UserFrom(r.Context()); u != nil {
		me, userID = userViewer(u), u.ID
	} else {
		token := r.Header.Get(GuestTokenHeader)
		if c, err := r.Cookie(GuestCookie); err == nil && token == "" {
			token = c.Value
		}
		guest, err := v.guests.Lookup(r.Context(), token)
		if errors.Is(err, errNoGuest) {
			httpx.Error(w, http.StatusUnauthorized, "no_channel_guest", "sign in or join through the channel link first")
			return
		}
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		me = guestViewer(guest)
	}
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the handshake error
	}
	v.hub.serve(ws, me, userID)
}

type guideInput struct {
	From time.Time `query:"from" doc:"Start of the stretch (RFC 3339); now when omitted."`
	To   time.Time `query:"to" doc:"End of the stretch; a day after from when omitted, at most 8 days after it."`
}

type guideOutput struct{ Body CouchTvGuide }

// maxGuide bounds one guide read: today and the next 7 days.
const maxGuide = 8 * 24 * time.Hour

// Guide lists the programs of a stretch of the channel.
func (v *Viewer) Guide(ctx context.Context, in *guideInput) (*guideOutput, error) {
	from, to := in.From, in.To
	if from.IsZero() {
		from = v.live.Channel.now()
	}
	if to.IsZero() {
		to = from.Add(24 * time.Hour)
	}
	if !to.After(from) || to.Sub(from) > maxGuide {
		return nil, httpx.BadRequestError("to must be after from, by at most 8 days")
	}
	programs, err := v.scheduler.Store.Listings(ctx, from, to, false)
	if err != nil {
		return nil, err
	}
	cfg := LoadConfig(ctx, v.scheduler.Settings)
	return &guideOutput{Body: CouchTvGuide{From: from, To: to, TimeZone: cfg.TimeZone, Days: broadcastDays(cfg, from, to),
		Programs: programs}}, nil
}

// broadcastDays are the broadcast days overlapping [from, to).
func broadcastDays(cfg CouchTvConfig, from, to time.Time) []CouchTvDay {
	days := []CouchTvDay{}
	for date := cfg.dayOf(from); ; date = nextDate(date) {
		start, end := cfg.dayStart(date), cfg.dayStart(nextDate(date))
		if !start.Before(to) {
			return days
		}
		days = append(days, CouchTvDay{Date: date.Format(time.DateOnly), StartsAt: start, EndsAt: end})
	}
}
