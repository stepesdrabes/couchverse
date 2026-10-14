package couchtv

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/flags"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the Couch TV routes.
type Module struct {
	admin    *Admin
	titles   *Titles
	guests   *Guests
	viewer   *Viewer
	live     *Live
	settings *settings.Store
}

func NewModule(s *Scheduler, a *Analyzer, soundtracks *Soundtracks, live *Live, hub *Hub, guests *Guests, set *settings.Store,
	grants *grant.Signer) *Module {
	return &Module{admin: &Admin{scheduler: s}, titles: &Titles{analyzer: a, soundtracks: soundtracks}, guests: guests,
		viewer: &Viewer{scheduler: s, live: live, grants: grants, hub: hub, guests: guests}, live: live, settings: set}
}

const tag httpx.Tag = "couchtv"

// Register adds the channel's operations. The viewer ones and the stream are
// hidden while the couchTvEnabled flag is off; the admin ones are deliberately
// not, since the channel is set up and planned before it goes on air.
func (m *Module) Register(rt httpx.Routes) {
	viewer := huma.NewGroup(rt.User)
	viewer.UseMiddleware(httpx.Guard(viewer, flags.CouchTvOn(m.settings)))
	tune := httpx.Localized(tag.Op("tuneCouchTv", http.MethodGet, "/couchtv/tune"))
	tune.Summary = "Tune in to Couch TV"
	tune.Description = "Issues a channel grant and answers with the live stream's URL, the server clock and what is on now and next."
	huma.Register(viewer, tune, m.viewer.Tune)
	huma.Register(viewer, httpx.Localized(tag.Op("getCouchTvGuide", http.MethodGet, "/couchtv/guide")), m.viewer.Guide)

	// guests (each handler checks its own credentials) and the room, for everyone
	public := huma.NewGroup(rt.Public)
	public.UseMiddleware(httpx.Guard(public, flags.CouchTvOn(m.settings)))
	join := tag.Op("joinCouchTv", http.MethodPost, "/tv/join/{code}")
	join.Summary = "Join Couch TV through the channel link"
	join.Description = "Lets a viewer without an account watch the live channel under the name they type (D23): it sets the " +
		"channel cookie, scoped to /api/v1/tv, or answers with the guest token for delivery=body. 404 no_channel_link when " +
		"the link was rotated or turned off; at most 10 tries a minute from one address."
	huma.Register(public, join, m.guests.Join)
	guestTune := httpx.Localized(tag.Op("tuneCouchTvGuest", http.MethodGet, "/tv/tune"))
	guestTune.Summary = "Tune in to Couch TV as a guest"
	guestTune.Description = "tuneCouchTv for a guest, by the channel cookie or the X-Couch-Tv-Token header (401 " +
		"no_channel_guest without a working one): the live channel only, with an artwork grant."
	huma.Register(public, guestTune, m.viewer.TuneGuest)
	httpx.Raw(public, socketOp(public), m.viewer.Socket)

	// the live stream, authorized by the channel grant in its path
	channel := huma.NewGroup(rt.Channel)
	channel.UseMiddleware(httpx.Guard(channel, flags.CouchTvOn(m.settings)))
	apiErr := httpx.ErrorResponse(channel)
	httpx.Raw(channel, masterOp(apiErr), m.live.Master)
	httpx.Raw(channel, playlistOp(apiErr), m.live.Playlist)
	httpx.Raw(channel, airingFileOp(apiErr), m.live.AiringFile)
	httpx.Raw(channel, airingPartFileOp(apiErr), m.live.AiringPartFile)
	httpx.Raw(channel, slateFileOp(apiErr), m.live.SlateFile)
	httpx.Raw(channel, standbyFileOp(apiErr), m.live.StandbyFile)

	huma.Register(rt.Admin, tag.Op("adminGetCouchTv", http.MethodGet, "/couchtv"), m.admin.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateCouchTv", http.MethodPut, "/couchtv"), m.admin.Update)
	huma.Register(rt.Admin, tag.Op("adminListAirings", http.MethodGet, "/couchtv/airings"), m.admin.ListAirings)
	huma.Register(rt.Admin, tag.Created("adminInsertAiring", http.MethodPost, "/couchtv/airings"), m.admin.InsertAiring)
	huma.Register(rt.Admin, tag.Op("adminUpdateAiring", http.MethodPatch, "/couchtv/airings/{id}"), m.admin.UpdateAiring)
	huma.Register(rt.Admin, tag.Op("adminRegenerateDay", http.MethodPost, "/couchtv/days/{date}/regenerate"), m.admin.RegenerateDay)
	huma.Register(rt.Admin, tag.Op("adminGetCouchTvTitle", http.MethodGet, "/couchtv/titles/{id}"), m.titles.Get)
	update := tag.Op("adminUpdateCouchTvTitle", http.MethodPut, "/couchtv/titles/{id}")
	update.Description = "Stores an admin's edit of a title's channel facts. A part left out of the body goes back to the analysis " +
		"(the soundtrack to the finder), which picks it again; the trailer's scenes alone are picked again, never the whole title. " +
		"409 trailer_scene_gone when trailer names a scene the trailer no longer has, because the analysis picked it again meanwhile."
	huma.Register(rt.Admin, update, m.titles.Update)
	huma.Register(rt.Admin, tag.Accepted("adminAnalyzeCouchTvTitle", http.MethodPost, "/couchtv/titles/{id}/analyze"), m.titles.Analyze)
	search := tag.Accepted("adminSearchCouchTvSoundtracks", http.MethodPost, "/couchtv/titles/{id}/soundtracks/search")
	search.Summary = "Search a title's soundtrack on the web again"
	search.Description = "Queues a search of iTunes and Deezer for the title's own music (searchingSoundtrack until it ends): " +
		"the finder's search, or the admin's own query, whose candidates only offer more to choose from. 409 excluded " +
		"for a title off the channel."
	huma.Register(rt.Admin, search, m.titles.SearchSoundtrack)
	httpx.Raw(rt.Admin, soundtrackPreviewOp(httpx.ErrorResponse(rt.Admin)), m.titles.SoundtrackPreview)
	huma.Register(rt.Admin, tag.Op("adminCouchTvWaveform", http.MethodGet, "/couchtv/media-files/{id}/waveform"), m.titles.Waveform)
	huma.Register(rt.Admin, tag.Op("adminCouchTvRenderHealth", http.MethodGet, "/couchtv/render"), m.admin.RenderHealth)
	rotate := tag.Op("adminRotateCouchTvLink", http.MethodPost, "/couchtv/link")
	rotate.Description = "Makes a new channel link, turned on, and answers with its code this once (only its hash is kept); " +
		"the old link stops working and every guest who joined through it is signed out."
	huma.Register(rt.Admin, rotate, m.guests.Rotate)
	toggle := tag.Op("adminSetCouchTvLink", http.MethodPut, "/couchtv/link")
	toggle.Description = "Turns the channel link on or off; off signs every guest out. 409 no_channel_link when there is no link to turn on."
	huma.Register(rt.Admin, toggle, m.guests.Switch)
	bulletins := tag.Op("adminGetCouchTvBulletins", http.MethodGet, "/couchtv/bulletins")
	bulletins.Summary = "Get Couch TV's bulletins"
	bulletins.Description = "The newest weather and news bulletins couchtv_bulletins fetched for the breaks: the forecast for the " +
		"configured cities with ČHMÚ's warnings, and the day's top stories by how many outlets carry them. A kind is absent " +
		"before its first bulletin; a bad fetch leaves the last good one."
	huma.Register(rt.Admin, bulletins, m.admin.Bulletins)
}

// socketOp describes the room's WebSocket upgrade, which stays a plain handler.
// The frames it carries are described by `couchverse couchtv-schema`.
func socketOp(api huma.API) huma.Operation {
	op := tag.Op("couchTvSocket", http.MethodGet, "/tv/socket")
	op.Summary = "Open the Couch TV room's WebSocket"
	op.Description = "Upgrades to the channel's room: presence, reactions, schedule changes and the viewer's reports of " +
		"what they watch. A signed-in viewer is known by their session, a guest by the channel cookie or the " +
		"X-Couch-Tv-Token header (401 no_channel_guest otherwise). Cross-origin browser upgrades are refused."
	op.Security = []map[string][]string{{"cookieSession": {}}, {"bearerDevice": {}}, {}}
	op.Parameters = []*huma.Param{
		{Name: GuestCookie, In: "cookie", Description: "A guest's channel cookie, set by joinCouchTv.",
			Schema: &huma.Schema{Type: huma.TypeString}},
		{Name: GuestTokenHeader, In: "header", Description: "A guest's token from joinCouchTv with delivery=body.",
			Schema: &huma.Schema{Type: huma.TypeString}},
	}
	op.Responses = map[string]*huma.Response{
		"101":     {Description: "Switched to the WebSocket protocol."},
		"default": httpx.ErrorResponse(api),
	}
	return op
}

var (
	textSchema   = &huma.Schema{Type: huma.TypeString}
	binarySchema = &huma.Schema{Type: huma.TypeString, Format: "binary"}
)

func pathParam(name, doc string, schema *huma.Schema) *huma.Param {
	return &huma.Param{Name: name, In: "path", Required: true, Description: doc, Schema: schema}
}

func response(desc string, schema *huma.Schema, types ...string) *huma.Response {
	r := &huma.Response{Description: desc, Content: map[string]*huma.MediaType{}}
	for _, t := range types {
		r.Content[t] = &huma.MediaType{Schema: schema}
	}
	return r
}

func masterOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvMaster", http.MethodGet, "/live/master.m3u8")
	op.Summary = "Get the Couch TV multivariant playlist"
	op.Description = "One video variant covering everything that airs, Czech and English audio (each program's track in " +
		"that language, else its original) and subtitles (empty where a program has none). Use the URL from tuneCouchTv."
	op.Parameters = []*huma.Param{{Name: "lang", In: "query", Description: "Display language (ISO 639-1): the default audio.",
		Schema: textSchema}}
	op.Responses = map[string]*huma.Response{
		"200":     response("The multivariant playlist.", textSchema, playlistType),
		"default": apiErr,
	}
	return op
}

func playlistOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvPlaylist", http.MethodGet, "/live/{playlist}")
	op.Summary = "Get a Couch TV live media playlist"
	op.Description = "The last two minutes of a rendition: a discontinuity and a program date at every program, break " +
		"and stretch of slate, a constant 16 s target duration. 404 off_air when nothing is on air."
	op.Parameters = []*huma.Param{pathParam("playlist", "video.m3u8, audio-<lang>.m3u8 or subs-<lang>.m3u8.",
		&huma.Schema{Type: huma.TypeString, Pattern: playlistRe.String()})}
	op.Responses = map[string]*huma.Response{
		"200":     response("The media playlist.", textSchema, playlistType),
		"default": apiErr,
	}
	return op
}

func airingFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvAiringFile", http.MethodGet, "/airings/{airing}/{track}/{file}")
	op.Summary = "Get a file of what an airing airs"
	op.Description = "The init section or a segment of a program's or break's rendition, or a WebVTT segment of a " +
		"program's subtitles, as the playlists list them. Served while the airing is within ten minutes of its time on " +
		"air; 404 off_air otherwise."
	op.Parameters = []*huma.Param{
		pathParam("airing", "The airing.", &huma.Schema{Type: huma.TypeString, Format: "uuid"}),
		pathParam("track", "video, audio-<lang> or subs-<lang>.", &huma.Schema{Type: huma.TypeString, Pattern: `^(video|audio-[a-z]{2}|subs-[a-z]{2})$`}),
		pathParam("file", "init.mp4, a segment, or <n>.vtt for subtitles.", &huma.Schema{Type: huma.TypeString, Pattern: `^[A-Za-z0-9_.-]+$`}),
		{Name: "keep", In: "query", Description: "Cut the segment to its first seconds (a segment that runs past its item).",
			Schema: &huma.Schema{Type: huma.TypeNumber}},
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The file.", binarySchema, "video/mp4", "audio/mp4", "text/vtt"),
		"default": apiErr,
	}
	return op
}

func airingPartFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvAiringPartFile", http.MethodGet, "/airings/{airing}/parts/{part}/{track}/{file}")
	op.Summary = "Get a file of one part of what an airing airs"
	op.Description = "getCouchTvAiringFile for the airing's media parts after its first, as the playlists list " +
		"them: a break with bulletin slots airs in sections, a slot's own a bulletin when one is ready. Served " +
		"while the airing is within ten minutes of its time on air; 404 off_air otherwise."
	op.Parameters = []*huma.Param{
		pathParam("airing", "The airing.", &huma.Schema{Type: huma.TypeString, Format: "uuid"}),
		pathParam("part", "The media part, counted from 0 among the airing's parts that are not fillers.",
			&huma.Schema{Type: huma.TypeInteger, Minimum: ptr(0.0), Maximum: ptr(float64(maxParts))}),
		pathParam("track", "video, audio-<lang> or subs-<lang>.", &huma.Schema{Type: huma.TypeString, Pattern: `^(video|audio-[a-z]{2}|subs-[a-z]{2})$`}),
		pathParam("file", "init.mp4 or a segment.", &huma.Schema{Type: huma.TypeString, Pattern: `^[A-Za-z0-9_.-]+$`}),
		{Name: "keep", In: "query", Description: "Cut the segment to its first seconds (a segment that runs past its item).",
			Schema: &huma.Schema{Type: huma.TypeNumber}},
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The file.", binarySchema, "video/mp4", "audio/mp4", "text/vtt"),
		"default": apiErr,
	}
	return op
}

func slateFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvSlateFile", http.MethodGet, "/slate/{track}/{file}")
	op.Summary = "Get a file of the Couch TV slate"
	op.Description = "What airs when nothing else can and no standby card is rendered: video and audio init " +
		"sections, copy <n> of their looping segment, and subs/empty.vtt, a subtitle segment without cues."
	op.Parameters = []*huma.Param{
		pathParam("track", "video, audio or subs.", &huma.Schema{Type: huma.TypeString, Enum: []any{"video", "audio", "subs"}}),
		pathParam("file", "init.mp4, <n>.m4s or empty.vtt.", &huma.Schema{Type: huma.TypeString, Pattern: `^(init\.mp4|\d{1,6}\.m4s|empty\.vtt)$`}),
		{Name: "keep", In: "query", Description: "Cut the segment to its first seconds.", Schema: &huma.Schema{Type: huma.TypeNumber}},
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The file.", binarySchema, "video/mp4", "audio/mp4", "text/vtt"),
		"default": apiErr,
	}
	return op
}

func soundtrackPreviewOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("adminCouchTvSoundtrackPreview", http.MethodGet, "/couchtv/soundtracks/{id}/preview")
	op.Summary = "Play a soundtrack candidate's preview"
	op.Description = "The catalog's 30-second preview of a soundtrack candidate, from the server's copy (downloaded on " +
		"first play), so the browser never reaches the catalog itself: AAC in MP4 from iTunes, MP3 from Deezer. 502 " +
		"soundtrack_unavailable when the catalog does not deliver it."
	op.Parameters = []*huma.Param{pathParam("id", "The soundtrack candidate.", &huma.Schema{Type: huma.TypeString, Format: "uuid"})}
	op.Responses = map[string]*huma.Response{
		"200":     response("The preview.", binarySchema, "audio/mp4", "audio/mpeg"),
		"default": apiErr,
	}
	return op
}

func standbyFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getCouchTvStandbyFile", http.MethodGet, "/standby/{clip}/{track}/{file}")
	op.Summary = "Get a file of the Couch TV standby card"
	op.Description = "The rendered standby card, looped where a break is not ready or a program cannot air: video " +
		"and audio init sections and copy <n> of their segments (segment n mod S of pass n / S, retimed by whole passes)."
	op.Parameters = []*huma.Param{
		pathParam("clip", "The standby card's clip.", &huma.Schema{Type: huma.TypeString, Format: "uuid"}),
		pathParam("track", "video or audio.", &huma.Schema{Type: huma.TypeString, Enum: []any{"video", "audio"}}),
		pathParam("file", "init.mp4 or <n>.m4s.", &huma.Schema{Type: huma.TypeString, Pattern: `^(init\.mp4|\d{1,6}\.m4s)$`}),
		{Name: "keep", In: "query", Description: "Cut the segment to its first seconds.", Schema: &huma.Schema{Type: huma.TypeNumber}},
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The file.", binarySchema, "video/mp4", "audio/mp4"),
		"default": apiErr,
	}
	return op
}
