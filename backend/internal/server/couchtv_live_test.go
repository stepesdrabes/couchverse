package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/config"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/grant"
	"couchverse/internal/hls"
	"couchverse/internal/media/mp4"
	"couchverse/internal/settings"
)

// writeRendition lays out a finished package rendition from an init section and
// one segment repeated for every duration.
func writeRendition(t *testing.T, dir string, info playback.RenditionInfo, init, segment []byte, durations ...float64) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &hls.Media{Version: 7, PlaylistType: "VOD", EndList: true, TargetDuration: 6}
	for i, d := range durations {
		name := fmt.Sprintf("seg_%05d.m4s", i)
		m.Segments = append(m.Segments, hls.Segment{Duration: d, URI: name, Map: &hls.Map{URI: "init.mp4"}})
		info.Duration += d
		if err := os.WriteFile(filepath.Join(dir, name), segment, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(info)
	for name, body := range map[string][]byte{"init.mp4": init, "index.m3u8": []byte(m.String()), "rendition.json": data} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../media/mp4/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func setFeatures(t *testing.T, env *testEnv, couchTv bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]bool{"couchEnabled": true, "rankingsEnabled": true, "downloadsEnabled": true, "couchTvEnabled": couchTv})
	if err := settings.NewStore(env.pool).Set(context.Background(), "features", body); err != nil {
		t.Fatal(err)
	}
}

// get fetches a channel URL (it carries its grant) and returns status, body and code.
func (e *testEnv) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	res, err := http.Get(e.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

// TestCouchTvPrepareJob runs couchtv_prepare over two programs about to air: it
// queues what their packages lack (the copied source of an H.264 movie, a 1080p
// rung for a 4K HDR HEVC episode), then, once a package is there, records the
// file as ready, fits the program's row to its cut and moves the break after it.
func TestCouchTvPrepareJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	clock := time.Now().Truncate(time.Minute).Add(time.Hour)
	p := &couchtv.Preparer{Store: couchtv.NewStore(env.pool), Files: library.NewStore(env.pool), Jobs: jobs.NewStore(env.pool),
		Settings: settings.NewStore(env.pool), DataDir: env.cfg.DataDir, Now: func() time.Time { return clock }}
	run := func() {
		t.Helper()
		if err := p.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	movieAt, pilotAt := clock.Add(2*time.Hour), clock.Add(4*time.Hour)
	if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, episode_id, media_file_id, fill) VALUES
		($1::timestamptz, $1::timestamptz + interval '6720.5 seconds', 'program', $3::uuid, NULL, $4::uuid, 'movie'),
		($1::timestamptz + interval '6720.5 seconds', $2::timestamptz, 'break', NULL, NULL, NULL, NULL),
		($2::timestamptz, $2::timestamptz + interval '2520 seconds', 'program', $5::uuid, $6::uuid, $7::uuid, 'strip')`,
		movieAt, pilotAt, movieID, movieFileID, seriesID, pilotID, hdrFileID); err != nil {
		t.Fatal(err)
	}
	queued := func() []string {
		rows, err := env.pool.Query(ctx, `SELECT payload->>'mediaFileId' || ' ' || (payload->>'variant') FROM jobs
			WHERE type = 'transcode_hls' AND status = 'pending' ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run()
	want := []string{movieFileID + " package", movieFileID + " trickplay",
		hdrFileID + " 1080p", hdrFileID + " package", hdrFileID + " trickplay"}
	if got := queued(); !slices.Equal(got, want) {
		t.Errorf("queued %v, want %v", got, want)
	}
	var next time.Time
	if err := env.pool.QueryRow(ctx, `SELECT run_at FROM jobs WHERE type = $1 AND payload->>'at' <> ''`, couchtv.PrepareJob).Scan(&next); err != nil ||
		next.Sub(clock) > 10*time.Minute || !next.After(clock) {
		t.Errorf("the next run is at %s (%v)", next, err)
	}

	// the movie's package arrives (4 x 6 s, the last 0.5 s): it airs to its end, cut at 18.5 s
	pkg := playback.PackageDir(env.cfg.DataDir, movieFileID)
	writeRendition(t, filepath.Join(pkg, "source"), playback.RenditionInfo{Codec: "avc1.640028", Width: 1920, Height: 1080, VideoRange: "SDR"},
		readFixture(t, "avc.init.mp4"), readFixture(t, "avc.seg.m4s"), 6, 6, 6, 0.5)
	writeRendition(t, filepath.Join(pkg, "audio-1-aac"), playback.RenditionInfo{Codec: "mp4a.40.2", Channels: "2", Default: true},
		readFixture(t, "aac.init.mp4"), readFixture(t, "aac.seg.m4s"), 6, 6, 6, 0.5)
	run()
	if got := queued(); !slices.Equal(got, want) {
		t.Errorf("queued again: %v", got)
	}
	var video string
	var onAir int
	if err := env.pool.QueryRow(ctx, `SELECT video, on_air_ms FROM couchtv_media WHERE media_file_id = $1`, movieFileID).Scan(&video, &onAir); err != nil ||
		video != "source" || onAir != 18500 {
		t.Errorf("couchtv_media %s %d (%v)", video, onAir, err)
	}
	type row struct {
		kind, state string
		starts, end time.Time
	}
	rows, err := env.pool.Query(ctx, `SELECT kind, state, starts_at, ends_at FROM couchtv_airings ORDER BY starts_at`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.kind, &x.state, &x.starts, &x.end)
	})
	if err != nil {
		t.Fatal(err)
	}
	cut := movieAt.Add(18500 * time.Millisecond)
	if len(got) != 3 || got[0].state != "ready" || !got[0].end.Equal(cut) || !got[1].starts.Equal(cut) ||
		!got[1].end.Equal(pilotAt) || got[2].state != "planned" {
		t.Errorf("rows %+v", got)
	}
}

// TestCouchTvChannel serves the live stream of a schedule around now through
// the real handlers: tuning in, the playlists, the files of what is on air and
// only those, and the channel grant's reach.
func TestCouchTvChannel(t *testing.T) {
	_, ffmpegErr := exec.LookPath("ffmpeg")
	env := newTestEnv(t, func(cfg *config.Config) {
		if ffmpegErr == nil {
			cfg.FFmpegPath = "ffmpeg"
		}
	})
	ctx := context.Background()
	setFeatures(t, env, true)

	// the movie's package: a copied source and English (default) and Czech audio,
	// 4 x 6 s; the audio runs 0.02 s longer per segment, so its last one overruns
	pkg := playback.PackageDir(env.cfg.DataDir, movieFileID)
	writeRendition(t, filepath.Join(pkg, "source"), playback.RenditionInfo{Codec: "avc1.64001f", Width: 1280, Height: 720,
		FrameRate: 24, VideoRange: "SDR", Peak: 900_000, Average: 800_000}, readFixture(t, "avc.init.mp4"), readFixture(t, "avc.seg.m4s"), 6, 6, 6, 6)
	for i, lang := range []string{"en", "cs"} {
		writeRendition(t, filepath.Join(pkg, fmt.Sprintf("audio-%d-aac", i+1)), playback.RenditionInfo{Codec: "mp4a.40.2", Channels: "2",
			StreamIndex: i + 1, Language: lang, Default: i == 0, Peak: 130_000, Average: 128_000},
			readFixture(t, "aac.init.mp4"), readFixture(t, "aac.seg.m4s"), 6.02, 6.02, 6.02, 6.02)
	}
	if err := os.MkdirAll(filepath.Join(env.cfg.DataDir, "subtitles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.cfg.DataDir, "subtitles", "glass-en.vtt"),
		[]byte("WEBVTT\n\n00:00:01.000 --> 00:00:04.000\nThe sea has a door.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// an airing that went on air half an hour ago, the movie now (after the
	// slate fills the hole), its break (not rendered: the slate) and one tomorrow
	now := time.Now().Truncate(time.Millisecond)
	program := now.Add(-20 * time.Second)
	var old, onAir, brk, later string
	insert := func(id *string, kind string, start, end time.Time, title, file any, origin, feed any) {
		t.Helper()
		if err := env.pool.QueryRow(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, media_file_id, fill, origin, feed)
			VALUES ($1, $2, $3, $4, $5, CASE WHEN $3 = 'program' THEN 'movie' END, $6, $7) RETURNING id`,
			start, end, kind, title, file, origin, feed).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	oldStart := now.Add(-30 * time.Minute)
	insert(&old, "program", oldStart, oldStart.Add(24*time.Second), movieID, movieFileID,
		`{"mediaSequence": {"video": 10}, "discontinuitySequence": 3}`,
		fmt.Sprintf(`{"start": %q, "parts": [{"kind": "program", "ms": 24000, "file": %q, "video": "source"}]}`, oldStart.Format(time.RFC3339Nano), movieFileID))
	insert(&onAir, "program", program, program.Add(24*time.Second), movieID, movieFileID, nil, nil)
	insert(&brk, "break", program.Add(24*time.Second), program.Add(90*time.Second), nil, nil, nil, nil)
	insert(&later, "program", now.Add(24*time.Hour), now.Add(24*time.Hour+24*time.Second), movieID, movieFileID, nil, nil)

	status, body, _ := env.request(t, env.clients[memberName], "GET", "/couchtv/tune?lang=en", nil)
	if status != http.StatusOK {
		t.Fatalf("tune: %d %s", status, body)
	}
	var tune struct {
		Grant      string `json:"grant"`
		StreamURL  string `json:"streamUrl"`
		ServerTime int64  `json:"serverTime"`
		Now        *struct {
			ID        string `json:"id"`
			TitleName string `json:"titleName"`
		} `json:"now"`
	}
	if err := json.Unmarshal(body, &tune); err != nil {
		t.Fatal(err)
	}
	if tune.Now == nil || tune.Now.ID != onAir || tune.Now.TitleName != "Glass Harbor" ||
		tune.StreamURL != "/api/v1/tv/"+tune.Grant+"/live/master.m3u8?lang=en" || time.Since(time.UnixMilli(tune.ServerTime)) > time.Minute {
		t.Fatalf("tune %s", body)
	}
	base := "/api/v1/tv/" + tune.Grant + "/"

	status, body = env.get(t, tune.StreamURL)
	if status != http.StatusOK || !strings.Contains(string(body), `LANGUAGE="en",DEFAULT=YES`) {
		t.Fatalf("master: %d\n%s", status, body)
	}
	status, body = env.get(t, base+"live/video.m3u8")
	video := string(body)
	if status != http.StatusOK || !strings.Contains(video, "../airings/"+onAir+"/video/seg_00003.m4s") ||
		!strings.Contains(video, "#EXT-X-TARGETDURATION:10") {
		t.Fatalf("video playlist: %d\n%s", status, body)
	}
	_, body = env.get(t, base+"live/audio-cs.m3u8")
	if !strings.Contains(string(body), "../airings/"+onAir+"/audio-cs/seg_00003.m4s?keep=5.940") {
		t.Errorf("the audio past the cut is not trimmed:\n%s", body)
	}
	_, body = env.get(t, base+"live/subs-en.m3u8")
	if !strings.Contains(string(body), "../airings/"+onAir+"/subs-en/0.vtt") {
		t.Errorf("subtitles:\n%s", body)
	}
	// the airing on air chained from the one before it, a hole of slate between
	var feed string
	var origin []byte
	if err := env.pool.QueryRow(ctx, `SELECT feed::text, origin FROM couchtv_airings WHERE id = $1 AND state = 'aired'`, onAir).Scan(&feed, &origin); err != nil {
		t.Fatalf("the airing on air was not stored: %v", err)
	}
	if !strings.Contains(feed, `"kind": "slate"`) || !strings.Contains(string(origin), `"discontinuitySequence": 4`) {
		t.Errorf("feed %s, origin %s", feed, origin)
	}

	// the files of the airing on air, and nothing else
	file := func(path string, want int, code string) []byte {
		t.Helper()
		status, body := env.get(t, base+path)
		if status != want || code != "" && !strings.Contains(string(body), `"`+code+`"`) {
			t.Errorf("%s: %d %.200s, want %d %s", path, status, body, want, code)
		}
		return body
	}
	if got := file("airings/"+onAir+"/video/seg_00001.m4s", 200, ""); string(got) != string(readFixture(t, "avc.seg.m4s")) {
		t.Error("the segment differs from the package's")
	}
	file("airings/"+onAir+"/video/init.mp4", 200, "")
	in, _ := mp4.ParseInit(readFixture(t, "aac.init.mp4"))
	trimmed := file("airings/"+onAir+"/audio-cs/seg_00003.m4s?keep=0.5", 200, "")
	if frags, err := mp4.ParseSegment(trimmed, in); err != nil || float64(frags[0].Duration)/float64(in.Tracks[0].Timescale) > 0.6 {
		t.Errorf("a trimmed segment: %v %+v", err, frags)
	}
	if got := file("airings/"+onAir+"/subs-en/0.vtt", 200, ""); !strings.Contains(string(got), "The sea has a door.") ||
		!strings.Contains(string(got), "X-TIMESTAMP-MAP=MPEGTS:126000") {
		t.Errorf("subtitle segment:\n%s", got)
	}
	file("airings/"+onAir+"/subs-cs/0.vtt", 404, "")
	file("airings/"+onAir+"/audio-de/seg_00000.m4s", 404, "")
	file("airings/"+onAir+"/video/rendition.json", 404, "")
	file("airings/"+onAir+"/video/index.m3u8", 404, "")
	file("airings/"+old+"/video/seg_00000.m4s", 404, "off_air")
	file("airings/"+later+"/video/seg_00000.m4s", 404, "")
	file("airings/"+uuid.NewString()+"/video/seg_00000.m4s", 404, "")
	file("live/master.m3u8.bak", 404, "")
	if got := file("slate/subs/empty.vtt", 200, ""); !strings.HasPrefix(string(got), "WEBVTT") {
		t.Errorf("empty subtitles %q", got)
	}
	if ffmpegErr == nil {
		init := file("slate/video/init.mp4", 200, "")
		vin, err := mp4.ParseInit(init)
		if err != nil {
			t.Fatal(err)
		}
		first, err1 := mp4.ParseSegment(file("slate/video/0.m4s", 200, ""), vin)
		third, err2 := mp4.ParseSegment(file("slate/video/2.m4s", 200, ""), vin)
		if err1 != nil || err2 != nil || third[0].BaseDecodeTime != first[0].BaseDecodeTime+2*first[0].Duration ||
			vin.Tracks[0].Width != 1920 || vin.Tracks[0].Height != 1080 {
			t.Errorf("slate copies %+v %+v (%v %v)", first, third, err1, err2)
		}
		if cut, err := mp4.ParseSegment(file("slate/video/1.m4s?keep=0.5", 200, ""), vin); err != nil ||
			float64(cut[0].Duration)/float64(vin.Tracks[0].Timescale) > 0.55 {
			t.Errorf("a trimmed slate copy %+v %v", cut, err)
		}
	}

	// a grant reaches only its own routes, and only while it lasts
	key, err := grant.LoadSecret(ctx, settings.NewStore(env.pool))
	if err != nil {
		t.Fatal(err)
	}
	signer := grant.NewSigner(key)
	media := signer.Issue(grant.Grant{Scope: grant.Media, Subject: 2, Resource: uuid.MustParse(movieFileID)}, time.Hour)
	if status, body := env.get(t, "/api/v1/tv/"+media+"/live/master.m3u8"); status != 403 || !strings.Contains(string(body), "invalid_grant") {
		t.Errorf("a media grant on the channel: %d %s", status, body)
	}
	if status, body := env.get(t, "/api/v1/media/"+tune.Grant+"/hls/source/index.m3u8"); status != 403 || !strings.Contains(string(body), "invalid_grant") {
		t.Errorf("a channel grant on the library: %d %s", status, body)
	}
	expired := signer.Sign(grant.Grant{Scope: grant.Channel, Subject: 2, Resource: grant.CouchTv, Expires: now.Add(-time.Minute)})
	if status, body := env.get(t, "/api/v1/tv/"+expired+"/live/video.m3u8"); status != 403 || !strings.Contains(string(body), "grant_expired") {
		t.Errorf("an expired grant: %d %s", status, body)
	}
	setFeatures(t, env, false)
	if status, body := env.get(t, tune.StreamURL); status != 404 || !strings.Contains(string(body), "feature_disabled") {
		t.Errorf("the channel switched off: %d %s", status, body)
	}
}
