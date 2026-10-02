package subtitles

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/library"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
)

type Subtitles struct {
	store   *Store
	library *library.Store
	service *Service
}

func NewSubtitles(st *Store, lib *library.Store, service *Service) *Subtitles {
	return &Subtitles{store: st, library: lib, service: service}
}

// Serve returns the WebVTT file for a subtitle track.
func (h *Subtitles) Serve(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	sub, err := h.store.SubtitleByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if g, _ := grant.From(r.Context()); sub.MediaFileID != g.Resource.String() {
		httpx.NotFound(w)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	httpx.ServeFile(w, r, h.service.Path(sub))
}

type uploadForm struct {
	Lang  string        `form:"lang" required:"false" doc:"Language code; und when omitted."`
	Label string        `form:"label" required:"false" doc:"Track name in the player; the upper-cased language when omitted."`
	File  huma.FormFile `form:"file" doc:"A .srt or .vtt file (checked by extension); .srt is converted to WebVTT."`
}

type uploadInput struct {
	ID      string `path:"id" format:"uuid"`
	RawBody huma.MultipartFormFiles[uploadForm]
}

type subtitleOutput struct{ Body *media.Subtitle }

// Upload adds a subtitle track to a media file.
func (h *Subtitles) Upload(ctx context.Context, in *uploadInput) (*subtitleOutput, error) {
	form := in.RawBody.Data()
	defer form.File.Close()
	if _, err := h.library.MediaFileByID(ctx, in.ID); err != nil {
		return nil, err
	}
	sub, err := h.service.SaveUpload(ctx, in.ID, form.Lang, form.Label, form.File.Filename, form.File)
	if err != nil {
		return nil, httpx.Fail(http.StatusBadRequest, "subtitle_failed", err.Error())
	}
	return &subtitleOutput{Body: sub}, nil
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

// Delete removes a subtitle track and its file.
func (h *Subtitles) Delete(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.service.Delete(ctx, in.ID)
}

type subtitlesOutput struct{ Body []media.Subtitle }

func (h *Subtitles) ListForMediaFile(ctx context.Context, in *idInput) (*subtitlesOutput, error) {
	subs, err := h.store.SubtitlesForMediaFile(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &subtitlesOutput{Body: subs}, nil
}
