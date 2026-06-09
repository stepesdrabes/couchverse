// Package migrations embeds the goose SQL migrations run at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
