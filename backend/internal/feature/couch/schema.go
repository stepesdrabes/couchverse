package couch

import (
	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

// ProtocolSchema describes every frame in ServerFrames and ClientFrames as a
// JSON Schema document (contract/couch-protocol.schema.json).
func ProtocolSchema() map[string]any {
	doc, _ := protocolSchema()
	return doc
}

func protocolSchema() (map[string]any, huma.Registry) {
	return httpx.FrameSchema("Couchverse couch protocol",
		"Frames on the couch WebSocket (GET /api/v1/couch/{token}/ws). Every frame is {\"type\", \"data\"}; ServerFrame lists what the server sends and ClientFrame what clients send.",
		ServerFrames, ClientFrames)
}
