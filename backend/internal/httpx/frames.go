package httpx

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// Frame describes one message type of a WebSocket protocol (couch, Couch TV):
// its payload type (nil for none) and what it means. A protocol lists its frames
// once, in tables the server's code and its published schema both read.
type Frame struct {
	Type    string
	Payload any
	Doc     string
}

// FrameSchema describes a protocol's frames as a JSON Schema document, so clients
// generate their protocol types instead of restating them. ServerFrame and
// ClientFrame are unions discriminated by the envelope's type; the registry
// holds the payload schemas, for tests that validate frames against them.
func FrameSchema(title, description string, server, client []Frame) (map[string]any, huma.Registry) {
	registry := huma.NewMapRegistry("#/$defs/", huma.DefaultSchemaNamer)
	union := func(frames []Frame) map[string]any {
		variants := make([]any, 0, len(frames))
		for _, f := range frames {
			props := map[string]any{"type": map[string]any{"const": f.Type}}
			required := []string{"type"}
			if f.Payload != nil {
				props["data"] = registry.Schema(reflect.TypeOf(f.Payload), true, "")
				required = append(required, "data")
			}
			variants = append(variants, map[string]any{
				"type":                 "object",
				"description":          f.Doc,
				"properties":           props,
				"required":             required,
				"additionalProperties": false,
			})
		}
		return map[string]any{"oneOf": variants}
	}

	defs := map[string]any{
		"ServerFrame": union(server),
		"ClientFrame": union(client),
	}
	for name, schema := range registry.Map() {
		defs[name] = schema
	}
	doc := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       title,
		"description": description,
		"$defs":       defs,
	}
	return doc, registry
}
