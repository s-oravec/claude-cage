package imgstore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayerPath_Shards(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	p := LayerPath("sha256:abcdef0123")
	assert.Equal(t, "/tmp/cc/layers/sha256/ab/abcdef0123/layer.qcow2", p)
}

func TestManifestPath_Shards(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	p := ManifestPath("sha256:0123456789")
	assert.Equal(t, "/tmp/cc/manifests/sha256/01/0123456789/manifest.json", p)
}

func TestLocalRefPath(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Equal(t, "/tmp/cc/refs/_local/myimage/v1", LocalRefPath("myimage", "v1"))
}

func TestRegistryRefPath(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Equal(t, "/tmp/cc/refs/cage-hub.io/stiivo/devbox/v1",
		RegistryRefPath("cage-hub.io", "stiivo", "devbox", "v1"))
}

func TestRejectsNonSha256(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Panics(t, func() { LayerPath("md5:abc") })
}

func TestRefDirsBelowSharding(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	// Common path prefix shape sanity check.
	assert.True(t, strings.HasPrefix(LayerPath("sha256:ff00"), "/tmp/cc/layers/sha256/ff/"))
}
