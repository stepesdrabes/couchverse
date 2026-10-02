package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/app"
	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/server"
)

// Seeded ids (testdata/seed.sql).
const (
	movieID        = "00000000-0000-4000-8000-000000000101"
	seriesID       = "00000000-0000-4000-8000-000000000102"
	draftID        = "00000000-0000-4000-8000-000000000103"
	season1ID      = "00000000-0000-4000-8000-000000000201"
	season2ID      = "00000000-0000-4000-8000-000000000202"
	pilotID        = "00000000-0000-4000-8000-000000000301"
	episode2ID     = "00000000-0000-4000-8000-000000000302"
	episode3ID     = "00000000-0000-4000-8000-000000000303"
	unknownID      = "00000000-0000-4000-8000-000000000999"
	movieFileID    = "00000000-0000-4000-8000-000000000501"
	hdrFileEpisode = pilotID
	altAudioID     = "00000000-0000-4000-8000-000000000502"
	hdrFileID      = "00000000-0000-4000-8000-000000000503"
	ladderFileID   = "00000000-0000-4000-8000-000000000504"
	thumbArtID     = "00000000-0000-4000-8000-000000000404"
	movieSubID     = "00000000-0000-4000-8000-000000000701"
	failedRungID   = "00000000-0000-4000-8000-000000000802"
	readyRungID    = "00000000-0000-4000-8000-000000000801"
	doneJobID      = "901"
	failedJobID    = "902"
	memberName     = "nora"
	privateMember  = "piet"
)

// apiCase is one request in the conformance run. Cases run in order, so later
// ones may depend on (or destroy) what earlier ones read.
type apiCase struct {
	op     string // operationId it exercises
	as     string // "admin", "nora" or "" for anonymous
	method string
	path   string
	body   any
	// upload sends the body as multipart/form-data with this file in a "file" part
	upload *upload
	// fields are the text parts sent alongside upload
	fields map[string]string
	// raw sends these bytes as application/octet-stream
	raw    []byte
	status int
	// save stores response fields for later paths: {"couch": "shareToken"}
	// makes {{couch}} expand to that value
	save map[string]string
}

type upload struct {
	name    string
	content []byte
}

// pngImage is a small valid image for the avatar, banner and artwork uploads.
func pngImage() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// TestAPIConformance drives every typed operation against a seeded database and
// validates each JSON response against the OpenAPI document, so the published
// contract cannot drift from what the handlers actually send.
func TestAPIConformance(t *testing.T) {
	env := newTestEnv(t)
	doc := server.OpenAPI()
	ops := operations(doc)

	covered := map[string]bool{}
	vars := map[string]string{}
	for i, c := range apiCases() {
		for k, v := range vars {
			c.path = strings.ReplaceAll(c.path, "{{"+k+"}}", v)
		}
		name := fmt.Sprintf("%02d %s %s %s", i, c.op, c.method, c.path)
		op, ok := ops[c.op]
		if !ok {
			t.Fatalf("%s: unknown operation %q", name, c.op)
		}
		if op.method != c.method || !op.matches(c.path) {
			t.Fatalf("%s: does not match %s %s", name, op.method, op.path)
		}
		covered[c.op] = true
		status, body, contentType := env.do(t, c)
		if status != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, status, c.status, body)
			continue
		}
		schema := op.responseSchema(doc, status)
		if schema == nil {
			continue
		}
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("%s: content type %q, want JSON", name, contentType)
			continue
		}
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Errorf("%s: invalid JSON: %v", name, err)
			continue
		}
		for k, field := range c.save {
			if obj, ok := value.(map[string]any); ok {
				vars[k] = fmt.Sprint(obj[field])
			}
		}
		res := &huma.ValidateResult{}
		huma.Validate(doc.Components.Schemas, schema, huma.NewPathBuffer([]byte{}, 0), huma.ModeReadFromServer, value, res)
		for _, err := range res.Errors {
			t.Errorf("%s: response does not match the spec: %v", name, err)
		}
	}

	for id, op := range ops {
		if op.typed && !covered[id] {
			t.Errorf("operation %s (%s %s) has no conformance case", id, op.method, op.path)
		}
	}
}

func apiCases() []apiCase {
	return []apiCase{
		{op: "getTheme", method: "GET", path: "/theme", status: 200},
		{op: "getMe", as: "nora", method: "GET", path: "/auth/me", status: 200},
		{op: "getMe", method: "GET", path: "/auth/me", status: 401},
		{op: "login", method: "POST", path: "/auth/login", body: map[string]string{"username": "nora", "password": "wrong"}, status: 401},
		{op: "getFeatures", as: "nora", method: "GET", path: "/features", status: 200},

		{op: "listGenres", as: "nora", method: "GET", path: "/genres?lang=cs", status: 200},
		{op: "getHome", as: "nora", method: "GET", path: "/home", status: 200},
		{op: "getHome", as: "admin", method: "GET", path: "/home?lang=cs", status: 200},
		{op: "browseTitles", as: "nora", method: "GET", path: "/titles?kind=series&sort=name", status: 200},
		{op: "browseTitles", as: "nora", method: "GET", path: "/titles?sort=bogus", status: 400},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/glass-harbor-2025?lang=cs", status: 200},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/static-bloom-2024", status: 200},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/unfinished-2026", status: 404},
		{op: "search", as: "nora", method: "GET", path: "/search?q=glass", status: 200},
		{op: "saveProgress", as: "nora", method: "PUT", path: "/progress", body: map[string]any{"titleId": movieID, "positionSeconds": 1900, "durationSeconds": 6720, "watchedSeconds": 10}, status: 204},
		{op: "saveProgress", as: "nora", method: "PUT", path: "/progress", body: map[string]any{"positionSeconds": 1, "durationSeconds": 2}, status: 400},
		{op: "saveProgressBeacon", as: "nora", method: "POST", path: "/progress", body: map[string]any{"episodeId": episode2ID, "positionSeconds": 700, "durationSeconds": 2640}, status: 204},
		{op: "listContinueWatching", as: "nora", method: "GET", path: "/me/continue-watching", status: 200},
		{op: "addToWatchlist", as: "nora", method: "PUT", path: "/me/watchlist/" + seriesID, status: 204},
		{op: "listWatchlist", as: "nora", method: "GET", path: "/me/watchlist", status: 200},
		{op: "removeFromWatchlist", as: "nora", method: "DELETE", path: "/me/watchlist/" + movieID, status: 204},
		{op: "removeFromWatchlist", as: "nora", method: "DELETE", path: "/me/watchlist/not-a-uuid", status: 404},

		{op: "adminListLibrary", as: "admin", method: "GET", path: "/admin/library?type=series&sort=size", status: 200},
		{op: "adminListLibrary", as: "nora", method: "GET", path: "/admin/library", status: 403},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + seriesID, status: 200},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + movieID, status: 200},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + unknownID, status: 404},
		{op: "adminGetTitleStorage", as: "admin", method: "GET", path: "/admin/titles/" + seriesID + "/storage", status: 200},
		{op: "adminGetEpisodeTranslations", as: "admin", method: "GET", path: "/admin/episodes/" + pilotID + "/translations", status: 200},
		{op: "adminCreateTitle", as: "admin", method: "POST", path: "/admin/titles", body: map[string]any{"kind": "movie", "name": "Brand New", "year": 2020}, status: 201},
		{op: "adminCreateTitle", as: "admin", method: "POST", path: "/admin/titles", body: map[string]any{"kind": "film", "name": "x"}, status: 400},
		{op: "adminUpdateTitle", as: "admin", method: "PATCH", path: "/admin/titles/" + movieID, body: map[string]any{"overview": "Changed", "year": nil, "genres": []string{"Comedy"}}, status: 200},
		{op: "adminSetTitleTranslation", as: "admin", method: "PATCH", path: "/admin/titles/" + movieID + "/translations/cs", body: map[string]string{"name": "N", "overview": "O"}, status: 204},
		{op: "adminDeleteTitleLanguage", as: "admin", method: "DELETE", path: "/admin/titles/" + seriesID + "/languages/en", status: 400},
		{op: "adminDeleteTitleLanguage", as: "admin", method: "DELETE", path: "/admin/titles/" + movieID + "/languages/cs", status: 204},
		{op: "adminCreateSeason", as: "admin", method: "POST", path: "/admin/titles/" + seriesID + "/seasons", body: map[string]any{"seasonNumber": 3, "name": "Three"}, status: 201},
		{op: "adminCreateEpisode", as: "admin", method: "POST", path: "/admin/seasons/" + season2ID + "/episodes", body: map[string]any{"episodeNumber": 2, "name": "Two"}, status: 201},
		{op: "adminUpdateEpisode", as: "admin", method: "PATCH", path: "/admin/episodes/" + episode3ID, body: map[string]any{"name": "Renamed"}, status: 200},
		{op: "adminSetEpisodeTranslation", as: "admin", method: "PATCH", path: "/admin/episodes/" + pilotID + "/translations/cs", body: map[string]string{"name": "A", "overview": "B"}, status: 204},
		{op: "adminBulkTitles", as: "admin", method: "POST", path: "/admin/titles/bulk", body: map[string]any{"ids": []string{draftID}, "action": "publish"}, status: 204},
		{op: "adminBulkTitles", as: "admin", method: "POST", path: "/admin/titles/bulk", body: map[string]any{"ids": []string{}, "action": "publish"}, status: 400},
		{op: "adminDeleteEpisode", as: "admin", method: "DELETE", path: "/admin/episodes/" + episode3ID, status: 204},
		{op: "adminDeleteSeason", as: "admin", method: "DELETE", path: "/admin/seasons/" + season2ID, status: 204},
		{op: "adminDeleteTitle", as: "admin", method: "DELETE", path: "/admin/titles/" + draftID, status: 204},

		{op: "getPreferences", as: "nora", method: "GET", path: "/me/preferences", status: 200},
		{op: "updatePreferences", as: "nora", method: "PUT", path: "/me/preferences", body: map[string]any{"publicProfile": true, "language": "cs", "subtitles": map[string]any{"fontSizePct": 120, "color": "#ffe600", "fontFamily": "serif", "backgroundOpacity": 40}}, status: 200},
		{op: "updatePreferences", as: "nora", method: "PUT", path: "/me/preferences", body: map[string]any{"subtitles": map[string]any{"fontSizePct": 900}}, status: 400},
		{op: "updateProfile", as: "nora", method: "PATCH", path: "/me/profile", body: map[string]any{"displayName": "Nora B", "bio": "Hi **there**"}, status: 200},
		{op: "uploadAvatar", as: "nora", method: "POST", path: "/me/avatar", upload: &upload{"avatar.png", pngImage()}, status: 200},
		{op: "uploadBanner", as: "nora", method: "POST", path: "/me/banner", upload: &upload{"banner.png", pngImage()}, status: 200},
		{op: "deleteBanner", as: "nora", method: "DELETE", path: "/me/banner", status: 200},
		{op: "deleteAvatar", as: "nora", method: "DELETE", path: "/me/avatar", status: 200},

		{op: "getMyStats", as: "nora", method: "GET", path: "/me/stats?lang=cs", status: 200},
		{op: "checkAchievements", as: "nora", method: "POST", path: "/me/achievements/check", status: 200},
		// throttled: answers rank null so the client keeps what it has
		{op: "checkAchievements", as: "nora", method: "POST", path: "/me/achievements/check", status: 200},
		{op: "getProfile", as: "admin", method: "GET", path: "/users/" + memberName + "/profile?lang=cs", status: 200},
		{op: "getProfile", as: "admin", method: "GET", path: "/users/" + privateMember + "/profile", status: 404},
		{op: "getLeaderboard", as: "nora", method: "GET", path: "/leaderboard?period=all", status: 200},
		{op: "getLeaderboard", as: "nora", method: "GET", path: "/leaderboard?period=week", status: 200},
		{op: "getLeaderboard", as: "nora", method: "GET", path: "/leaderboard?period=bogus", status: 400},
		{op: "adminGetRanks", as: "admin", method: "GET", path: "/admin/ranks", status: 200},
		{op: "adminUpdateRanksConfig", as: "admin", method: "PUT", path: "/admin/ranks/config", body: map[string]any{
			"rates": map[string]int{"videoMinute": 2, "movie": 100, "episode": 20, "couchHost": 50, "couchJoin": 25, "bronze": 50, "silver": 150, "gold": 400, "platinum": 1000},
			"tiers": []int{0, 500, 1500, 3500, 7000, 13000, 23000, 40000, 70000, 120000},
		}, status: 200},
		{op: "adminUpdateRanksConfig", as: "admin", method: "PUT", path: "/admin/ranks/config", body: map[string]any{
			"rates": map[string]int{"videoMinute": 2, "movie": 100, "episode": 20, "couchHost": 50, "couchJoin": 25, "bronze": 50, "silver": 150, "gold": 400, "platinum": 1000},
			"tiers": []int{5, 500, 1500, 3500, 7000, 13000, 23000, 40000, 70000, 120000},
		}, status: 400},
		{op: "adminGetAnalytics", as: "admin", method: "GET", path: "/admin/analytics/overview?days=7", status: 200},

		{op: "adminListUsers", as: "admin", method: "GET", path: "/admin/users", status: 200},
		{op: "adminCreateUser", as: "admin", method: "POST", path: "/admin/users", body: map[string]any{"username": "zed", "password": "secret"}, status: 201},
		{op: "adminCreateUser", as: "admin", method: "POST", path: "/admin/users", body: map[string]any{"username": "zed", "password": "secret"}, status: 409},
		{op: "adminUpdateUser", as: "admin", method: "PATCH", path: "/admin/users/3", body: map[string]any{"displayName": "Piet P", "role": "member"}, status: 200},
		{op: "adminDeleteUser", as: "admin", method: "DELETE", path: "/admin/users/4", status: 204},

		{op: "getPlayback", as: "nora", method: "GET", path: "/playback/movie/" + movieID + "?caps=hevc,av1&lang=cs", status: 200},
		{op: "getPlayback", as: "nora", method: "GET", path: "/playback/episode/" + hdrFileEpisode, status: 200},
		{op: "getPlayback", as: "nora", method: "GET", path: "/playback/trailer/" + movieID, status: 404},
		// no hardware encoder in tests, so instant play is off
		{op: "createStreamSession", as: "nora", method: "POST", path: "/stream/" + hdrFileID + "/sessions", body: map[string]any{"startAt": 30}, status: 412},
		{op: "keepStreamSessionAlive", as: "nora", method: "POST", path: "/stream/sessions/abcdef0123456789/keepalive", status: 404},
		{op: "adminGetTranscodeInfo", as: "admin", method: "GET", path: "/admin/transcode/info", status: 200},
		{op: "adminListActiveTranscodes", as: "admin", method: "GET", path: "/admin/transcode/active", status: 200},
		{op: "adminListVariants", as: "admin", method: "GET", path: "/admin/media-files/" + ladderFileID + "/variants", status: 200},
		{op: "adminEnqueueTranscode", as: "admin", method: "POST", path: "/admin/media-files/" + ladderFileID + "/transcode", body: map[string]any{"variants": []string{"480p"}}, status: 202},
		{op: "adminDeleteVariant", as: "admin", method: "DELETE", path: "/admin/transcode-variants/" + failedRungID, status: 204},
		{op: "adminUpdateMediaFile", as: "admin", method: "PATCH", path: "/admin/media-files/" + altAudioID, body: map[string]any{"audioLang": "de", "audioRole": "audio_alt"}, status: 204},
		{op: "adminDeleteMediaFile", as: "admin", method: "DELETE", path: "/admin/media-files/" + altAudioID, status: 204},

		{op: "adminCreateUpload", as: "admin", method: "POST", path: "/admin/uploads", body: map[string]any{"filename": "Paper Moon (1973).mp4", "size": 4}, status: 201, save: map[string]string{"upload": "id"}},
		{op: "adminListUploads", as: "admin", method: "GET", path: "/admin/uploads", status: 200},
		{op: "adminAppendUpload", as: "admin", method: "PUT", path: "/admin/uploads/{{upload}}?offset=0", raw: []byte("moov"), status: 200},
		{op: "adminGetUpload", as: "admin", method: "GET", path: "/admin/uploads/{{upload}}", status: 200},
		{op: "adminCompleteUpload", as: "admin", method: "POST", path: "/admin/uploads/{{upload}}/complete", body: map[string]any{"libraryKind": "movies"}, status: 200},
		{op: "adminCreateUpload", as: "admin", method: "POST", path: "/admin/uploads", body: map[string]any{"filename": "Abandoned.mkv", "size": 10}, status: 201, save: map[string]string{"dropped": "id"}},
		{op: "adminAbortUpload", as: "admin", method: "DELETE", path: "/admin/uploads/{{dropped}}", status: 204},

		{op: "createCouch", as: "nora", method: "POST", path: "/couch", body: map[string]any{"kind": "movie", "id": movieID}, status: 201, save: map[string]string{"couch": "shareToken"}},
		{op: "createCouch", method: "POST", path: "/couch", body: map[string]any{"kind": "movie", "id": movieID}, status: 401},
		{op: "getCouchInfo", as: "guest", method: "GET", path: "/couch/{{couch}}/info?lang=cs", status: 200},
		{op: "joinCouch", as: "guest", method: "POST", path: "/couch/{{couch}}/join", status: 200},
		{op: "getCouchPlayback", as: "guest", method: "GET", path: "/couch/{{couch}}/playback", status: 200},
		{op: "leaveCouch", as: "guest", method: "POST", path: "/couch/{{couch}}/leave", status: 204},
		{op: "endCouch", as: "admin", method: "POST", path: "/couch/{{couch}}/end", status: 403},
		{op: "endCouch", as: "nora", method: "POST", path: "/couch/{{couch}}/end", status: 204},

		{op: "adminListJobs", as: "admin", method: "GET", path: "/admin/jobs", status: 200},
		{op: "adminListJobs", as: "admin", method: "GET", path: "/admin/jobs?status=failed&mediaFileId=" + ladderFileID + "&limit=5", status: 200},
		{op: "adminListJobs", as: "admin", method: "GET", path: "/admin/jobs?status=bogus", status: 400},
		{op: "adminListJobs", as: "nora", method: "GET", path: "/admin/jobs", status: 403},
		{op: "adminRetryJob", as: "admin", method: "POST", path: "/admin/jobs/" + failedJobID + "/retry", status: 204},
		{op: "adminRetryJob", as: "admin", method: "POST", path: "/admin/jobs/" + doneJobID + "/retry", status: 404},
		// right after the retry: the runner may fail it again, but only back to pending
		{op: "adminCancelJob", as: "admin", method: "POST", path: "/admin/jobs/" + failedJobID + "/cancel", status: 204},
		{op: "adminCancelJob", as: "admin", method: "POST", path: "/admin/jobs/" + doneJobID + "/cancel", status: 404},

		// no TMDB key is configured, so the TMDB reads stop at no_tmdb_key
		{op: "adminSearchMetadata", as: "admin", method: "GET", path: "/admin/metadata/search?q=glass&kind=movie", status: 412},
		{op: "adminSearchMetadata", as: "admin", method: "GET", path: "/admin/metadata/search?q=glass", status: 400},
		{op: "adminListMetadataSeasons", as: "admin", method: "GET", path: "/admin/titles/" + seriesID + "/metadata/seasons", status: 412},
		{op: "adminListMetadataSeasons", as: "admin", method: "GET", path: "/admin/titles/" + movieID + "/metadata/seasons", status: 400},
		{op: "adminApplyMetadata", as: "admin", method: "POST", path: "/admin/titles/" + movieID + "/metadata/apply", body: map[string]int{"tmdbId": 1001}, status: 202},
		{op: "adminApplyMetadata", as: "admin", method: "POST", path: "/admin/titles/" + movieID + "/metadata/apply", body: map[string]int{"tmdbId": 0}, status: 400},
		{op: "adminImportEpisodes", as: "admin", method: "POST", path: "/admin/titles/" + seriesID + "/metadata/import-episodes", body: map[string][]int{"seasons": {1}}, status: 202},
		{op: "adminImportEpisodes", as: "admin", method: "POST", path: "/admin/titles/" + seriesID + "/metadata/import-episodes", status: 202},
		{op: "adminImportEpisodes", as: "admin", method: "POST", path: "/admin/titles/" + movieID + "/metadata/import-episodes", status: 400},

		// replacing the pilot's thumb keeps its row, so it can be deleted by id
		{op: "adminUploadArtwork", as: "admin", method: "POST", path: "/admin/artwork", upload: &upload{"thumb.png", pngImage()},
			fields: map[string]string{"ownerKind": "episode", "ownerId": pilotID, "kind": "thumb"}, status: 201},
		{op: "adminUploadArtwork", as: "admin", method: "POST", path: "/admin/artwork", upload: &upload{"thumb.gif", pngImage()},
			fields: map[string]string{"ownerKind": "episode", "ownerId": pilotID, "kind": "thumb"}, status: 400},
		{op: "adminUploadArtwork", as: "admin", method: "POST", path: "/admin/artwork", upload: &upload{"thumb.png", pngImage()},
			fields: map[string]string{"ownerKind": "user", "ownerId": "2", "kind": "avatar"}, status: 400},
		{op: "adminDeleteArtwork", as: "admin", method: "DELETE", path: "/admin/artwork/" + thumbArtID, status: 204},
		{op: "adminDeleteArtwork", as: "admin", method: "DELETE", path: "/admin/artwork/" + thumbArtID, status: 404},

		{op: "adminListSubtitles", as: "admin", method: "GET", path: "/admin/media-files/" + movieFileID + "/subtitles", status: 200},
		{op: "adminUploadSubtitle", as: "admin", method: "POST", path: "/admin/media-files/" + movieFileID + "/subtitles",
			upload: &upload{"commentary.vtt", []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n")},
			fields: map[string]string{"lang": "de", "label": "Deutsch"}, status: 201},
		{op: "adminUploadSubtitle", as: "admin", method: "POST", path: "/admin/media-files/" + movieFileID + "/subtitles",
			upload: &upload{"commentary.txt", []byte("hello")}, status: 400},
		{op: "adminUploadSubtitle", as: "admin", method: "POST", path: "/admin/media-files/" + unknownID + "/subtitles",
			upload: &upload{"commentary.vtt", []byte("WEBVTT\n")}, status: 404},
		{op: "adminDeleteSubtitle", as: "admin", method: "DELETE", path: "/admin/subtitles/" + movieSubID, status: 204},
		{op: "adminDeleteSubtitle", as: "admin", method: "DELETE", path: "/admin/subtitles/" + movieSubID, status: 404},

		{op: "adminGetSettings", as: "admin", method: "GET", path: "/admin/settings", status: 200},
		{op: "adminGetSettings", as: "nora", method: "GET", path: "/admin/settings", status: 403},
		{op: "adminUpdateSettings", as: "admin", method: "PUT", path: "/admin/settings", body: map[string]any{
			"tmdb.api_key": "",
			"transcode": map[string]any{"hwAccel": "auto", "ladder": []string{"720p", "480p"}, "preset": "veryfast",
				"maxConcurrent": 1, "jitEnabled": nil, "autoPrepare": true, "deleteSourceAfterTranscode": false},
			"features":   map[string]bool{"couchEnabled": true, "rankingsEnabled": true},
			"home":       map[string]int{"featuredCount": 4},
			"appearance": map[string]string{"accent": "#3b82f6"},
		}, status: 200},
		{op: "adminUpdateSettings", as: "admin", method: "PUT", path: "/admin/settings", body: map[string]any{
			"transcode": map[string]any{"hwAccel": "auto", "ladder": []string{"4k"}, "preset": "veryfast",
				"maxConcurrent": 1, "jitEnabled": nil, "autoPrepare": true, "deleteSourceAfterTranscode": false},
		}, status: 400},
		{op: "adminUpdateSettings", as: "admin", method: "PUT", path: "/admin/settings", body: map[string]any{"ranks": map[string]any{}}, status: 400},
		{op: "adminGetStorage", as: "admin", method: "GET", path: "/admin/storage", status: 200},
		{op: "adminGetOverview", as: "admin", method: "GET", path: "/admin/overview", status: 200},
		{op: "adminGetSystemStats", as: "admin", method: "GET", path: "/admin/system", status: 200},
		{op: "adminGetLive", as: "admin", method: "GET", path: "/admin/live", status: 200},
		{op: "adminGetHomeRows", as: "admin", method: "GET", path: "/admin/home-rows", status: 200},
		{op: "adminUpdateHomeRows", as: "admin", method: "PUT", path: "/admin/home-rows", body: []map[string]any{
			{"id": 1, "position": 1, "kind": "continue_watching", "genreId": nil, "label": "Continue watching", "enabled": true},
			{"kind": "genre", "genreId": 102, "label": "Science Fiction", "enabled": true},
		}, status: 200},
		{op: "adminUpdateHomeRows", as: "admin", method: "PUT", path: "/admin/home-rows", body: []map[string]any{{"kind": "genre", "label": "No genre"}}, status: 400},

		{op: "changePassword", as: "nora", method: "PATCH", path: "/me/password", body: map[string]string{"currentPassword": "wrong", "newPassword": "long enough"}, status: 400},
		{op: "changePassword", as: "nora", method: "PATCH", path: "/me/password", body: map[string]string{"currentPassword": "admin", "newPassword": "long enough"}, status: 204},
		{op: "logout", as: "admin", method: "POST", path: "/auth/logout", status: 204},
	}
}

type operation struct {
	id, method, path string
	op               *huma.Operation
	// typed operations answer JSON; raw ones (streams, uploads, sockets) are
	// documented but have no response schema to check
	typed bool
}

func operations(doc *huma.OpenAPI) map[string]operation {
	out := map[string]operation{}
	for path, item := range doc.Paths {
		for method, op := range map[string]*huma.Operation{
			"GET": item.Get, "POST": item.Post, "PUT": item.Put, "PATCH": item.Patch, "DELETE": item.Delete,
		} {
			if op == nil {
				continue
			}
			o := operation{id: op.OperationID, method: method, path: path, op: op}
			for status, resp := range op.Responses {
				if strings.HasPrefix(status, "2") && (status == "204" || resp.Content["application/json"] != nil) {
					o.typed = true
				}
			}
			out[op.OperationID] = o
		}
	}
	return out
}

// matches reports whether a concrete request path fits the operation's template.
func (o operation) matches(requestPath string) bool {
	requestPath, _, _ = strings.Cut(requestPath, "?")
	tmpl := strings.Split(strings.Trim(o.path, "/"), "/")
	got := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(tmpl) != len(got) {
		return false
	}
	for i := range tmpl {
		if strings.Contains(tmpl[i], "{") {
			continue
		}
		if tmpl[i] != got[i] {
			return false
		}
	}
	return true
}

func (o operation) responseSchema(doc *huma.OpenAPI, status int) *huma.Schema {
	resp := o.op.Responses[strconv.Itoa(status)]
	if resp == nil {
		resp = o.op.Responses["default"]
	}
	if resp == nil {
		return nil
	}
	if resp.Ref != "" {
		resp = doc.Components.Responses[strings.TrimPrefix(resp.Ref, "#/components/responses/")]
	}
	if mt := resp.Content["application/json"]; mt != nil {
		return mt.Schema
	}
	return nil
}

type testEnv struct {
	srv     *httptest.Server
	clients map[string]*http.Client
}

// newTestEnv migrates and seeds a throwaway database and serves the real app
// on it. It needs TEST_DATABASE_URL (a Postgres the test may create databases
// on) and skips without it.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	name := fmt.Sprintf("couchverse_test_%d", os.Getpid())
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := config.Config{AdminUsername: "admin", AdminPassword: "admin", DataDir: t.TempDir(), JobWorkers: 1}
	a, err := app.New(ctx, cfg, pool)
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	t.Cleanup(a.Close)
	seed, err := os.ReadFile("testdata/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	env := &testEnv{srv: httptest.NewServer(a.Handler()), clients: map[string]*http.Client{}}
	t.Cleanup(env.srv.Close)
	env.clients[""] = env.srv.Client()
	// an anonymous viewer that keeps cookies, like a couch follower without an account
	guestJar, _ := cookiejar.New(nil)
	env.clients["guest"] = &http.Client{Jar: guestJar}
	for _, user := range []string{"admin", memberName} {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar}
		status, body, _ := env.request(t, c, "POST", "/auth/login", map[string]string{"username": user, "password": "admin"})
		if status != http.StatusOK {
			t.Fatalf("login %s: %d %s", user, status, body)
		}
		env.clients[user] = c
	}
	return env
}

func (e *testEnv) do(t *testing.T, c apiCase) (int, []byte, string) {
	if c.raw != nil {
		return e.send(t, e.clients[c.as], c.method, c.path, bytes.NewReader(c.raw), "application/octet-stream")
	}
	if c.upload == nil {
		return e.request(t, e.clients[c.as], c.method, c.path, c.body)
	}
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	for name, value := range c.fields {
		_ = form.WriteField(name, value)
	}
	part, err := form.CreateFormFile("file", c.upload.name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(c.upload.content)
	_ = form.Close()
	return e.send(t, e.clients[c.as], c.method, c.path, &buf, form.FormDataContentType())
}

func (e *testEnv) request(t *testing.T, client *http.Client, method, path string, body any) (int, []byte, string) {
	t.Helper()
	if body == nil {
		return e.send(t, client, method, path, nil, "")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return e.send(t, client, method, path, bytes.NewReader(raw), "application/json")
}

func (e *testEnv) send(t *testing.T, client *http.Client, method, path string, body io.Reader, contentType string) (int, []byte, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, data, res.Header.Get("Content-Type")
}
