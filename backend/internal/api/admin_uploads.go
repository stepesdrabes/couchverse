package api

import (
	"errors"
	"net/http"
	"strconv"

	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/internal/upload"

	"github.com/go-chi/chi/v5"
)

type AdminUploads struct {
	store   *store.Store
	manager *upload.Manager
}

func NewAdminUploads(st *store.Store, manager *upload.Manager) *AdminUploads {
	return &AdminUploads{store: st, manager: manager}
}

func (h *AdminUploads) List(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.ActiveUploadSessions(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sessions)
}

func (h *AdminUploads) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Filename == "" || req.Size <= 0 {
		httpx.BadRequest(w, "filename and a positive size are required")
		return
	}
	session, err := h.manager.Create(r.Context(), auth.UserFrom(r.Context()).ID, req.Filename, req.Size)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, session)
}

func (h *AdminUploads) Get(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.UploadSession(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *AdminUploads) Append(w http.ResponseWriter, r *http.Request) {
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		httpx.BadRequest(w, "offset query parameter is required")
		return
	}

	newOffset, err := h.manager.Append(r.Context(), chi.URLParam(r, "id"), offset, r.Body)
	if err != nil {
		var mismatch *upload.ErrOffsetMismatch
		switch {
		case errors.As(err, &mismatch):
			httpx.JSON(w, http.StatusConflict, map[string]int64{"offset": mismatch.Offset})
		case errors.Is(err, httpx.ErrNotFound):
			httpx.NotFound(w)
		default:
			httpx.Error(w, http.StatusBadRequest, "append_failed", err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"offset": newOffset})
}

func (h *AdminUploads) Complete(w http.ResponseWriter, r *http.Request) {
	var assign upload.Assign
	if err := httpx.Decode(r, &assign); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if assign.LibraryKind != "movies" && assign.LibraryKind != "series" && assign.LibraryKind != "music" {
		httpx.BadRequest(w, "libraryKind must be movies, series or music")
		return
	}
	mediaFileID, err := h.manager.Complete(r.Context(), chi.URLParam(r, "id"), assign)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			httpx.NotFound(w)
			return
		}
		httpx.Error(w, http.StatusBadRequest, "complete_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"mediaFileId": mediaFileID})
}

func (h *AdminUploads) Abort(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.Abort(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
