package server_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"couchverse/internal/config"
	"couchverse/internal/feature/downloads"
)

// TestDownloads prepares real MP4s with the prepare_download job: the source
// video copied for an Apple device, a 480p transcode for a browser profile, one
// shared file for two users asking for the same plan, and the cleanup removing
// what nobody wants. It needs ffmpeg.
func TestDownloads(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes video")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)
	w.setTranscode(ctx, `{"autoPrepare": false}`)

	// H.264 with AC-3 5.1 and two subtitle languages, in Matroska
	s := sample{title: "Harbor Lights (2025)", file: "Harbor Lights (2025).mkv",
		args: []string{"-map", "0:v", "-map", "2:a", "-c:v", "libx264", "-preset", "ultrafast", "-g", "48",
			"-c:a", "ac3", "-b:a", "384k", "-metadata:s:a:0", "language=eng"},
		subs: []string{"eng", "ces"}}
	path := filepath.Join(env.cfg.DataDir, "media", "movies", s.title, s.file)
	generate(t, path, s)
	w.ingest(ctx, path)
	w.drain(ctx)
	titleID := w.titleID(ctx, s.title)

	request := func(user string, body map[string]any) downloads.Download {
		t.Helper()
		body["kind"], body["id"] = "movie", titleID
		status, raw, _ := env.request(t, env.clients[user], "POST", "/me/downloads", body)
		var d downloads.Download
		if err := json.Unmarshal(raw, &d); status != 200 || err != nil {
			t.Fatalf("request download: %d %s", status, raw)
		}
		return d
	}
	get := func(user, id string) downloads.Download {
		t.Helper()
		status, raw, _ := env.request(t, env.clients[user], "GET", "/me/downloads/"+id, nil)
		var d downloads.Download
		if err := json.Unmarshal(raw, &d); status != 200 || err != nil {
			t.Fatalf("get download: %d %s", status, raw)
		}
		return d
	}

	apple := request(memberName, map[string]any{"quality": "original", "profile": appleProfile})
	browser := request(memberName, map[string]any{"quality": "480p", "audio": []string{"en"}, "profile": chromeProfile})
	if apple.Status != "queued" || browser.Status != "queued" || apple.URL != "" {
		t.Fatalf("before preparation: %+v %+v", apple, browser)
	}
	w.drain(ctx)

	apple = get(memberName, apple.ID)
	if apple.Status != "ready" || apple.URL == "" || apple.ExpiresAt == nil || apple.SizeBytes == 0 || apple.Height != 0 {
		t.Fatalf("apple download: %+v", apple)
	}
	if len(apple.Subtitles) != 2 || len(apple.Audio) != 1 || apple.Audio[0].Lang != "en" {
		t.Errorf("apple tracks: %+v %+v", apple.Audio, apple.Subtitles)
	}
	file := fetch(t, env, apple.URL, apple.SizeBytes)
	streams := probeStreams(t, file)
	want := []string{"video:h264", "audio:ac3:eng", "subtitle:mov_text:ces", "subtitle:mov_text:eng"}
	if !slices.Equal(streams, want) {
		t.Errorf("apple MP4 streams %v, want %v", streams, want)
	}
	if boxes := topBoxes(t, file); len(boxes) < 3 || boxes[0] != "ftyp" || slices.Index(boxes, "moov") > slices.Index(boxes, "mdat") {
		t.Errorf("not faststart: %v", boxes)
	}

	browser = get(memberName, browser.ID)
	if browser.Status != "ready" || browser.Height != 480 {
		t.Fatalf("browser download: %+v", browser)
	}
	streams = probeStreams(t, fetch(t, env, browser.URL, browser.SizeBytes))
	want = []string{"video:h264:480", "audio:aac:eng", "subtitle:mov_text:ces", "subtitle:mov_text:eng"}
	if !slices.Equal(streams, want) {
		t.Errorf("browser MP4 streams %v, want %v", streams, want)
	}

	// a range request resumes an interrupted transfer
	req, _ := http.NewRequest("GET", env.srv.URL+apple.URL, nil)
	req.Header.Set("Range", "bytes=0-99")
	resp, err := env.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || resp.ContentLength != 100 {
		t.Errorf("range: %d, %d bytes", resp.StatusCode, resp.ContentLength)
	}

	// the same plan for someone else is the same file, ready at once
	shared := request("admin", map[string]any{"quality": "original", "profile": appleProfile})
	if shared.Status != "ready" || shared.ID == apple.ID || shared.SizeBytes != apple.SizeBytes {
		t.Errorf("shared download: %+v", shared)
	}

	// a grant for another file cannot fetch it
	status, body, _ := env.request(t, env.clients[memberName], "POST", "/playback/movie/"+movieID, chromeProfile)
	var other struct{ Grant string }
	if err := json.Unmarshal(body, &other); status != 200 || err != nil {
		t.Fatalf("playback: %d %s", status, body)
	}
	_, fileID := filepath.Split(apple.URL)
	if status, _, _ := env.send(t, env.clients[""], "GET", "/media/"+other.Grant+"/downloads/"+fileID, nil, ""); status != 404 {
		t.Errorf("another file's grant fetched the download: %d", status)
	}

	// the cleanup keeps what someone still wants and removes the rest
	for _, d := range []downloads.Download{apple, browser} {
		if status, _, _ := env.request(t, env.clients[memberName], "DELETE", "/me/downloads/"+d.ID, nil); status != 204 {
			t.Fatalf("delete: %d", status)
		}
	}
	dl := downloads.NewStore(env.pool)
	if err := downloads.Sweep(ctx, dl, env.cfg.DataDir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(env.cfg.DataDir, "cache", "downloads"))
	if len(entries) != 1 || entries[0].Name() != fileID+".mp4" {
		t.Errorf("after cleanup: %v, want only %s.mp4", entries, fileID)
	}
	if get("admin", shared.ID).Status != "ready" {
		t.Error("the shared download went with the other user's")
	}
}

// fetch downloads a ready MP4 into a temp file.
func fetch(t *testing.T, env *testEnv, url string, size int64) string {
	t.Helper()
	resp, err := env.srv.Client().Get(env.srv.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "video/mp4" {
		t.Fatalf("fetch %s: %d %s", url, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	out := filepath.Join(t.TempDir(), "download.mp4")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if n, err := io.Copy(f, resp.Body); err != nil || n != size {
		t.Fatalf("fetch %s: %d bytes of %d: %v", url, n, size, err)
	}
	return out
}

// probeStreams lists a file's streams as type:codec[:height][:language].
func probeStreams(t *testing.T, path string) []string {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name,height:stream_tags=language",
		"-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var probe struct {
		Streams []struct {
			Type   string `json:"codec_type"`
			Codec  string `json:"codec_name"`
			Height int    `json:"height"`
			Tags   struct {
				Language string `json:"language"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	var streams []string
	for _, s := range probe.Streams {
		desc := s.Type + ":" + s.Codec
		if s.Type == "video" && s.Height != 720 {
			desc += ":" + strconv.Itoa(s.Height)
		}
		if s.Tags.Language != "" && s.Tags.Language != "und" {
			desc += ":" + s.Tags.Language
		}
		streams = append(streams, desc)
	}
	return streams
}

// topBoxes lists the MP4's top-level box types in file order.
func topBoxes(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var boxes []string
	var offset int64
	header := make([]byte, 16)
	for {
		if _, err := f.ReadAt(header[:8], offset); err != nil {
			return boxes
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		boxes = append(boxes, string(header[4:8]))
		if size == 1 {
			if _, err := f.ReadAt(header[8:16], offset+8); err != nil {
				return boxes
			}
			size = int64(binary.BigEndian.Uint64(header[8:16]))
		}
		if size < 8 {
			return boxes
		}
		offset += size
	}
}
