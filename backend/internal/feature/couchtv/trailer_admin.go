package couchtv

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// The admin's word on a title's trailer (D43, plan 6.4). The picker's scenes are the
// composer's own; the admin sees them as cuts, keeps some of them or none, and hands the
// choice back to the picker.

// ErrTrailerSceneGone is the code of an admin pick that names a scene the trailer no longer
// has: the picker chose again since the admin looked.
const ErrTrailerSceneGone = "trailer_scene_gone"

// CouchTvTrailerCut is a scene of a title's trailer as the admin sees it.
type CouchTvTrailerCut struct {
	MediaFileID string   `json:"mediaFileId" format:"uuid" doc:"The file the picture is cut from. With startMs it names the scene when an admin keeps some."`
	StartMs     int      `json:"startMs" doc:"Where the scene starts in that file."`
	DurationMs  int      `json:"durationMs" doc:"Whole frames at 25 fps, 3 to 8 seconds."`
	Lines       []string `json:"lines" doc:"What is said in the scene, in order, as the subtitles have it; empty for a scene found by listening, which has no text."`
	Source      string   `json:"source" enum:"subtitles,speech" doc:"How its lines were found: from the subtitles, or by listening to the sound."`
}

// CouchTvTrailerSceneRef names a scene of a title's trailer by what it is cut from, which
// holds while the picker has not chosen again; a position in the list would shift.
type CouchTvTrailerSceneRef struct {
	MediaFileID string `json:"mediaFileId" format:"uuid"`
	StartMs     int    `json:"startMs" minimum:"0"`
}

// CouchTvTrailerPick is an admin's word on a title's trailer.
type CouchTvTrailerPick struct {
	Keep []CouchTvTrailerSceneRef `json:"keep" maxItems:"12" doc:"The scenes the trailer keeps, each one of its current scenes; they play in the order the trailer already has. None means no trailer for the title, and so do fewer than trailerMinScenes."`
}

// trailerCuts are a trailer's scenes as the admin sees them.
func trailerCuts(scenes []CouchTvTrailerScene) []CouchTvTrailerCut {
	out := make([]CouchTvTrailerCut, len(scenes))
	for i, s := range scenes {
		out[i] = CouchTvTrailerCut{MediaFileID: s.MediaFileID, StartMs: s.StartMs, DurationMs: s.DurationMs, Source: s.Source, Lines: []string{}}
		for _, l := range s.Lines {
			if l.Text != "" {
				out[i].Lines = append(out[i].Lines, l.Text)
			}
		}
	}
	return out
}

// keepScenes are the scenes of a trailer an admin's pick keeps, in the order the trailer
// plays them. A pick that names a scene the trailer does not have is an error: the admin
// chose from a trailer that has been picked again. Naming a scene twice keeps it once.
func keepScenes(current []CouchTvTrailerScene, keep []CouchTvTrailerSceneRef) ([]CouchTvTrailerScene, error) {
	named := func(s CouchTvTrailerScene, r CouchTvTrailerSceneRef) bool {
		return s.MediaFileID == r.MediaFileID && s.StartMs == r.StartMs
	}
	for _, r := range keep {
		if !slices.ContainsFunc(current, func(s CouchTvTrailerScene) bool { return named(s, r) }) {
			return nil, fmt.Errorf("the trailer has no scene of file %s at %d ms", r.MediaFileID, r.StartMs)
		}
	}
	kept := []CouchTvTrailerScene{}
	for _, s := range current {
		if slices.ContainsFunc(keep, func(r CouchTvTrailerSceneRef) bool { return named(s, r) }) {
			kept = append(kept, s)
		}
	}
	return kept, nil
}

// sameScenes reports whether two trailers are made of the same scenes in the same order.
func sameScenes(a, b []CouchTvTrailerScene) bool {
	return slices.EqualFunc(a, b, func(x, y CouchTvTrailerScene) bool {
		return x.MediaFileID == y.MediaFileID && x.StartMs == y.StartMs && x.DurationMs == y.DurationMs
	})
}

// saveTrailer stores the admin's scenes for a title's trailer, or hands the trailer back
// (nil scenes): the picker then chooses again, and until it has the scenes the admin kept
// stay, as an earlier automatic pick would.
func saveTrailer(ctx context.Context, q querier, titleID string, scenes []CouchTvTrailerScene) error {
	if scenes == nil {
		_, err := q.Exec(ctx, `UPDATE couchtv_titles SET trailer_source = 'auto', trailer_version = 0, updated_at = now()
			WHERE title_id = $1 AND trailer_source = 'admin'`, titleID)
		return err
	}
	body, err := json.Marshal(scenes)
	if err != nil {
		return err
	}
	// stamped with the picker's version, so the backfill does not queue the title again
	_, err = q.Exec(ctx, `UPDATE couchtv_titles SET trailer = $2, trailer_source = 'admin', trailer_version = $3, updated_at = now()
		WHERE title_id = $1`, titleID, body, TrailerVersion)
	return err
}

// ResetTrailerBreaks drops the compositions of the breaks that have not gone on air and air a
// title's trailer, so they are composed again from the trailer it has now: a scene an admin
// dropped, or the whole trailer, must not air from a break composed before. It reports how
// many it dropped.
func (s *Store) ResetTrailerBreaks(ctx context.Context, titleID string, now time.Time) (int, error) {
	tag, err := s.q.Exec(ctx, `
		DELETE FROM couchtv_breaks b USING couchtv_airings a
		WHERE b.airing_id = a.id AND a.kind = 'break' AND a.state <> 'replaced' AND a.origin IS NULL AND a.starts_at > $2
		  AND b.recipe->'trailers' @> jsonb_build_array($1::text)`, titleID, now)
	return int(tag.RowsAffected()), err
}
