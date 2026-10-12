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
	viewer   *Viewer
	live     *Live
	settings *settings.Store
}

func NewModule(s *Scheduler, a *Analyzer, live *Live, set *settings.Store, grants *grant.Signer) *Module {
	return &Module{admin: &Admin{scheduler: s}, titles: &Titles{analyzer: a},
		viewer: &Viewer{scheduler: s, live: live, grants: grants}, live: live, settings: set}
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

	// the live stream, authorized by the channel grant in its path
	channel := huma.NewGroup(rt.Channel)
	channel.UseMiddleware(httpx.Guard(channel, flags.CouchTvOn(m.settings)))
	apiErr := httpx.ErrorResponse(channel)
	httpx.Raw(channel, masterOp(apiErr), m.live.Master)
	httpx.Raw(channel, playlistOp(apiErr), m.live.Playlist)
	httpx.Raw(channel, airingFileOp(apiErr), m.live.AiringFile)
	httpx.Raw(channel, slateFileOp(apiErr), m.live.SlateFile)
	httpx.Raw(channel, standbyFileOp(apiErr), m.live.StandbyFile)

	huma.Register(rt.Admin, tag.Op("adminGetCouchTv", http.MethodGet, "/couchtv"), m.admin.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateCouchTv", http.MethodPut, "/couchtv"), m.admin.Update)
	huma.Register(rt.Admin, tag.Op("adminListAirings", http.MethodGet, "/couchtv/airings"), m.admin.ListAirings)
	huma.Register(rt.Admin, tag.Op("adminRegenerateDay", http.MethodPost, "/couchtv/days/{date}/regenerate"), m.admin.RegenerateDay)
	huma.Register(rt.Admin, tag.Op("adminGetCouchTvTitle", http.MethodGet, "/couchtv/titles/{id}"), m.titles.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateCouchTvTitle", http.MethodPut, "/couchtv/titles/{id}"), m.titles.Update)
	huma.Register(rt.Admin, tag.Accepted("adminAnalyzeCouchTvTitle", http.MethodPost, "/couchtv/titles/{id}/analyze"), m.titles.Analyze)
	huma.Register(rt.Admin, tag.Op("adminCouchTvWaveform", http.MethodGet, "/couchtv/media-files/{id}/waveform"), m.titles.Waveform)
	huma.Register(rt.Admin, tag.Op("adminCouchTvRenderHealth", http.MethodGet, "/couchtv/render"), m.admin.RenderHealth)
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
		"and stretch of slate, a constant 10 s target duration. 404 off_air when nothing is on air."
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
