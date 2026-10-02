package auth

import (
	"context"
	"net/http"

	"couchverse/internal/httpx"
)

// SignedIn is an httpx.Guard check: the request must carry a session.
func SignedIn(ctx context.Context) error {
	if UserFrom(ctx) == nil {
		return httpx.Fail(http.StatusUnauthorized, "unauthorized", "authentication required")
	}
	return nil
}

// Admin is an httpx.Guard check: the request must carry an admin session.
func Admin(ctx context.Context) error {
	u := UserFrom(ctx)
	if u == nil {
		return httpx.Fail(http.StatusUnauthorized, "unauthorized", "authentication required")
	}
	if u.Role != "admin" {
		return httpx.Fail(http.StatusForbidden, "forbidden", "admin access required")
	}
	return nil
}
