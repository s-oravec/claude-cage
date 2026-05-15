package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	// MediaTypeManifestV1 is the media type for a cage-hub v1 image manifest.
	MediaTypeManifestV1 = "application/vnd.cage.manifest.v1+json"
	// MediaTypeLayerV1 is the media type for a cage-hub v1 qcow2 layer blob.
	MediaTypeLayerV1 = "application/vnd.cage.layer.v1.qcow2"
	// SchemaVersionV1 is the schema version for v1 cage-hub manifests.
	SchemaVersionV1 = 1
)

// Manifest describes a cage-hub image: its base, layered blobs, and runtime config.
type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	MediaType     string  `json:"mediaType"`
	Base          Base    `json:"base"`
	Layers        []Layer `json:"layers"`
	Config        Config  `json:"config"`
}

// Base identifies the parent image a manifest is built on top of.
type Base struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// Layer references a single content-addressed blob that composes the image.
type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

// Config carries the runtime configuration baked into a cage-hub image.
type Config struct {
	OS          string     `json:"os"`
	Arch        string     `json:"arch"`
	User        string     `json:"user,omitempty"`
	Workdir     string     `json:"workdir,omitempty"`
	Env         []string   `json:"env,omitempty"`
	Description string     `json:"description,omitempty"`
	Readme      string     `json:"readme,omitempty"`
	Cagefile    string     `json:"cagefile,omitempty"`
	Resources   *Resources `json:"resources,omitempty"`
}

// Resources declares default VM resource requests (memory, vCPU, disk) for the image.
type Resources struct {
	MemoryMB int `json:"memory_mb,omitempty"`
	VCPU     int `json:"vcpu,omitempty"`
	DiskGB   int `json:"disk_gb,omitempty"`
}

// Canonical returns the deterministic JSON encoding used for digest
// computation and network transport. Struct field order is fixed, so
// stdlib json.Marshal is sufficient.
func Canonical(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// Digest computes the sha256:<hex> of the canonical encoding.
func Digest(m *Manifest) (string, error) {
	data, err := Canonical(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// DigestBytes computes sha256:<hex> of arbitrary bytes (used for layer
// blobs read from disk).
func DigestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
