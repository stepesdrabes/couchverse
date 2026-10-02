package couch

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// ProtocolSchema describes every frame in ServerFrames and ClientFrames as a
// JSON Schema document, so clients generate their protocol types instead of
// restating them. ServerFrame and ClientFrame are unions discriminated by the
// envelope's type.
func ProtocolSchema() map[string]any {
	doc, _ := protocolSchema()
	return doc
}

func protocolSchema() (map[string]any, huma.Registry) {
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
		"ServerFrame": union(ServerFrames),
		"ClientFrame": union(ClientFrames),
	}
	for name, schema := range registry.Map() {
		defs[name] = schema
	}
	doc := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "Couchverse couch protocol",
		"description": "Frames on the couch WebSocket (GET /api/v1/couch/{token}/ws). Every frame is {\"type\", \"data\"}; ServerFrame lists what the server sends and ClientFrame what clients send.",
		"$defs":       defs,
	}
	return doc, registry
}
