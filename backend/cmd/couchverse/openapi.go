package main

import (
	"encoding/json"
	"io"

	"couchverse/internal/server"
)

// printOpenAPI writes the OpenAPI 3.1 document clients are generated from
// (`make contract` stores it as contract/openapi.json).
func printOpenAPI(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(server.OpenAPI())
}
