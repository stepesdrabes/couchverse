package server_test

import (
	"context"
	"testing"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/media"
)

// TestPrepareKeepsFinishedTrickplay prepares a rung of a file whose trick-play rendition
// is encoded: it stays ready (in the presentations) and is not queued again.
func TestPrepareKeepsFinishedTrickplay(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	files, jb := library.NewStore(env.pool), jobs.NewStore(env.pool)
	mf, err := files.MediaFileByID(ctx, movieFileID)
	if err != nil {
		t.Fatal(err)
	}
	v, err := files.UpsertVariant(ctx, mf.ID, media.VariantTrickplay, 180, 0, 0, "transcode")
	if err != nil {
		t.Fatal(err)
	}
	if err := files.SetVariantStatus(ctx, v.ID, "ready", "hls/"+mf.ID+"/trickplay/index.m3u8", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := library.Prepare(ctx, files, jb, mf, false, []string{"480p"}, library.PrepareOpts{}); err != nil {
		t.Fatal(err)
	}
	var status string
	var queued int
	if err := env.pool.QueryRow(ctx, `SELECT status, (SELECT count(*) FROM jobs WHERE type = 'transcode_hls'
		AND payload->>'variant' = 'trickplay') FROM transcode_variants WHERE media_file_id = $1 AND name = 'trickplay'`,
		mf.ID).Scan(&status, &queued); err != nil {
		t.Fatal(err)
	}
	if status != "ready" || queued != 0 {
		t.Errorf("trick-play %s with %d encodes queued, want it ready and none", status, queued)
	}
}
