package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"couchverse/internal/version"
)

func init() {
	// Handlers encode empty lists as [] (the contract tests reject null), so
	// arrays are typed non-nullable and every client decodes plain lists.
	huma.DefaultArrayNullable = false
	huma.NewError = newError
}

// NewAPI creates the typed API on r, which is mounted at /api/v1. Operations
// register on it (directly or through a huma.Group) so the OpenAPI document is
// derived from the handler types and cannot drift from them.
func NewAPI(r chi.Router) huma.API {
	cfg := huma.DefaultConfig("Couchverse", strconv.Itoa(version.APILevel))
	// the spec is published through `couchverse openapi`, not served, and
	// responses carry no $schema links
	cfg.OpenAPIPath, cfg.DocsPath, cfg.SchemasPath = "", "", ""
	cfg.CreateHooks = nil
	cfg.Servers = []*huma.Server{{URL: "/api/v1"}}
	return humachi.New(r, cfg)
}

// newError adapts huma's own failures (validation, body limits, handler errors
// that are not an APIError) to the API envelope. A wrapped db.ErrNotFound is a
// 404 wherever it surfaces.
func newError(status int, msg string, errs ...error) huma.StatusError {
	for _, err := range errs {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		if errors.Is(err, ErrNotFound) {
			return NotFoundError()
		}
	}
	switch {
	case status >= http.StatusInternalServerError:
		return InternalError(fmt.Errorf("%s: %w", msg, errors.Join(errs...)))
	case status == http.StatusUnprocessableEntity:
		// a path that does not parse names no resource; other invalid input
		// was always a 400 here, so keep it one
		if inPath(errs) {
			return NotFoundError()
		}
		return BadRequestError(detailMessage(msg, errs))
	}
	return Fail(status, statusCode(status), detailMessage(msg, errs))
}

func inPath(errs []error) bool {
	for _, err := range errs {
		var d *huma.ErrorDetail
		if errors.As(err, &d) && strings.HasPrefix(d.Location, "path.") {
			return true
		}
	}
	return false
}

func detailMessage(msg string, errs []error) string {
	var parts []string
	for _, err := range errs {
		var d *huma.ErrorDetail
		if errors.As(err, &d) && d.Location != "" {
			parts = append(parts, d.Location+": "+d.Message)
		} else if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return msg
	}
	return strings.Join(parts, "; ")
}

func statusCode(status int) string {
	switch status {
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusRequestEntityTooLarge:
		return "too_large"
	}
	return strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
}

// Localized marks a read whose catalog text follows the display language: it
// documents ?lang= and puts it on the context for the stores (see LangFrom).
func Localized(op huma.Operation) huma.Operation {
	op.Parameters = append(op.Parameters, &huma.Param{
		Name:        "lang",
		In:          "query",
		Description: "Display language (ISO 639-1). Empty serves the base text.",
		Schema:      &huma.Schema{Type: huma.TypeString},
	})
	op.Middlewares = append(op.Middlewares, func(ctx huma.Context, next func(huma.Context)) {
		next(huma.WithContext(ctx, WithLang(ctx.Context(), ctx.Query("lang"))))
	})
	return op
}

// Guard is middleware that rejects a request when check returns an error;
// install it on a group for auth or feature-flag gates.
func Guard(api huma.API, check func(context.Context) error) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if err := check(ctx.Context()); err != nil {
			Reject(api, ctx, err)
			return
		}
		next(ctx)
	}
}

// Reject answers a request from middleware with err as the API would from a
// handler: an APIError as is, anything else as a logged 500.
func Reject(api huma.API, ctx huma.Context, err error) {
	se := newError(http.StatusInternalServerError, "middleware", err)
	_ = huma.WriteErr(api, ctx, se.GetStatus(), se.Error(), se)
}

// Raw registers a route served by a plain handler (byte streams, WebSockets,
// multipart, beacons) through the same API, so it shares the group's prefix,
// middleware and security and is still documented for typed clients.
func Raw(api huma.API, op huma.Operation, h http.HandlerFunc) {
	if d, ok := api.(huma.OperationDocumenter); ok {
		d.DocumentOperation(&op)
	} else {
		api.OpenAPI().AddOperation(&op)
	}
	api.Adapter().Handle(&op, api.Middlewares().Handler(op.Middlewares.Handler(func(ctx huma.Context) {
		r, w := humachi.Unwrap(ctx)
		h(w, r)
	})))
}

// Routes are the API groups a feature registers its operations on.
type Routes struct {
	Public huma.API // anyone, including anonymous couch followers
	User   huma.API // a signed-in user
	Admin  huma.API // an admin; paths are prefixed with /admin
	// Media serves one media file to whoever holds its grant (grant.From):
	// paths are prefixed with /media/{grant}
	Media huma.API
	// Artwork admits a signed-in user or the holder of an artwork grant (?g=)
	Artwork huma.API
}

// Tag is a feature's OpenAPI tag; its methods describe the feature's operations.
type Tag string

func (t Tag) Op(id, method, path string) huma.Operation {
	return huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{string(t)}}
}

// NoContent describes an operation that answers 204 on success.
func (t Tag) NoContent(id, method, path string) huma.Operation {
	op := t.Op(id, method, path)
	op.DefaultStatus = http.StatusNoContent
	return op
}

// Created describes an operation that answers 201 with the created resource.
func (t Tag) Created(id, method, path string) huma.Operation {
	op := t.Op(id, method, path)
	op.DefaultStatus = http.StatusCreated
	return op
}

// Accepted describes an operation that answers 202: the work runs as a queued job.
func (t Tag) Accepted(id, method, path string) huma.Operation {
	op := t.Op(id, method, path)
	op.DefaultStatus = http.StatusAccepted
	return op
}

// ErrorResponse documents the error envelope for a raw operation, matching what
// typed operations get automatically.
func ErrorResponse(api huma.API) *huma.Response {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[APIError](), true, "")
	return &huma.Response{
		Description: "Error",
		Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
	}
}
