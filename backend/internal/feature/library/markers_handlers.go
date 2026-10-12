package library

import (
	"context"
	"fmt"
	"net/http"

	"couchverse/internal/httpx"
)

// AdminMarkers serves the markers editor.
type AdminMarkers struct {
	store    *Store
	analyzer *MarkerAnalyzer
}

type markersOutput struct{ Body MediaMarkers }

// Get reads a file's markers, the analysis' and the admin's.
func (h *AdminMarkers) Get(ctx context.Context, in *idInput) (*markersOutput, error) {
	m, err := h.store.MediaMarkers(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &markersOutput{Body: *m}, nil
}

// MediaMarkerEdit is an admin's marker of one kind.
type MediaMarkerEdit struct {
	Kind    string `json:"kind" enum:"intro,credits,credits_music"`
	StartMs *int   `json:"startMs,omitempty" required:"false" minimum:"0" doc:"Required unless none."`
	EndMs   *int   `json:"endMs,omitempty" required:"false" minimum:"1" doc:"Required for intro and credits_music; optional for credits."`
	None    bool   `json:"none,omitempty" required:"false" doc:"The file has none of this kind, whatever the analysis found."`
}

// MediaMarkersUpdate is the full set of a file's admin markers.
type MediaMarkersUpdate struct {
	Markers []MediaMarkerEdit `json:"markers" doc:"One per kind at most; a kind left out falls back to the analysis."`
}

type updateMarkersInput struct {
	ID   string `path:"id" format:"uuid"`
	Body MediaMarkersUpdate
}

// Update replaces a file's admin markers and answers with all of them. Couch TV picks
// its promo again from the new ones, and its schedule airs to the new credits from its
// next plan run.
func (h *AdminMarkers) Update(ctx context.Context, in *updateMarkersInput) (*markersOutput, error) {
	current, err := h.store.MediaMarkers(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	markers, err := validMarkers(in.Body.Markers, current.DurationMs)
	if err != nil {
		return nil, err
	}
	if err := h.store.SetAdminMarkers(ctx, in.ID, markers); err != nil {
		return nil, err
	}
	if h.analyzer != nil {
		if err := h.analyzer.FileChanged(ctx, in.ID); err != nil {
			return nil, err
		}
	}
	return h.Get(ctx, &idInput{ID: in.ID})
}

func validMarkers(edits []MediaMarkerEdit, durationMs int) ([]MediaMarker, error) {
	seen := map[string]bool{}
	out := []MediaMarker{}
	for _, e := range edits {
		bad := func(why string) error {
			return httpx.Fail(http.StatusBadRequest, "invalid_marker", fmt.Sprintf("%s: %s", e.Kind, why))
		}
		if seen[e.Kind] {
			return nil, bad("given twice")
		}
		seen[e.Kind] = true
		m := MediaMarker{Kind: e.Kind, Source: MarkerAdmin, None: e.None}
		switch {
		case e.None:
			if e.StartMs != nil || e.EndMs != nil {
				return nil, bad("none takes no times")
			}
		case e.StartMs == nil:
			return nil, bad("a start is required")
		case e.EndMs == nil && e.Kind != MarkerCredits:
			return nil, bad("an end is required")
		case e.EndMs != nil && *e.EndMs <= *e.StartMs:
			return nil, bad("the end must be after the start")
		case durationMs > 0 && (*e.StartMs >= durationMs || e.EndMs != nil && *e.EndMs > durationMs):
			return nil, bad("past the end of the file")
		default:
			m.StartMs, m.EndMs = e.StartMs, e.EndMs
		}
		out = append(out, m)
	}
	return out, nil
}
