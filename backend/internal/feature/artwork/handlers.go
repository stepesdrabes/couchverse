package artwork

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/grant"
	"couchverse/internal/httpx"
)

type Handlers struct {
	service *Service
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

const tag httpx.Tag = "artwork"

func (h *Handlers) Register(rt httpx.Routes) {
	httpx.Raw(rt.Artwork, serveOp(rt.Artwork), h.Serve)

	huma.Register(rt.Admin, tag.Created("adminUploadArtwork", http.MethodPost, "/artwork"), h.Upload)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteArtwork", http.MethodDelete, "/artwork/{id}"), h.Delete)
}

// serveOp documents the image route, which stays a plain handler because it
// answers with file bytes (and conditional 304s) rather than JSON.
func serveOp(api huma.API) huma.Operation {
	op := tag.Op("getArtwork", http.MethodGet, "/artwork/{id}")
	op.Summary = "Get an artwork image"
	op.Description = "The stored original (JPEG, PNG or WebP), or with size a cached JPEG resize."
	op.Parameters = []*huma.Param{
		{Name: "id", In: "path", Required: true, Schema: &huma.Schema{Type: huma.TypeString, Format: "uuid"}},
		{Name: "size", In: "query", Description: "Resize to this width; the original when omitted.",
			Schema: &huma.Schema{Type: huma.TypeString, Enum: []any{"w342", "w780"}}},
		{Name: "v", In: "query", Description: "Version token (the artwork's createdAt); a versioned URL is cached as immutable.",
			Schema: &huma.Schema{Type: huma.TypeString}},
		{Name: "g", In: "query", Description: "An artwork grant, for a request without the session (a system image fetch, an anonymous couch guest).",
			Schema: &huma.Schema{Type: huma.TypeString}},
	}
	image := &huma.MediaType{Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}}
	op.Responses = map[string]*huma.Response{
		"200": {Description: "The image", Content: map[string]*huma.MediaType{
			"image/jpeg": image, "image/png": image, "image/webp": image,
		}},
		"304":     {Description: "Not modified since If-Modified-Since"},
		"default": httpx.ErrorResponse(api),
	}
	return op
}

// ArtworkGrant lets a system component fetch artwork without the app's session
// (tvOS Top Shelf, AirPlay receivers): append it to an artwork URL as ?g=.
type ArtworkGrant struct {
	Grant     string `json:"grant"`
	ExpiresIn int    `json:"expiresIn" doc:"Seconds until the grant expires."`
}

// IssueGrant signs an artwork grant to subject (0 for an anonymous couch guest).
func IssueGrant(grants *grant.Signer, subject int64) ArtworkGrant {
	return ArtworkGrant{
		Grant:     grants.Issue(grant.Grant{Scope: grant.Artwork, Subject: subject}, grant.ArtworkTTL),
		ExpiresIn: int(grant.ArtworkTTL.Seconds()),
	}
}

// Serve returns the artwork image, resized on first request when ?size= is given.
func (h *Handlers) Serve(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	art, err := h.service.Store.ArtworkByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	path, err := h.service.Resolve(r.Context(), art, r.URL.Query().Get("size"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// versioned URLs (?v=<token>) carry the artwork's updated time, so the bytes
	// for a given URL never change - cache them hard. Unversioned URLs keep
	// revalidating (ServeFile answers 304 via Last-Modified) so replaced artwork
	// shows up immediately even from a call site that doesn't pass a version.
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	httpx.ServeFile(w, r, path)
}

type uploadForm struct {
	OwnerKind string        `form:"ownerKind" enum:"title,season,episode"`
	OwnerID   string        `form:"ownerId"`
	Kind      string        `form:"kind" enum:"poster,backdrop,thumb"`
	File      huma.FormFile `form:"file" doc:"A .jpg, .jpeg, .png or .webp image (checked by extension)."`
}

type uploadInput struct {
	RawBody huma.MultipartFormFiles[uploadForm]
}

type artworkOutput struct{ Body *Artwork }

// Upload stores an image in an owner's artwork slot, replacing any previous one.
func (h *Handlers) Upload(ctx context.Context, in *uploadInput) (*artworkOutput, error) {
	form := in.RawBody.Data()
	defer form.File.Close()
	art, err := h.service.Save(ctx, form.OwnerKind, form.OwnerID, form.Kind, form.File.Filename, form.File)
	if err != nil {
		return nil, httpx.Fail(http.StatusBadRequest, "artwork_failed", err.Error())
	}
	return &artworkOutput{Body: art}, nil
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

// Delete removes the artwork row, its file and cached resizes.
func (h *Handlers) Delete(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.service.Delete(ctx, in.ID)
}
