package manifest

// MediaTypeIndexV1 is the wire Content-Type returned by GET /manifests/:reference
// when the reference resolves to a multi-arch index.
const MediaTypeIndexV1 = "application/vnd.cage.index.v1+json"

// IndexBody is the deserialized form of a multi-arch index. CLI never composes one
// (the server auto-composes it in PUT /manifests/:tag); we only deserialize on pull.
// Field names match cage-hub packages/shared/src/dtos.ts ManifestIndexBodySchema.
type IndexBody struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []IndexEntry `json:"manifests"`
}

// IndexEntry references one per-architecture manifest within an index.
type IndexEntry struct {
	Digest   string   `json:"digest"`
	Platform Platform `json:"platform"`
}

// Platform identifies the target architecture of an index entry.
type Platform struct {
	Architecture string `json:"architecture"` // "amd64" | "arm64"
}
