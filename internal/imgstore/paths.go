package imgstore

import (
	"path/filepath"
	"strings"

	"github.com/s-oravec/cage/internal/config"
)

var rootOverride string

// SetRoot overrides the storage root (testing only). Empty restores default.
func SetRoot(s string) { rootOverride = s }

// Root is the base directory under ~/.cage where layered store lives.
func Root() string {
	if rootOverride != "" {
		return rootOverride
	}
	return config.Dir()
}

func splitDigest(d string) (algo, hex string) {
	parts := strings.SplitN(d, ":", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		panic("digest must start with sha256:")
	}
	return parts[0], parts[1]
}

// LayerPath returns the on-disk path for a layer blob.
func LayerPath(digest string) string {
	algo, hex := splitDigest(digest)
	return filepath.Join(Root(), "layers", algo, hex[:2], hex, "layer.qcow2")
}

// ManifestPath returns the on-disk path for a manifest blob.
func ManifestPath(digest string) string {
	algo, hex := splitDigest(digest)
	return filepath.Join(Root(), "manifests", algo, hex[:2], hex, "manifest.json")
}

// LocalRefPath returns the on-disk ref path for a local-only tag.
func LocalRefPath(name, tag string) string {
	return filepath.Join(Root(), "refs", "_local", name, tag)
}

// RegistryRefPath returns the on-disk ref path for a registry-qualified tag.
func RegistryRefPath(host, owner, name, tag string) string {
	return filepath.Join(Root(), "refs", host, owner, name, tag)
}
