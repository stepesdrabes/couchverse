package playback

import (
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

// Module bundles the streaming and transcode-admin handlers.
type Module struct {
	stream *Stream
	admin  *AdminTranscode
}

func NewModule(stream *Stream, admin *AdminTranscode) *Module {
	return &Module{stream: stream, admin: admin}
}

const tag httpx.Tag = "playback"

func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.User, httpx.Localized(tag.Op("getPlayback", http.MethodGet, "/playback/{kind}/{id}")), m.stream.Playback)
	resolve := tag.Op("resolvePlayback", http.MethodPost, "/playback/{kind}/{id}")
	resolve.Summary = "Decide how a device plays a movie or an episode"
	resolve.Description = "The body is the device's capability profile; the payload never offers a stream the profile does not cover. " +
		"Tiers: direct (the source file), remux (the source video copied into fMP4 HLS, with the audio copied or transcoded " +
		"and subtitles as renditions), transcode (the H.264 SDR ladder, or an instant-play session)."
	huma.Register(rt.User, httpx.Localized(resolve), m.stream.ResolvePlayback)

	// everything below plays one media file, authorized by the grant in its path
	media := rt.Media
	session := tag.Created("createStreamSession", http.MethodPost, "/jit")
	session.Middlewares = huma.Middlewares{httpx.Guard(media, m.stream.requireJIT)}
	huma.Register(media, session, m.stream.CreateSession)
	huma.Register(media, tag.NoContent("keepStreamSessionAlive", http.MethodPost, "/jit/{sid}/keepalive"), m.stream.SessionKeepalive)
	huma.Register(media, tag.NoContent("stopStreamSession", http.MethodDelete, "/jit/{sid}"), m.stream.StopSession)

	apiErr := httpx.ErrorResponse(media)
	httpx.Raw(media, streamMediaFileOp(apiErr), m.stream.Serve)
	httpx.Raw(media, streamFrameOp(apiErr), m.stream.Frame)
	httpx.Raw(media, hlsMasterOp(apiErr), m.stream.HLSMaster)
	httpx.Raw(media, hlsSubtitleFileOp(apiErr), m.stream.HLSSubtitleFile)
	httpx.Raw(media, hlsFileOp(apiErr), m.stream.HLSFile)
	httpx.Raw(media, sessionFileOp(apiErr), m.stream.SessionFile)

	huma.Register(rt.Admin, tag.Op("adminGetTranscodeInfo", http.MethodGet, "/transcode/info"), m.admin.Info)
	huma.Register(rt.Admin, tag.Op("adminListActiveTranscodes", http.MethodGet, "/transcode/active"), m.admin.Active)
	huma.Register(rt.Admin, tag.Accepted("adminEnqueueTranscode", http.MethodPost, "/media-files/{id}/transcode"), m.admin.Enqueue)
	huma.Register(rt.Admin, tag.Op("adminListVariants", http.MethodGet, "/media-files/{id}/variants"), m.admin.ListVariants)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteVariant", http.MethodDelete, "/transcode-variants/{id}"), m.admin.DeleteVariant)
}

// The byte-stream routes below stay plain handlers; their operations describe
// them so typed clients still get the URLs, parameters and content types.

const playlistType = "application/vnd.apple.mpegurl"

var (
	textSchema   = &huma.Schema{Type: huma.TypeString}
	binarySchema = &huma.Schema{Type: huma.TypeString, Format: "binary"}
	// hlsNameSchema is what hlsFileRe accepts for the variant and file segments.
	hlsNameSchema = &huma.Schema{Type: huma.TypeString, Pattern: hlsFileRe.String()}
	// segmentTypes are the media types of init sections and segments.
	segmentTypes = []string{"video/mp4", "audio/mp4", "video/mp2t"}
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

func streamMediaFileOp(apiErr *huma.Response) huma.Operation {
	types := make([]string, 0, len(contentTypes))
	for _, t := range contentTypes {
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	slices.Sort(types)

	op := tag.Op("streamMediaFile", http.MethodGet, "/stream")
	op.Summary = "Stream a media file for direct play"
	op.Description = "Serves the source file with HTTP range support: a Range header answers 206 with that byte range. " +
		"404 codes: not_found, source_deleted (the source was removed after transcoding), file_missing."
	op.Parameters = []*huma.Param{
		{Name: "Range", In: "header", Description: "Byte range to read, e.g. bytes=0-1048575.", Schema: textSchema}}
	op.Responses = map[string]*huma.Response{
		"200":     response("The whole file.", binarySchema, types...),
		"206":     response("The requested byte range.", binarySchema, types...),
		"416":     {Description: "The range lies outside the file."},
		"default": apiErr,
	}
	return op
}

func streamFrameOp(apiErr *huma.Response) huma.Operation {
	zero := 0.0
	op := tag.Op("getStreamFrame", http.MethodGet, "/frame")
	op.Summary = "Get a seek-preview still from a video"
	op.Description = "A 240px-wide JPEG of the source video, cached per 5-second bucket. " +
		"404 codes: not_found (no video source on disk), frame_failed."
	op.Parameters = []*huma.Param{{
		Name: "t", In: "query",
		Description: "Position in seconds, rounded down to its 5-second bucket and clamped to the duration.",
		Schema:      &huma.Schema{Type: huma.TypeInteger, Minimum: &zero, Default: 0},
	}}
	op.Responses = map[string]*huma.Response{
		"200":     response("The frame.", binarySchema, "image/jpeg"),
		"default": apiErr,
	}
	return op
}

func hlsMasterOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getHlsMaster", http.MethodGet, "/hls/master.m3u8")
	op.Summary = "Get a multivariant playlist of a media file's prepared renditions"
	op.Description = "Use the URL from the playback payload, which picks the playlist for the device. " +
		"The playlist pairs every video rendition with an AAC stereo audio group and, when asked, a surround group; " +
		"subtitles are WebVTT renditions and an I-frame playlist backs trick play. 404 when nothing it needs is ready."
	op.Parameters = []*huma.Param{
		{Name: "video", In: "query", Description: "original: the copied source video (outside the adaptive ladder); " +
			"ladder: the transcoded renditions; legacy: MPEG-TS variants prepared before HLS v2.",
			Schema: &huma.Schema{Type: huma.TypeString, Enum: []any{"original", "ladder", "legacy"}, Default: "ladder"}},
		{Name: "surround", In: "query", Description: "Comma-separated multichannel codecs the client takes (ac3, eac3) for the surround audio group.",
			Schema: textSchema},
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The multivariant playlist.", textSchema, playlistType),
		"default": apiErr,
	}
	return op
}

func hlsSubtitleFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getHlsSubtitleFile", http.MethodGet, "/hls/subtitles/{id}/{file}")
	op.Summary = "Get a subtitle track as an HLS rendition"
	op.Description = "index.m3u8 segments the track along the video's 6-second grid; <n>.vtt is segment n, WebVTT " +
		"with an X-TIMESTAMP-MAP header. The track must belong to the granted file."
	op.Parameters = []*huma.Param{
		pathParam("id", "Subtitle track id.", &huma.Schema{Type: huma.TypeString, Format: "uuid"}),
		pathParam("file", "index.m3u8 or a segment.", &huma.Schema{Type: huma.TypeString, Pattern: subtitleFileRe.String()}),
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The playlist or a segment.", textSchema, playlistType, "text/vtt"),
		"default": apiErr,
	}
	return op
}

func hlsFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getHlsFile", http.MethodGet, "/hls/{variant}/{file}")
	op.Summary = "Get an HLS playlist, init section or segment of a prepared rendition"
	op.Description = "Serves the transcode cache: a rendition's index.m3u8, init.mp4 and fMP4 segments " +
		"(trickplay also has iframes.m3u8), and the MPEG-TS variants and multiaudio master prepared before HLS v2. " +
		"400 for names outside the allowed characters."
	op.Parameters = []*huma.Param{
		pathParam("variant", "Rendition: source, a ladder rung (e.g. 720p), audio-<stream>-<codec>, trickplay or multiaudio.", hlsNameSchema),
		pathParam("file", "Playlist, init section or segment file name.", hlsNameSchema),
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("A playlist or media.", binarySchema, append([]string{playlistType}, segmentTypes...)...),
		"default": apiErr,
	}
	return op
}

func sessionFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getStreamSessionFile", http.MethodGet, "/jit/{sid}/{file}")
	op.Summary = "Get a JIT session's playlist, init section or segment"
	op.Description = "master.m3u8 is the multivariant playlist from createStreamSession; index.m3u8 lists the whole " +
		"duration up front; a segment request waits until it is encoded, restarting the encoder on a far seek. " +
		"400 for a malformed session id; 404 codes: not_found (no such session for this viewer), segment_unavailable."
	op.Parameters = []*huma.Param{
		pathParam("sid", "Session id from createStreamSession.", &huma.Schema{Type: huma.TypeString, Pattern: sessionIDRe.String()}),
		pathParam("file", "master.m3u8, index.m3u8, init.mp4 or a segment name.",
			&huma.Schema{Type: huma.TypeString, Pattern: `^(master\.m3u8|index\.m3u8|init\.mp4|seg_\d{5}\.m4s)$`}),
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The playlist or media.", binarySchema, playlistType, "video/mp4"),
		"default": apiErr,
	}
	return op
}
