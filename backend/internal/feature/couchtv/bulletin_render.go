package couchtv

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/playback"
	"couchverse/internal/flags"
)

// A bulletin's clip is rendered by the sidecar shortly before the break that airs it (plan
// 6.10, D38): a job type of its own, so it never waits for a transcode or the quiet hours, and
// ahead of every other clip.
const (
	BulletinRenderJob      = "couchtv_bulletin_render"
	bulletinRenderPriority = 1
)

// renderForSlot asks the sidecar for a clip of the newest bulletin of a kind that is valid
// for the next slot of that kind, unless the bulletin has had one: only on the kind's last run
// before that slot's break goes on air less bulletinLead (the render's time), since the runs
// come more often than the slots and a later bulletin would replace this one's clip unseen.
func (b *Bulletins) renderForSlot(ctx context.Context, cfg CouchTvConfig, kind string, now time.Time) error {
	if !cfg.Bulletins.Enabled || !flags.Load(ctx, b.Settings).CouchTvEnabled {
		return nil
	}
	renderer, ok := loadRendererState(ctx, b.Settings)
	// a sidecar from before the bulletins cannot render one: its slots air their fallbacks
	if _, known := renderer.Versions[kind]; !ok || !known {
		return nil
	}
	slot, err := b.Store.nextSlot(ctx, kind, now.Add(bulletinLead))
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !nextBulletin(now, cfg.location(), bulletinEvery[kind]).After(slot.onAir.Add(-bulletinLead)) {
		return nil
	}
	row, err := b.Store.bulletinFor(ctx, kind, slot.ends)
	if errors.Is(err, db.ErrNotFound) || err == nil && row.clipID != "" {
		return nil
	}
	if err != nil {
		return err
	}
	var spec Spec
	if kind == BulletinWeather {
		var w CouchTvWeather
		if err := json.Unmarshal(row.data, &w); err != nil {
			return err
		}
		spec = weatherSpec(w)
	} else {
		var n CouchTvNews
		if err := json.Unmarshal(row.data, &n); err != nil {
			return err
		}
		spec = newsSpec(n, cfg.location())
	}
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("the %s bulletin of %s: %w", kind, row.fetchedAt, err)
	}
	key, err := clipKey(renderer.Versions[spec.Template], spec)
	if err != nil {
		return err
	}
	id, state, err := b.Store.RequestClip(ctx, kind, key, spec, bulletinFrames[kind], slot.starts)
	if err != nil {
		return err
	}
	if err := b.Store.setBulletinClip(ctx, row.id, id); err != nil || state == ClipReady {
		return err
	}
	if state == ClipFailed {
		if err := b.Store.RetryClip(ctx, id); err != nil {
			return err
		}
	}
	slog.Info("couchtv: rendering a bulletin", "kind", kind, "fetchedAt", row.fetchedAt, "slot", slot.starts)
	_, err = b.Jobs.EnqueueJobOnce(ctx, BulletinRenderJob, RenderPayload{ClipID: id}, jobs.EnqueueOpts{Priority: bulletinRenderPriority})
	return err
}

// upcomingSlot is a slot of a break composed ahead: when the break goes on air, and when the
// slot starts and ends if the break airs as composed.
type upcomingSlot struct {
	onAir, starts, ends time.Time
}

// nextSlot is the first slot of a kind in a break that goes on air after after.
func (s *Store) nextSlot(ctx context.Context, kind string, after time.Time) (upcomingSlot, error) {
	var u upcomingSlot
	var at, frames int
	err := s.q.QueryRow(ctx, `
		SELECT a.starts_at, (bs->>'at')::int, (bs->>'frames')::int
		FROM couchtv_breaks b JOIN couchtv_airings a ON a.id = b.airing_id
		CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(b.recipe->'slots') = 'array'
			THEN b.recipe->'slots' ELSE '[]' END) bs
		WHERE b.state IN ('composed', 'rendering', 'ready') AND a.state <> 'replaced' AND a.feed IS NULL
		  AND a.starts_at > $2 AND bs->>'kind' = $1
		ORDER BY a.starts_at LIMIT 1`, kind, after).Scan(&u.onAir, &at, &frames)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, db.ErrNotFound
	}
	u.starts = u.onAir.Add(time.Duration(at) * time.Second / FPS)
	u.ends = u.starts.Add(time.Duration(frames) * time.Second / FPS)
	return u, err
}

// storedBulletin is a couchtv_bulletins row.
type storedBulletin struct {
	id, clipID string
	fetchedAt  time.Time
	data       []byte
}

// bulletinFor is the newest bulletin of a kind still valid at until.
func (s *Store) bulletinFor(ctx context.Context, kind string, until time.Time) (storedBulletin, error) {
	var b storedBulletin
	err := s.q.QueryRow(ctx, `SELECT id, coalesce(clip_id::text, ''), fetched_at, data FROM couchtv_bulletins
		WHERE kind = $1 AND valid_until >= $2 ORDER BY fetched_at DESC LIMIT 1`, kind, until).
		Scan(&b.id, &b.clipID, &b.fetchedAt, &b.data)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, db.ErrNotFound
	}
	return b, err
}

func (s *Store) setBulletinClip(ctx context.Context, id, clipID string) error {
	_, err := s.q.Exec(ctx, `UPDATE couchtv_bulletins SET clip_id = $2 WHERE id = $1`, id, clipID)
	return err
}

// ReadyBulletin is the rendered clip, frames long, of the newest bulletin of a kind still valid
// at until (db.ErrNotFound when there is none): what a slot ending then airs.
func (s *Store) ReadyBulletin(ctx context.Context, kind string, until time.Time, frames int) (*Clip, error) {
	var id string
	err := s.q.QueryRow(ctx, `SELECT c.id FROM couchtv_bulletins b JOIN couchtv_clips c ON c.id = b.clip_id
		WHERE b.kind = $1 AND c.state = 'ready' AND b.valid_until >= $2 AND c.duration_ms = $3
		ORDER BY b.fetched_at DESC LIMIT 1`, kind, until, frames*1000/FPS).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.Clip(ctx, id)
}

// slotLoudnesses are the loudnesses a bulletin's clip has a rendition of: every whole LU a
// break is mixed at, so it airs at the loudness of whichever slot it fills (slotLoudness).
func slotLoudnesses() []float64 {
	var out []float64
	for l := quietestLUFS; l <= loudestLUFS; l++ {
		out = append(out, l)
	}
	return out
}

// slotAudio is the directory of a bulletin clip's rendition at a slot's loudness.
func slotAudio(lufs float64) string { return fmt.Sprintf("audio-l%d-aac", int(math.Round(-lufs))) }

// Fades a bulletin's renditions open and close with, so whatever its template ends on, its
// slot's edges are clean.
const (
	slotFadeIn  = 0.005
	slotFadeOut = 0.02
)

// encodeSlotSounds encodes a bulletin clip's sound (raw, the page's render) once at each slot
// loudness into the clip's package in work, faded in and out at its edges.
func (r *Renderer) encodeSlotSounds(ctx context.Context, raw, work string, frames int) error {
	data, err := os.ReadFile(raw)
	if err != nil {
		return err
	}
	samples := make([]float32, len(data)/4)
	for i := range samples {
		samples[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4*i:]))
	}
	m := newMeter(sampleRate)
	m.add(samples)
	levels := slotLoudnesses()
	var graph strings.Builder
	fmt.Fprintf(&graph, "[0:a]afade=t=in:d=%g,afade=t=out:st=%.3f:d=%g,asplit=%d", slotFadeIn, frameSeconds(frames)-slotFadeOut,
		slotFadeOut, len(levels))
	for i := range levels {
		fmt.Fprintf(&graph, "[s%d]", i)
	}
	for i, l := range levels {
		fmt.Fprintf(&graph, ";[s%d]volume=%.4fdB[o%d]", i, m.normalization(l, truePeakCeiling).GainDB, i)
	}
	args := append(playback.InputArgs(raw, "-f", "f32le", "-ar", "48000", "-ac", "2"), "-filter_complex", graph.String())
	for i, l := range levels {
		dir := filepath.Join(work, slotAudio(l))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		args = append(args, playback.RenditionArgs(dir, append([]string{"-map", fmt.Sprintf("[o%d]", i)}, audioCodec...), false)...)
	}
	if out, err := exec.CommandContext(ctx, r.FFmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("encode the slot sounds: %v: %s", err, strings.TrimSpace(string(out)))
	}
	for i, l := range levels {
		if _, err := playback.DescribeRendition(filepath.Join(work, slotAudio(l)),
			playback.RenditionInfo{StreamIndex: 2 + i, Language: "en", Name: fmt.Sprintf("%g LUFS", l)}); err != nil {
			return err
		}
	}
	return nil
}
