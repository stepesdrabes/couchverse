package playback

import (
	"net/http"
	"reflect"
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
	stream := rt.Stream
	huma.Register(stream, httpx.Localized(tag.Op("getPlayback", http.MethodGet, "/playback/{kind}/{id}")), m.stream.Playback)

	session := tag.Created("createStreamSession", http.MethodPost, "/stream/{id}/sessions")
	session.Middlewares = huma.Middlewares{httpx.Guard(stream, m.stream.requireJIT)}
	huma.Register(stream, session, m.stream.CreateSession)
	huma.Register(stream, tag.NoContent("keepStreamSessionAlive", http.MethodPost, "/stream/sessions/{sid}/keepalive"), m.stream.SessionKeepalive)

	apiErr := errorResponse(stream)
	httpx.Raw(stream, streamMediaFileOp(apiErr), m.stream.Serve)
	httpx.Raw(stream, streamFrameOp(apiErr), m.stream.Frame)
	httpx.Raw(stream, hlsMasterOp(apiErr), m.stream.HLSMaster)
	httpx.Raw(stream, hlsFileOp(apiErr), m.stream.HLSFile)
	httpx.Raw(stream, sessionFileOp(apiErr), m.stream.SessionFile)

	huma.Register(rt.Admin, tag.Op("adminGetTranscodeInfo", http.MethodGet, "/transcode/info"), m.admin.Info)
	huma.Register(rt.Admin, tag.Op("adminListActiveTranscodes", http.MethodGet, "/transcode/active"), m.admin.Active)
	enqueue := tag.Op("adminEnqueueTranscode", http.MethodPost, "/media-files/{id}/transcode")
	enqueue.DefaultStatus = http.StatusAccepted
	huma.Register(rt.Admin, enqueue, m.admin.Enqueue)
	huma.Register(rt.Admin, tag.Op("adminListVariants", http.MethodGet, "/media-files/{id}/variants"), m.admin.ListVariants)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteVariant", http.MethodDelete, "/transcode-variants/{id}"), m.admin.DeleteVariant)
}

// The byte-stream routes below stay plain handlers; their operations describe
// them so typed clients still get the URLs, parameters and content types.

const (
	playlistType = "application/vnd.apple.mpegurl"
	segmentType  = "video/mp2t"
)

var (
	textSchema   = &huma.Schema{Type: huma.TypeString}
	binarySchema = &huma.Schema{Type: huma.TypeString, Format: "binary"}
	// hlsNameSchema is what hlsFileRe accepts for the variant and file segments.
	hlsNameSchema = &huma.Schema{Type: huma.TypeString, Pattern: hlsFileRe.String()}
)

func pathParam(name, doc string, schema *huma.Schema) *huma.Param {
	return &huma.Param{Name: name, In: "path", Required: true, Description: doc, Schema: schema}
}

func mediaFileParam() *huma.Param {
	return pathParam("id", "Media file id.", &huma.Schema{Type: huma.TypeString, Format: "uuid"})
}

func response(desc string, schema *huma.Schema, types ...string) *huma.Response {
	r := &huma.Response{Description: desc, Content: map[string]*huma.MediaType{}}
	for _, t := range types {
		r.Content[t] = &huma.MediaType{Schema: schema}
	}
	return r
}

// errorResponse is the API error envelope, as typed operations document it.
func errorResponse(api huma.API) *huma.Response {
	ref := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[httpx.APIError](), true, "")
	return response("Error", ref, "application/json")
}

func streamMediaFileOp(apiErr *huma.Response) huma.Operation {
	types := make([]string, 0, len(contentTypes))
	for _, t := range contentTypes {
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	slices.Sort(types)

	op := tag.Op("streamMediaFile", http.MethodGet, "/stream/{id}")
	op.Summary = "Stream a media file for direct play"
	op.Description = "Serves the source file with HTTP range support: a Range header answers 206 with that byte range. " +
		"404 codes: not_found, source_deleted (the source was removed after transcoding), file_missing."
	op.Parameters = []*huma.Param{mediaFileParam(),
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
	op := tag.Op("getStreamFrame", http.MethodGet, "/stream/{id}/frame")
	op.Summary = "Get a seek-preview still from a video"
	op.Description = "A 240px-wide JPEG of the source video, cached per 5-second bucket. " +
		"404 codes: not_found (no video source on disk), frame_failed."
	op.Parameters = []*huma.Param{mediaFileParam(), {
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
	op := tag.Op("getHlsMaster", http.MethodGet, "/stream/{id}/hls/master.m3u8")
	op.Summary = "Get the HLS master playlist of a media file's ready variants"
	op.Description = "404 when no variant is ready."
	op.Parameters = []*huma.Param{mediaFileParam()}
	op.Responses = map[string]*huma.Response{
		"200":     response("The master playlist.", textSchema, playlistType),
		"default": apiErr,
	}
	return op
}

func hlsFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getHlsFile", http.MethodGet, "/stream/{id}/hls/{variant}/{file}")
	op.Summary = "Get an HLS playlist or segment of a prepared variant"
	op.Description = "Serves the transcode cache: a variant's index.m3u8 and its MPEG-TS segments, " +
		"and the multiaudio variant's own master.m3u8. 400 for names outside the allowed characters."
	op.Parameters = []*huma.Param{
		mediaFileParam(),
		pathParam("variant", "Rendition name (e.g. 720p), source or multiaudio.", hlsNameSchema),
		pathParam("file", "Playlist or segment file name.", hlsNameSchema),
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("A playlist or a segment.", binarySchema, playlistType, segmentType),
		"default": apiErr,
	}
	return op
}

func sessionFileOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("getStreamSessionFile", http.MethodGet, "/stream/sessions/{sid}/{file}")
	op.Summary = "Get a JIT session's playlist or segment"
	op.Description = "index.m3u8 lists the whole duration up front; a segment request waits until it is " +
		"encoded, restarting the encoder on a far seek. 400 for a malformed session id; " +
		"404 codes: not_found (no such session), segment_unavailable."
	op.Parameters = []*huma.Param{
		pathParam("sid", "Session id from createStreamSession.", &huma.Schema{Type: huma.TypeString, Pattern: sessionIDRe.String()}),
		pathParam("file", "index.m3u8 or a segment name.", &huma.Schema{Type: huma.TypeString, Pattern: `^(index\.m3u8|seg_\d{5}\.ts)$`}),
	}
	op.Responses = map[string]*huma.Response{
		"200":     response("The playlist or a segment.", binarySchema, playlistType, segmentType),
		"default": apiErr,
	}
	return op
}
