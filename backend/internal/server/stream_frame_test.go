package server_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/config"
	"couchverse/internal/grant"
	"couchverse/internal/settings"
)

// A still is cut from the second asked for when the caller says exact, for a tool that has to
// show a particular scene; otherwise it is the start of its 5-second bucket, as the seek bar
// reuses stills.
func TestStreamFrameExact(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()

	// the seeded movie's file, a clip of 12 s whose picture is another flat colour each second
	dir := t.TempDir()
	if _, err := env.pool.Exec(ctx, `UPDATE libraries SET path = $1 WHERE id = (SELECT library_id FROM media_files WHERE id = $2)`, dir, movieFileID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE media_files SET path = 'colours.mp4' WHERE id = $1`, movieFileID); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
		"nullsrc=s=320x180:r=10:d=12,format=rgb24,geq=r='mod(floor(T)*83,256)':g='mod(floor(T)*151,256)':b='mod(floor(T)*37,256)'",
		"-c:v", "libx264", "-g", "10", "-pix_fmt", "yuv420p", filepath.Join(dir, "colours.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}

	key, err := grant.LoadSecret(ctx, settings.NewStore(env.pool))
	if err != nil {
		t.Fatal(err)
	}
	token := grant.NewSigner(key).Issue(grant.Grant{Scope: grant.Media, Subject: 2, Resource: uuid.MustParse(movieFileID)}, time.Hour)
	frame := func(query string) []byte {
		t.Helper()
		status, body := env.get(t, "/api/v1/media/"+token+"/frame?"+query)
		if status != http.StatusOK {
			t.Fatalf("frame %s: %d %s", query, status, body)
		}
		return body
	}
	cached := func(second int) bool {
		_, err := os.Stat(filepath.Join(env.cfg.DataDir, "cache", "frames", movieFileID, fmt.Sprintf("%d.jpg", second)))
		return err == nil
	}

	// bucketed, seconds 6 and 7 are the still of second 5
	if !bytes.Equal(frame("t=6"), frame("t=7")) || !cached(5) || cached(6) || cached(7) {
		t.Error("stills of seconds 6 and 7 are not the 5-second bucket's")
	}
	// exact, each second is its own still
	six, seven := frame("t=6&exact=true"), frame("t=7&exact=true")
	if bytes.Equal(six, seven) || bytes.Equal(six, frame("t=5")) || !cached(6) || !cached(7) {
		t.Error("exact stills of seconds 6 and 7 are the same picture")
	}
	if !bytes.Equal(frame("t=7&exact=true"), seven) {
		t.Error("an exact still changes between requests")
	}
}
