// Package broadcast embeds Couch TV's broadcast bundle (clients/broadcast's render page),
// which the render sidecar serves to headless Chromium. `make build` and the Dockerfile copy
// the build into dist/; in a bare checkout only .gitkeep is present and the sidecar refuses
// to start.
package broadcast

import "embed"

//go:embed all:dist
var Dist embed.FS
