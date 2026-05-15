package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// SupportedOS is the closed whitelist of operating systems a v1 manifest may declare.
var SupportedOS = []string{"linux"}

// SupportedArch is the closed whitelist of CPU architectures a v1 manifest may declare.
var SupportedArch = []string{"amd64", "arm64"}

// Validate checks that a manifest conforms to the v1 schema: required fields
// are present, digests are sha256-prefixed, layers are non-empty, and
// config.os / config.arch fall within the supported whitelists. It is called
// on the build path (before publish) and on the pull path (after fetch).
func (m *Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("schemaVersion: want %d, got %d", SchemaVersionV1, m.SchemaVersion)
	}
	if m.MediaType != MediaTypeManifestV1 {
		return fmt.Errorf("mediaType: want %q, got %q", MediaTypeManifestV1, m.MediaType)
	}
	if m.Base.Type != "distro" {
		return fmt.Errorf("base.type: only %q supported", "distro")
	}
	if m.Base.Name == "" {
		return fmt.Errorf("base.name: required")
	}
	if !hasPrefix(m.Base.Digest, "sha256:") {
		return fmt.Errorf("base.digest: must be sha256:<hex>")
	}
	if len(m.Layers) == 0 {
		return fmt.Errorf("layers: at least one layer required")
	}
	for i, l := range m.Layers {
		if !hasPrefix(l.Digest, "sha256:") {
			return fmt.Errorf("layers[%d].digest: must be sha256:<hex>", i)
		}
		if l.Size <= 0 {
			return fmt.Errorf("layers[%d].size: must be > 0", i)
		}
		if l.MediaType != MediaTypeLayerV1 {
			return fmt.Errorf("layers[%d].mediaType: want %q", i, MediaTypeLayerV1)
		}
	}
	if !inList(m.Config.OS, SupportedOS) {
		return fmt.Errorf("config.os: must be one of %v, got %q", SupportedOS, m.Config.OS)
	}
	if !inList(m.Config.Arch, SupportedArch) {
		return fmt.Errorf("config.arch: must be one of %v, got %q", SupportedArch, m.Config.Arch)
	}
	if len(m.Config.Cagefile) > 64*1024 {
		return fmt.Errorf("config.cagefile: exceeds 64KB")
	}
	return nil
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func inList(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
