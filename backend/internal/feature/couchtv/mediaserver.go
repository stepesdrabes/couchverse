package couchtv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/playback"
	"couchverse/internal/media"
)

// The render page loads everything from one localhost server: the broadcast bundle and the
// media its specs name. Media URLs describe what they are cut from, so the composer (in
// the app) writes them and the sidecar extracts each once, before a clip loads:
//
//	/media/v1/artwork/<artwork id>?v=<version>[&size=w780|w1920|1600x800]
//	/media/v1/frames/<media file id>/<start ms>_<count>/<n>.jpg   25 fps, 1080p, SDR
//	/media/v1/audio/<media file id>/<start ms>_<length ms>.flac    the default track, stereo
//	/media/v1/audio/<media file id>/<stream>/<start ms>_<length ms>.flac
//	                                         one stream (its ffprobe index) of the file, stereo with
//	                                         the dialogue kept up when it is multichannel (trailers)
//	/media/v1/soundtrack/<catalog>/<track id>/<start ms>_<length ms>.flac
//	                                         a catalog track's preview (soundtracks.go), stereo
//
// The v1 is the extraction recipe: a change to it changes every URL, so every clip key. The
// squeeze's program frames are never extracted: ffmpeg composites them from the source
// (composite.go), so only promo footage is drawn from these files. Chromium has no AAC
// decoder, so a preview from iTunes plays as FLAC like everything else.

const mediaPrefix = "/media/v1/"

var (
	framesURL     = regexp.MustCompile(`^/media/v1/frames/([0-9a-f-]{36})/(\d+)_(\d+)/(\d+)\.jpg$`)
	audioURL      = regexp.MustCompile(`^/media/v1/audio/([0-9a-f-]{36})/(\d+)_(\d+)\.flac$`)
	streamURL     = regexp.MustCompile(`^/media/v1/audio/([0-9a-f-]{36})/(\d{1,3})/(\d+)_(\d+)\.flac$`)
	soundtrackURL = regexp.MustCompile(`^/media/v1/soundtrack/(itunes|deezer)/(\d{1,20})/(\d+)_(\d+)\.flac$`)
	specMedia     = regexp.MustCompile(`"(/media/v1/(?:frames|audio|soundtrack)/[^"]+)"`)
)

// FramesURLs are the URLs of count frames of a media file from start, at 25 fps.
func FramesURLs(mediaFileID string, start time.Duration, count int) []string {
	urls := make([]string, count)
	for i := range urls {
		urls[i] = fmt.Sprintf("%sframes/%s/%d_%d/%d.jpg", mediaPrefix, mediaFileID, start.Milliseconds(), count, i)
	}
	return urls
}

// AudioURL is the URL of an excerpt of a media file's default audio track.
func AudioURL(mediaFileID string, start, length time.Duration) string {
	return fmt.Sprintf("%saudio/%s/%d_%d.flac", mediaPrefix, mediaFileID, start.Milliseconds(), length.Milliseconds())
}

// StreamURL is the URL of an excerpt of one audio stream of a media file, by its ffprobe
// index: a trailer's scene takes the dialogue of the original-language stream whatever the
// file's default is. A multichannel stream is downmixed with the dialogue kept up
// (dialogueFilter); the default form's stays the plain downmix, so existing beds keep their keys.
func StreamURL(mediaFileID string, stream int, start, length time.Duration) string {
	return fmt.Sprintf("%saudio/%s/%d/%d_%d.flac", mediaPrefix, mediaFileID, stream, start.Milliseconds(), length.Milliseconds())
}

// SoundtrackURL is the URL of an excerpt of a catalog track's preview: another track or
// another stretch of it is another URL, so the clips under it render again.
func SoundtrackURL(catalog, trackID string, start, length time.Duration) string {
	return fmt.Sprintf("%ssoundtrack/%s/%s/%d_%d.flac", mediaPrefix, catalog, trackID, start.Milliseconds(), length.Milliseconds())
}

// Sizes of artwork made for the screens besides the artwork feature's own (w342, w780): the
// screens that render for every break draw a backdrop 1920 wide, since a 4K original costs
// decode and texture time on every frame on a Pi, and every screen a logo within 1600 x 800
// (TMDB's run to 12000 px wide, 184 MB decoded).
const (
	backdropSize = "w1920"
	logoSize     = "1600x800"
)

var screenSizes = map[string]struct{ width, height int }{backdropSize: {1920, 0}, logoSize: {1600, 800}}

// ArtworkURL is the URL of an artwork image at a version (it changes when the image is
// replaced); size is empty for the original.
func ArtworkURL(id string, version int64, size string) string {
	u := fmt.Sprintf("%sartwork/%s?v=%d", mediaPrefix, id, version)
	if size != "" {
		u += "&size=" + size
	}
	return u
}

// mediaServer serves the render page its bundle and media, and takes its audio renders.
type mediaServer struct {
	bundle   fs.FS
	dataDir  string
	ffmpeg   string
	store    *Store
	art      *artwork.Service
	listener net.Listener
	srv      *http.Server

	mu      sync.Mutex
	uploads map[string]string // token -> file the next POST writes
}

func (m *mediaServer) start() error {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(m.bundle))
	mux.HandleFunc("GET /media/v1/artwork/{id}", m.artwork)
	mux.HandleFunc("GET /media/v1/", m.file)
	mux.HandleFunc("POST /upload/{token}", m.upload)
	m.listener, m.uploads = l, map[string]string{}
	m.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = m.srv.Serve(l) }()
	return nil
}

func (m *mediaServer) url(path string) string {
	return "http://" + m.listener.Addr().String() + path
}

func (m *mediaServer) close() {
	if m.srv != nil {
		m.srv.Close()
	}
}

func (m *mediaServer) artwork(w http.ResponseWriter, r *http.Request) {
	art, err := m.art.Store.ArtworkByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path, err := m.artworkFile(r.Context(), art, r.URL.Query().Get("size"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

// artworkFile is an artwork at the size its URL names: one of the screens' (made once and
// cached), else one of the artwork feature's, else the original.
func (m *mediaServer) artworkFile(ctx context.Context, art *artwork.Artwork, size string) (string, error) {
	if s, ok := screenSizes[size]; ok {
		return m.art.Within(ctx, art, s.width, s.height)
	}
	return m.art.Resolve(ctx, art, size)
}

// file serves extracted frames and excerpts, which prepare made before the clip loaded.
func (m *mediaServer) file(w http.ResponseWriter, r *http.Request) {
	path, ok := m.local(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if framesURL.MatchString(r.URL.Path) {
		// a footage frame is drawn once: Chromium's cache would only hold on to it
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, path)
}

// local maps a media URL to its file under DATA_DIR/couchtv.
func (m *mediaServer) local(u string) (string, bool) {
	if g := framesURL.FindStringSubmatch(u); g != nil {
		return filepath.Join(m.extractDir(g[1], g[2], g[3]), g[4]+".jpg"), true
	}
	if g := audioURL.FindStringSubmatch(u); g != nil {
		return filepath.Join(m.dataDir, "couchtv", "audio", g[1], g[2]+"_"+g[3]+".flac"), true
	}
	if g := streamURL.FindStringSubmatch(u); g != nil {
		// beside the default cuts, not in a directory of its own: the cleanup judges each
		// file's age, and touching a file does not touch its directory
		return filepath.Join(m.dataDir, "couchtv", "audio", g[1], "s"+g[2]+"_"+g[3]+"_"+g[4]+".flac"), true
	}
	if g := soundtrackURL.FindStringSubmatch(u); g != nil {
		return filepath.Join(m.dataDir, "couchtv", "audio", g[1]+"-"+g[2], g[3]+"_"+g[4]+".flac"), true
	}
	return "", false
}

func (m *mediaServer) extractDir(file, start, count string) string {
	return filepath.Join(m.dataDir, "couchtv", "frames", file, start+"_"+count)
}

// expect has the next POST to /upload/<token> written to path.
func (m *mediaServer) expect(path string) string {
	token := strconv.FormatInt(time.Now().UnixNano(), 36)
	m.mu.Lock()
	m.uploads[token] = path
	m.mu.Unlock()
	return token
}

func (m *mediaServer) upload(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	path, ok := m.uploads[r.PathValue("token")]
	delete(m.uploads, r.PathValue("token"))
	m.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Create(path)
	if err == nil {
		_, err = io.Copy(f, r.Body)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// prepare extracts the frames and excerpts a spec names that are not on disk yet, and
// touches the ones that are (the cleanup drops what has not been used for a while).
func (m *mediaServer) prepare(ctx context.Context, spec []byte) error {
	done := map[string]bool{}
	for _, u := range mediaURLs(spec) {
		if g := framesURL.FindStringSubmatch(u); g != nil {
			dir := m.extractDir(g[1], g[2], g[3])
			if done[dir] {
				continue
			}
			done[dir] = true
			start, _ := strconv.Atoi(g[2])
			count, _ := strconv.Atoi(g[3])
			if err := m.extractFrames(ctx, g[1], time.Duration(start)*time.Millisecond, count, dir); err != nil {
				return fmt.Errorf("frames %s: %w", u, err)
			}
		} else if g := audioURL.FindStringSubmatch(u); g != nil {
			path, _ := m.local(u)
			if done[path] {
				continue
			}
			done[path] = true
			start, _ := strconv.Atoi(g[2])
			length, _ := strconv.Atoi(g[3])
			if err := m.extractAudio(ctx, g[1], time.Duration(start)*time.Millisecond, time.Duration(length)*time.Millisecond, path); err != nil {
				return fmt.Errorf("audio %s: %w", u, err)
			}
		} else if g := streamURL.FindStringSubmatch(u); g != nil {
			path, _ := m.local(u)
			if done[path] {
				continue
			}
			done[path] = true
			stream, _ := strconv.Atoi(g[2])
			start, _ := strconv.Atoi(g[3])
			length, _ := strconv.Atoi(g[4])
			if err := m.extractStream(ctx, g[1], stream, time.Duration(start)*time.Millisecond, time.Duration(length)*time.Millisecond, path); err != nil {
				return fmt.Errorf("audio %s: %w", u, err)
			}
		} else if g := soundtrackURL.FindStringSubmatch(u); g != nil {
			path, _ := m.local(u)
			if done[path] {
				continue
			}
			done[path] = true
			start, _ := strconv.Atoi(g[3])
			length, _ := strconv.Atoi(g[4])
			source := filepath.Join(m.dataDir, previewFile(g[1], g[2]))
			if err := m.cutAudio(ctx, source, "0:a:0", time.Duration(start)*time.Millisecond, time.Duration(length)*time.Millisecond, path, ""); err != nil {
				return fmt.Errorf("soundtrack %s: %w", u, err)
			}
		}
	}
	return nil
}

// mediaURLs finds the extracted-media URLs in a spec's JSON.
func mediaURLs(spec []byte) []string {
	var out []string
	for _, m := range specMedia.FindAllSubmatch(spec, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// source is the file a media file is cut from, and the media file.
func (m *mediaServer) source(ctx context.Context, id string) (*media.MediaFile, string, error) {
	mf, err := media.ScanMediaFile(m.store.q.QueryRow(ctx, `SELECT `+media.MediaFileCols+` FROM media_files WHERE id = $1`, id))
	if err != nil {
		return nil, "", err
	}
	if mf.SourceDeletedAt != nil {
		return nil, "", errors.New("the source file was removed after transcoding")
	}
	var lib string
	if err := m.store.q.QueryRow(ctx, `SELECT path FROM libraries WHERE id = $1`, mf.LibraryID).Scan(&lib); err != nil {
		return nil, "", err
	}
	return mf, filepath.Join(lib, mf.Path), nil
}

// extractFrames cuts count frames at 25 fps from start: square pixels, within 1920x1080,
// 8-bit SDR (tone-mapped from HDR). A source that ends early repeats its last frame, so
// every URL the spec names exists.
func (m *mediaServer) extractFrames(ctx context.Context, fileID string, start time.Duration, count int, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".done")); err == nil {
		now := time.Now()
		return os.Chtimes(dir, now, now)
	}
	mf, path, err := m.source(ctx, fileID)
	if err != nil {
		return err
	}
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	filter := "fps=25,scale='iw*sar':ih,setsar=1,scale=1920:1080:force_original_aspect_ratio=decrease," +
		playback.SDRFilter(mf, playback.DetectFeatures(m.ffmpeg))
	cmd := exec.CommandContext(ctx, m.ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-ss", fmt.Sprintf("%.3f", start.Seconds()), "-i", path, "-map", "0:v:0", "-frames:v", strconv.Itoa(count),
		"-vf", filter, "-q:v", "3", "-start_number", "0", filepath.Join(tmp, "%d.jpg"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	last := ""
	for i := range count {
		p := filepath.Join(tmp, strconv.Itoa(i)+".jpg")
		if _, err := os.Stat(p); err == nil {
			last = p
			continue
		}
		if last == "" {
			return errors.New("the source has no frames there")
		}
		if err := os.Link(last, p); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, ".done"), nil, 0o644); err != nil {
		return err
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

// extractAudio cuts an excerpt of a media file's default audio track as stereo FLAC, which
// every Chromium decodes.
func (m *mediaServer) extractAudio(ctx context.Context, fileID string, start, length time.Duration, out string) error {
	if _, err := os.Stat(out); err == nil {
		now := time.Now()
		return os.Chtimes(out, now, now)
	}
	_, path, err := m.source(ctx, fileID)
	if err != nil {
		return err
	}
	stream := "0:a:0"
	var index int
	err = m.store.q.QueryRow(ctx, `SELECT stream_index FROM audio_streams WHERE media_file_id = $1
		ORDER BY is_default DESC, stream_index LIMIT 1`, fileID).Scan(&index)
	if err == nil {
		stream = "0:" + strconv.Itoa(index)
	}
	return m.cutAudio(ctx, path, stream, start, length, out, "")
}

// extractStream cuts an excerpt of one audio stream of a media file as stereo FLAC, the
// dialogue kept up when the stream is multichannel.
func (m *mediaServer) extractStream(ctx context.Context, fileID string, index int, start, length time.Duration, out string) error {
	if _, err := os.Stat(out); err == nil {
		now := time.Now()
		return os.Chtimes(out, now, now)
	}
	_, path, err := m.source(ctx, fileID)
	if err != nil {
		return err
	}
	var channels int
	err = m.store.q.QueryRow(ctx, `SELECT channels FROM audio_streams WHERE media_file_id = $1 AND stream_index = $2`,
		fileID, index).Scan(&channels)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("the file has no audio stream %d", index)
	}
	if err != nil {
		return err
	}
	return m.cutAudio(ctx, path, "0:"+strconv.Itoa(index), start, length, out, dialogueFilter(channels))
}

// dialogueFilter downmixes a 5.1 or 7.1 stream to stereo with the centre channel, where the
// dialogue sits, up against the sides and the surrounds (about 5 dB more than the standard
// downmix gives it); an empty filter leaves the downmix to ffmpeg. The weights add up to 1, so
// a loud scene does not clip. Channels are named by index, which the 5.1, 5.1(side) and 7.1
// layouts share (FL FR FC LFE then the surrounds); the effects channel is left out, the
// trailer has its own low end.
func dialogueFilter(channels int) string {
	switch channels {
	case 6:
		return "pan=stereo|c0=0.46*c2+0.34*c0+0.20*c4|c1=0.46*c2+0.34*c1+0.20*c5"
	case 8:
		return "pan=stereo|c0=0.44*c2+0.30*c0+0.14*c4+0.12*c6|c1=0.44*c2+0.30*c1+0.14*c5+0.12*c7"
	}
	return ""
}

// cutAudio writes a stretch of a stream of path as stereo FLAC, unless it is on disk
// already (then it is touched, for the cleanup). A filter downmixes it its own way; when
// ffmpeg refuses the filter (a layout it does not fit) the plain downmix is written.
func (m *mediaServer) cutAudio(ctx context.Context, path, stream string, start, length time.Duration, out, filter string) error {
	if _, err := os.Stat(out); err == nil {
		now := time.Now()
		return os.Chtimes(out, now, now)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	tmp := out + ".tmp.flac"
	cut := func(filter string) error {
		args := []string{"-hide_banner", "-loglevel", "error", "-y",
			"-ss", fmt.Sprintf("%.3f", start.Seconds()), "-t", fmt.Sprintf("%.3f", length.Seconds()), "-i", path, "-map", stream}
		if filter != "" {
			args = append(args, "-af", filter)
		}
		args = append(args, "-ac", "2", "-ar", "48000", "-c:a", "flac", tmp)
		if b, err := exec.CommandContext(ctx, m.ffmpeg, args...).CombinedOutput(); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
		}
		return nil
	}
	err := cut(filter)
	if err != nil && filter != "" && ctx.Err() == nil {
		err = cut("")
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, out)
}
