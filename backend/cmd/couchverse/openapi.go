package main

import (
	"encoding/json"
	"io"

	"couchverse/internal/feature/couch"
	"couchverse/internal/server"
)

// contractPrinters write the machine-readable contract clients are generated
// from; `make contract` stores them under contract/.
var contractPrinters = map[string]func(io.Writer) error{
	"openapi":      func(w io.Writer) error { return writeJSON(w, server.OpenAPI()) },
	"couch-schema": func(w io.Writer) error { return writeJSON(w, couch.ProtocolSchema()) },
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
