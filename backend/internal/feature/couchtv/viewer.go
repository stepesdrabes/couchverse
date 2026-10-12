package couchtv

import (
	"context"
	"net/url"
	"time"

	"couchverse/internal/feature/auth"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
)

// Viewer serves the channel to signed-in viewers: tuning in and the guide.
type Viewer struct {
	scheduler *Scheduler
	live      *Live
	grants    *grant.Signer
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

// CouchTvOnAir is a listing that may be missing: what is on, or what is next.
type CouchTvOnAir struct {
	_ struct{} `nullable:"true"`
	CouchTvListing
}

// CouchTvTune is everything a player needs to tune in to the channel.
type CouchTvTune struct {
	Grant          string           `json:"grant" doc:"The channel grant every channel URL carries; it expires, so tune in again on grant_expired."`
	StreamURL      string           `json:"streamUrl" doc:"The live multivariant playlist; its default audio follows the display language."`
	ServerTime     int64            `json:"serverTime" doc:"The server clock when the answer was made, in Unix milliseconds. Players hold the program date at server time minus three target durations."`
	TargetDuration int              `json:"targetDuration" doc:"The channel's EXT-X-TARGETDURATION, in seconds."`
	Now            *CouchTvOnAir    `json:"now" doc:"What is on air, a program or a break; null while nothing is."`
	Next           *CouchTvOnAir    `json:"next" doc:"The next program; null when nothing is planned."`
	Later          []CouchTvListing `json:"later" doc:"The programs after the next one, up to four."`
}

// CouchTvGuide is a stretch of the channel's programs.
type CouchTvGuide struct {
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	TimeZone string           `json:"timeZone" doc:"The channel's time zone, in which its days run."`
	Programs []CouchTvListing `json:"programs" doc:"The programs overlapping the stretch, in order."`
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
	token := v.grants.Issue(g, grant.ChannelTTL)
	stream := ChannelPath(token, "live/master.m3u8")
	if lang := httpx.LangFrom(ctx); lang != "" {
		stream += "?lang=" + url.QueryEscape(lang)
	}
	now := v.live.Channel.now()
	listings, err := v.scheduler.Store.Listings(ctx, now, now.Add(tuneLookahead), true)
	if err != nil {
		return nil, err
	}
	out := CouchTvTune{Grant: token, StreamURL: stream, ServerTime: now.UnixMilli(), TargetDuration: TargetDuration,
		Later: []CouchTvListing{}}
	for _, l := range listings {
		switch {
		case out.Now == nil && !l.StartsAt.After(now):
			out.Now = &CouchTvOnAir{CouchTvListing: l}
		case l.Kind != KindProgram:
		case out.Next == nil:
			out.Next = &CouchTvOnAir{CouchTvListing: l}
		case len(out.Later) < 4:
			out.Later = append(out.Later, l)
		}
	}
	return &tuneOutput{Body: out}, nil
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
	return &guideOutput{Body: CouchTvGuide{From: from, To: to, TimeZone: cfg.TimeZone, Programs: programs}}, nil
}
