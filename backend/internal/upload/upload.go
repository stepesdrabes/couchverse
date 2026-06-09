// Package upload implements resumable chunked uploads: create a session,
// append sequential chunks (resume from the server-reported offset after a
// disconnect), then complete to move the file into a managed library.
package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"couchverse/internal/media"
	"couchverse/internal/store"
)

const MaxChunkSize = 64 << 20

type Manager struct {
	Store   *store.Store
	DataDir string
}

func (m *Manager) tempDir() string {
	return filepath.Join(m.DataDir, "cache", "uploads")
}

var unsafeChars = regexp.MustCompile(`[\x00-\x1f/\\:*?"<>|]`)

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = unsafeChars.ReplaceAllString(name, " ")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || name == "." || name == ".." {
		name = "upload.bin"
	}
	return name
}

func (m *Manager) Create(ctx context.Context, userID int64, filename string, size int64) (*store.UploadSession, error) {
	if err := os.MkdirAll(m.tempDir(), 0o755); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	tempPath := filepath.Join(m.tempDir(), id+".part")

	f, err := os.Create(tempPath)
	if err != nil {
		return nil, err
	}
	f.Close()

	return m.Store.CreateUploadSession(ctx, id, userID, sanitizeFilename(filename), size, tempPath)
}

// ErrOffsetMismatch carries the server-side offset so clients can resync.
type ErrOffsetMismatch struct {
	Offset int64
}

func (e *ErrOffsetMismatch) Error() string {
	return fmt.Sprintf("offset mismatch, server is at %d", e.Offset)
}

func (m *Manager) Append(ctx context.Context, id string, offset int64, body io.Reader) (int64, error) {
	session, err := m.Store.UploadSession(ctx, id)
	if err != nil {
		return 0, err
	}
	if session.Status != "active" {
		return 0, fmt.Errorf("upload session is %s", session.Status)
	}
	if offset != session.ReceivedBytes {
		return 0, &ErrOffsetMismatch{Offset: session.ReceivedBytes}
	}

	f, err := os.OpenFile(session.TempPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(body, MaxChunkSize+1))
	if err != nil {
		// reconcile the on-disk size so the client can resume from truth
		if info, statErr := f.Stat(); statErr == nil {
			_ = m.Store.SetUploadReceived(context.WithoutCancel(ctx), id, info.Size())
		}
		return 0, err
	}
	if written > MaxChunkSize {
		return 0, fmt.Errorf("chunk exceeds %d bytes", MaxChunkSize)
	}

	newOffset := session.ReceivedBytes + written
	if newOffset > session.DeclaredSize {
		return 0, fmt.Errorf("received more bytes than declared size")
	}
	if err := m.Store.SetUploadReceived(ctx, id, newOffset); err != nil {
		return 0, err
	}
	return newOffset, nil
}

type Assign struct {
	LibraryKind string `json:"libraryKind"` // movies | series | music
	TitleID     *int64 `json:"titleId"`
	EpisodeID   *int64 `json:"episodeId"`
}

// Complete moves the finished upload into its managed library (same
// filesystem → rename) and queues a probe.
func (m *Manager) Complete(ctx context.Context, id string, assign Assign) (int64, error) {
	session, err := m.Store.UploadSession(ctx, id)
	if err != nil {
		return 0, err
	}
	if session.Status != "active" {
		return 0, fmt.Errorf("upload session is %s", session.Status)
	}
	if session.ReceivedBytes != session.DeclaredSize {
		return 0, fmt.Errorf("upload incomplete: %d of %d bytes", session.ReceivedBytes, session.DeclaredSize)
	}

	lib, err := m.Store.ManagedLibraryByKind(ctx, assign.LibraryKind)
	if err != nil {
		return 0, fmt.Errorf("no managed %q library", assign.LibraryKind)
	}

	relPath, err := m.destinationPath(ctx, lib, session.Filename, assign)
	if err != nil {
		return 0, err
	}
	absPath := filepath.Join(lib.Path, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return 0, err
	}
	if err := os.Rename(session.TempPath, absPath); err != nil {
		return 0, err
	}

	mediaFileID, err := m.Store.CreateAssignedMediaFile(ctx, lib.ID, relPath,
		session.ReceivedBytes, assign.TitleID, assign.EpisodeID)
	if err != nil {
		return 0, err
	}
	if _, err := m.Store.EnqueueJobOnce(ctx, "probe",
		media.ProbePayload{MediaFileID: mediaFileID}, store.EnqueueOpts{Priority: 5}); err != nil {
		return 0, err
	}
	if err := m.Store.SetUploadStatus(ctx, id, "complete"); err != nil {
		return 0, err
	}
	return mediaFileID, nil
}

// destinationPath picks a tidy library-relative location for the upload.
func (m *Manager) destinationPath(ctx context.Context, lib *store.Library, filename string, assign Assign) (string, error) {
	switch lib.Kind {
	case "movies":
		folder := strings.TrimSuffix(filename, filepath.Ext(filename))
		if assign.TitleID != nil {
			if t, err := m.Store.TitleByID(ctx, *assign.TitleID); err == nil {
				folder = t.Name
				if t.Year != nil {
					folder = fmt.Sprintf("%s (%d)", t.Name, *t.Year)
				}
			}
		} else if parsed := media.ParseVideoPath(filename); parsed.Name != "" {
			folder = parsed.Name
			if parsed.Year != nil {
				folder = fmt.Sprintf("%s (%d)", parsed.Name, *parsed.Year)
			}
		}
		return filepath.Join(sanitizeFilename(folder), filename), nil

	case "series":
		if assign.EpisodeID != nil {
			if ref, err := m.Store.EpisodeRef(ctx, *assign.EpisodeID); err == nil {
				return filepath.Join(
					sanitizeFilename(ref.TitleName),
					fmt.Sprintf("Season %02d", ref.SeasonNumber),
					filename), nil
			}
		}
		if parsed := media.ParseVideoPath(filename); parsed.IsEpisode {
			return filepath.Join(
				sanitizeFilename(parsed.ShowName),
				fmt.Sprintf("Season %02d", parsed.Season),
				filename), nil
		}
		return filename, nil

	default: // music — tags decide the catalog placement, keep files flat
		return filename, nil
	}
}

// Abort marks the session aborted and removes the temp file.
func (m *Manager) Abort(ctx context.Context, id string) error {
	session, err := m.Store.UploadSession(ctx, id)
	if err != nil {
		return err
	}
	os.Remove(session.TempPath)
	if err := m.Store.SetUploadStatus(ctx, id, "aborted"); err != nil {
		return err
	}
	return m.Store.DeleteUploadSession(ctx, id)
}

// Reap removes expired/aborted sessions and their temp files (cleanup job).
func (m *Manager) Reap(ctx context.Context) (int, error) {
	sessions, err := m.Store.ExpiredUploadSessions(ctx)
	if err != nil {
		return 0, err
	}
	for _, s := range sessions {
		os.Remove(s.TempPath)
		if err := m.Store.DeleteUploadSession(ctx, s.ID); err != nil {
			return 0, err
		}
	}
	return len(sessions), nil
}
