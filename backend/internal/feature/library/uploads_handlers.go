package library

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"

	"github.com/go-chi/chi/v5"
)

type AdminUploads struct {
	store   *Store
	manager *Manager
}

func NewAdminUploads(st *Store, manager *Manager) *AdminUploads {
	return &AdminUploads{store: st, manager: manager}
}

type uploadsOutput struct{ Body []UploadSession }

func (h *AdminUploads) List(ctx context.Context, _ *struct{}) (*uploadsOutput, error) {
	sessions, err := h.store.ActiveUploadSessions(ctx)
	if err != nil {
		return nil, err
	}
	return &uploadsOutput{Body: sessions}, nil
}

type NewUpload struct {
	Filename string `json:"filename" minLength:"1"`
	Size     int64  `json:"size" minimum:"1" doc:"Total file size in bytes."`
}

type createUploadInput struct{ Body NewUpload }

type uploadOutput struct{ Body *UploadSession }

func (h *AdminUploads) Create(ctx context.Context, in *createUploadInput) (*uploadOutput, error) {
	session, err := h.manager.Create(ctx, auth.UserFrom(ctx).ID, in.Body.Filename, in.Body.Size)
	if err != nil {
		return nil, err
	}
	return &uploadOutput{Body: session}, nil
}

func (h *AdminUploads) Get(ctx context.Context, in *idInput) (*uploadOutput, error) {
	session, err := h.store.UploadSession(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &uploadOutput{Body: session}, nil
}

// UploadOffset is how many bytes of an upload the server holds; the next
// chunk starts there.
type UploadOffset struct {
	Offset int64 `json:"offset"`
}

// Append is a raw handler: the chunk is the unparsed request body.
func (h *AdminUploads) Append(w http.ResponseWriter, r *http.Request) {
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		httpx.BadRequest(w, "offset query parameter is required")
		return
	}

	newOffset, err := h.manager.Append(r.Context(), chi.URLParam(r, "id"), offset, r.Body)
	if err != nil {
		var mismatch *ErrOffsetMismatch
		switch {
		case errors.As(err, &mismatch):
			httpx.JSON(w, http.StatusConflict, UploadOffset{Offset: mismatch.Offset})
		case errors.Is(err, httpx.ErrNotFound):
			httpx.NotFound(w)
		default:
			httpx.Error(w, http.StatusBadRequest, "append_failed", err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, UploadOffset{Offset: newOffset})
}

type completeUploadInput struct {
	ID   string `path:"id" format:"uuid"`
	Body UploadAssignment
}

// CompletedUpload names the media file an upload became; it is probed next.
type CompletedUpload struct {
	MediaFileID string `json:"mediaFileId"`
}

type completeUploadOutput struct{ Body CompletedUpload }

func (h *AdminUploads) Complete(ctx context.Context, in *completeUploadInput) (*completeUploadOutput, error) {
	mediaFileID, err := h.manager.Complete(ctx, in.ID, in.Body)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			return nil, httpx.NotFoundError()
		}
		return nil, httpx.Fail(http.StatusBadRequest, "complete_failed", err.Error())
	}
	return &completeUploadOutput{Body: CompletedUpload{MediaFileID: mediaFileID}}, nil
}

func (h *AdminUploads) Abort(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.manager.Abort(ctx, in.ID)
}
