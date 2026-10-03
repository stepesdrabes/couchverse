package playback

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/feature/library"
	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/media/mp4"
	"couchverse/internal/settings"
)

// JIT ("instant play") sessions encode on demand: ffmpeg starts at the
// requested position and the playlist pretends the whole file already exists.
// Far seeks restart ffmpeg at the target segment; idle sessions are reaped.
// Every run writes fMP4 on the shared timeline (see package.go), so segments of
// different runs line up and one init section serves them all.

const (
	sessionIdleTTL  = 2 * time.Minute
	segmentWaitMax  = 25 * time.Second
	lookaheadWindow = 10 // segments past the encode frontier before a restart
)

// Viewer binding of a session: the grant it was opened with.
type sessionOwner struct {
	Subject int64
	Couch   uuid.UUID
}

type Session struct {
	ID          string
	MediaFileID string
	owner       sessionOwner

	dir      string
	input    string
	duration float64
	mf       media.MediaFile
	plan     JITPlan
	encoder  string
	preset   string
	rend     media.Rendition
	features Features

	mu         sync.Mutex
	cancel     context.CancelFunc
	startSeg   int
	run        int
	lastAccess time.Time
}

type SessionManager struct {
	Files       *library.Store
	Settings    *settings.Store
	DataDir     string
	FFmpegPath  string
	MaxSessions int

	mu       sync.Mutex
	sessions map[string]*Session
	reapOnce sync.Once
}

func (m *SessionManager) ensureInit(ctx context.Context) {
	m.reapOnce.Do(func() {
		m.sessions = map[string]*Session{}
		go m.reapLoop(ctx)
	})
}

// Create starts a JIT session for a media file at startAt seconds, bound to owner.
func (m *SessionManager) Create(ctx context.Context, appCtx context.Context, mediaFileID string, owner sessionOwner, plan JITPlan, startAt float64) (*Session, error) {
	m.ensureInit(appCtx)

	mf, err := m.Files.MediaFileByID(ctx, mediaFileID)
	if err != nil {
		return nil, err
	}
	if mf.VideoCodec == "" {
		return nil, fmt.Errorf("not a video file")
	}
	lib, err := m.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return nil, err
	}
	tracks, err := m.Files.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	plan = validPlan(plan, tracks)

	m.mu.Lock()
	if len(m.sessions) >= m.MaxSessions {
		// reuse: drop the oldest idle session
		var oldest *Session
		for _, s := range m.sessions {
			if oldest == nil || s.lastAccess.Before(oldest.lastAccess) {
				oldest = s
			}
		}
		if oldest != nil && time.Since(oldest.lastAccess) > 30*time.Second {
			m.removeLocked(oldest)
		} else {
			m.mu.Unlock()
			return nil, fmt.Errorf("too many active playback sessions, try again shortly")
		}
	}
	m.mu.Unlock()

	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(raw)

	settings := media.LoadTranscodeSettings(ctx, m.Settings)
	rendition := media.Renditions["720p"]
	if mf.Height > 0 && mf.Height < 600 {
		rendition = media.Renditions["480p"]
	}
	rendition = rendition.CappedAt(mf.Bitrate)

	session := &Session{
		ID:          id,
		MediaFileID: mf.ID,
		owner:       owner,
		dir:         filepath.Join(m.DataDir, "cache", "sessions", id),
		input:       filepath.Join(lib.Path, mf.Path),
		duration:    mf.DurationSeconds,
		mf:          *mf,
		plan:        plan,
		encoder:     PickEncoder(m.FFmpegPath, settings.HWAccel),
		preset:      settings.Preset,
		rend:        rendition,
		features:    DetectFeatures(m.FFmpegPath),
		lastAccess:  time.Now(),
	}
	if err := os.MkdirAll(session.dir, 0o755); err != nil {
		return nil, err
	}

	startSeg := int(startAt / segmentSeconds)
	if err := session.startFFmpeg(appCtx, m.FFmpegPath, startSeg); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	slog.Info("jit session started", "id", id, "mediaFile", mf.ID, "video", plan.Video, "audio", plan.Audio, "encoder", session.encoder, "startSeg", startSeg)
	return session, nil
}

// validPlan keeps a client's plan to what the file has. Copying the video
// needs segment boundaries known in advance, which only a fixed keyframe grid
// gives, so instant play always encodes the video.
func validPlan(plan JITPlan, tracks []media.AudioStream) JITPlan {
	plan.Video = "transcode"
	var track *media.AudioStream
	for i := range tracks {
		if tracks[i].Index == plan.AudioStream || track == nil && plan.AudioStream < 0 && tracks[i].Default {
			track = &tracks[i]
		}
	}
	if track == nil && len(tracks) > 0 {
		track = &tracks[0]
	}
	if track == nil {
		plan.AudioStream = -1
		return plan
	}
	plan.AudioStream = track.Index
	switch {
	case plan.Audio == "copy" && (track.Codec == "aac" || track.Codec == "ac3" || track.Codec == "eac3"):
	case plan.Audio == "eac3" && track.Channels > 2:
	default:
		plan.Audio = "aac"
	}
	return plan
}

func (m *SessionManager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		return nil
	}
	return m.sessions[id]
}

// ActiveCount returns the number of live JIT (instant-play) sessions, for the
// admin dashboard's live stats.
func (m *SessionManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func (m *SessionManager) Touch(id string) bool {
	s := m.Get(id)
	if s == nil {
		return false
	}
	s.mu.Lock()
	s.lastAccess = time.Now()
	s.mu.Unlock()
	return true
}

func (m *SessionManager) removeLocked(s *Session) {
	s.stop()
	os.RemoveAll(s.dir)
	delete(m.sessions, s.ID)
	slog.Info("jit session removed", "id", s.ID)
}

func (m *SessionManager) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.StopAll()
			return
		case <-ticker.C:
			m.mu.Lock()
			for _, s := range m.sessions {
				s.mu.Lock()
				idle := time.Since(s.lastAccess)
				s.mu.Unlock()
				if idle > sessionIdleTTL {
					m.removeLocked(s)
				}
			}
			m.mu.Unlock()
		}
	}
}

// Stop ends one session and removes its scratch files.
func (m *SessionManager) Stop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[id]; s != nil {
		m.removeLocked(s)
	}
}

func (m *SessionManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		m.removeLocked(s)
	}
}

func (s *Session) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// args is the ffmpeg command for one run starting at segment fromSeg.
func (s *Session) args(fromSeg int, run int) []string {
	startAt := float64(fromSeg) * segmentSeconds
	codec := rungVideoArgs(&s.mf, s.rend, s.encoder, s.preset, s.features, startAt)
	if s.plan.AudioStream >= 0 {
		a := audioRendition{track: media.AudioStream{Index: s.plan.AudioStream}, codec: s.plan.Audio, copy: s.plan.Audio == "copy"}
		codec = append(codec, a.args()...)
	}
	args := inputArgs(s.input, startAt)
	args = append(args, codec...)
	return append(args,
		"-output_ts_offset", timelineOffset,
		"-f", "hls",
		"-hls_time", strconv.Itoa(int(segmentSeconds)),
		"-hls_segment_type", "fmp4",
		"-hls_segment_options", "movflags=+frag_discont+negative_cts_offsets",
		// every run writes its own init; the playlist serves the first one
		"-hls_fmp4_init_filename", fmt.Sprintf("init_%d.mp4", run),
		"-start_number", strconv.Itoa(fromSeg),
		// temp_file makes finished segments appear atomically
		"-hls_flags", "temp_file+independent_segments",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(s.dir, "seg_%05d.m4s"),
		filepath.Join(s.dir, fmt.Sprintf("run_%d.m3u8", run)),
	)
}

func (s *Session) startFFmpeg(appCtx context.Context, ffmpegPath string, fromSeg int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(appCtx)
	s.cancel = cancel
	s.startSeg = fromSeg
	args := s.args(fromSeg, s.run)
	s.run++

	go func() {
		if err := Run(ctx, ffmpegPath, args, false, 0, nil); err != nil && ctx.Err() == nil {
			slog.Warn("jit ffmpeg exited", "session", s.ID, "err", err)
		}
	}()
	return nil
}

// Playlist is the VOD media playlist covering the whole duration.
func (s *Session) Playlist() string {
	m := hls.Media{Version: 7, TargetDuration: int(segmentSeconds), PlaylistType: "VOD", IndependentSegments: true, EndList: true}
	init := &hls.Map{URI: "init.mp4"}
	total := int(s.duration / segmentSeconds)
	for i := 0; i < total; i++ {
		m.Segments = append(m.Segments, hls.Segment{Duration: segmentSeconds, URI: segmentName(i), Map: init})
	}
	if remainder := s.duration - float64(total)*segmentSeconds; remainder > 0.1 {
		m.Segments = append(m.Segments, hls.Segment{Duration: remainder, URI: segmentName(total), Map: init})
	}
	return m.String()
}

func segmentName(i int) string { return fmt.Sprintf("seg_%05d.m4s", i) }

var segNameRe = regexp.MustCompile(`^seg_(\d{5})\.m4s$`)

// InitPath waits for the first run's init section. The muxer creates the file
// before it writes the moov, so it counts once it parses.
func (m *SessionManager) InitPath(ctx context.Context, s *Session) (string, error) {
	path := filepath.Join(s.dir, "init_0.mp4")
	m.Touch(s.ID)
	return path, waitFor(ctx, path, func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		_, err = mp4.ParseInit(data)
		return err == nil
	}, nil)
}

// SegmentPath blocks until the requested segment exists, restarting ffmpeg
// when the request is outside the current encode window (a far seek).
func (m *SessionManager) SegmentPath(ctx context.Context, s *Session, name string) (string, error) {
	match := segNameRe.FindStringSubmatch(name)
	if match == nil {
		return "", fmt.Errorf("invalid segment name")
	}
	segIdx, _ := strconv.Atoi(match[1])
	path := filepath.Join(s.dir, name)

	m.Touch(s.ID)
	return path, waitFor(ctx, path, exists(path), func() error {
		s.mu.Lock()
		startSeg := s.startSeg
		s.mu.Unlock()
		// right after a restart the frontier lags behind the new window, and
		// stale segments from earlier windows can make it overshoot - anchor
		// the lookahead at whichever is further along
		frontier := max(s.encodeFrontier(), startSeg)

		// behind the window or far ahead of the frontier → restart at target
		if segIdx < startSeg || segIdx > frontier+lookaheadWindow {
			slog.Info("jit far seek", "session", s.ID, "segment", segIdx, "frontier", frontier)
			return s.startFFmpeg(context.WithoutCancel(ctx), m.FFmpegPath, segIdx)
		}
		return nil
	})
}

func exists(path string) func() bool {
	return func() bool {
		_, err := os.Stat(path)
		return err == nil
	}
}

// waitFor polls until the file at path is ready, calling check on every miss.
func waitFor(ctx context.Context, path string, ready func() bool, check func() error) error {
	deadline := time.Now().Add(segmentWaitMax)
	for {
		if ready() {
			return nil
		}
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s not ready in time", filepath.Base(path))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// encodeFrontier reports the highest segment index written so far.
func (s *Session) encodeFrontier() int {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return -1
	}
	frontier := -1
	for _, entry := range entries {
		if match := segNameRe.FindStringSubmatch(entry.Name()); match != nil {
			if n, _ := strconv.Atoi(match[1]); n > frontier {
				frontier = n
			}
		}
	}
	return frontier
}
