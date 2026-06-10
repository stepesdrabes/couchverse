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

	"couchverse/internal/feature/library"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// JIT ("instant play") sessions transcode on demand: ffmpeg starts at the
// requested position and the playlist pretends the whole file already exists.
// Far seeks restart ffmpeg at the target segment; idle sessions are reaped.

const (
	segmentSeconds  = 4.0
	sessionIdleTTL  = 2 * time.Minute
	segmentWaitMax  = 25 * time.Second
	lookaheadWindow = 15 // segments past the encode frontier before a restart
)

type Session struct {
	ID          string
	MediaFileID string

	dir      string
	input    string
	hasAudio bool
	duration float64
	encoder  string
	preset   string
	rend     media.Rendition

	mu         sync.Mutex
	cancel     context.CancelFunc
	startSeg   int
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

// Create starts a JIT session for a media file at startAt seconds.
func (m *SessionManager) Create(ctx context.Context, appCtx context.Context, mediaFileID string, startAt float64) (*Session, error) {
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

	session := &Session{
		ID:          id,
		MediaFileID: mf.ID,
		dir:         filepath.Join(m.DataDir, "cache", "sessions", id),
		input:       filepath.Join(lib.Path, mf.Path),
		hasAudio:    mf.AudioCodec != "",
		duration:    mf.DurationSeconds,
		encoder:     PickEncoder(m.FFmpegPath, settings.HWAccel),
		preset:      settings.Preset,
		rend:        rendition,
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

	slog.Info("jit session started", "id", id, "mediaFile", mf.ID, "encoder", session.encoder, "startSeg", startSeg)
	return session, nil
}

func (m *SessionManager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		return nil
	}
	return m.sessions[id]
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

func (s *Session) startFFmpeg(appCtx context.Context, ffmpegPath string, fromSeg int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(appCtx)
	s.cancel = cancel
	s.startSeg = fromSeg

	spec := BuildSpec{
		Input:       s.input,
		OutDir:      s.dir,
		Mode:        "transcode",
		Rendition:   s.rend,
		Encoder:     s.encoder,
		Preset:      s.preset,
		HasAudio:    s.hasAudio,
		StartAt:     float64(fromSeg) * segmentSeconds,
		JIT:         true,
		StartNumber: fromSeg,
	}

	go func() {
		if err := Run(ctx, ffmpegPath, spec, 0, nil); err != nil && ctx.Err() == nil {
			slog.Warn("jit ffmpeg exited", "session", s.ID, "err", err)
		}
	}()
	return nil
}

// Playlist generates the full VOD playlist covering the entire duration.
func (s *Session) Playlist() string {
	totalSegments := int(s.duration / segmentSeconds)
	remainder := s.duration - float64(totalSegments)*segmentSeconds

	var b []byte
	b = append(b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n"...)
	for i := 0; i < totalSegments; i++ {
		b = append(b, fmt.Sprintf("#EXTINF:%.3f,\nseg_%05d.ts\n", segmentSeconds, i)...)
	}
	if remainder > 0.1 {
		b = append(b, fmt.Sprintf("#EXTINF:%.3f,\nseg_%05d.ts\n", remainder, totalSegments)...)
	}
	b = append(b, "#EXT-X-ENDLIST\n"...)
	return string(b)
}

var segNameRe = regexp.MustCompile(`^seg_(\d{5})\.ts$`)

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

	deadline := time.Now().Add(segmentWaitMax)
	for {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

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
			if err := s.startFFmpeg(context.WithoutCancel(ctx), m.FFmpegPath, segIdx); err != nil {
				return "", err
			}
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("segment %d not ready in time", segIdx)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
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
