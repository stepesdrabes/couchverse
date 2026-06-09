// Package web embeds the built SvelteKit SPA. The frontend build is copied
// into dist/ by `make build` and the Dockerfile; in a bare checkout only
// .gitkeep is present and the server responds with a "frontend not built" hint.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
