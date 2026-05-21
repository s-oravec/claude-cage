package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagCmd_Args(t *testing.T) {
	c := NewTagCmd()
	c.SetArgs([]string{"only-one"})
	c.SilenceUsage = true
	c.SilenceErrors = true
	err := c.Execute()
	require.Error(t, err)
}

// TestTagCmd_ParentAcceptsTwoArgs verifies the bare tagging behavior still works:
// `cage tag a b` hits the parent RunE and creates a ref.
func TestTagCmd_ParentAcceptsTwoArgs(t *testing.T) {
	imgstore.SetRoot(filepath.Join(t.TempDir(), "store"))
	defer imgstore.SetRoot("")

	// Seed a source ref pointing at a manifest digest.
	src, err := imgstore.ParseRef("base:1.0")
	require.NoError(t, err)
	digest := "sha256:" + strings.Repeat("a", 64)
	require.NoError(t, imgstore.WriteRef(src, digest))

	c := NewTagCmd()
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{"base:1.0", "copy:1.0"})
	require.NoError(t, c.Execute())

	dst, err := imgstore.ParseRef("copy:1.0")
	require.NoError(t, err)
	got, err := imgstore.ReadRef(dst)
	require.NoError(t, err)
	assert.Equal(t, digest, got)
}

// TestTagCmd_HasInspectSubcommand verifies cobra routes `tag inspect` to the
// subcommand while the parent still exists with ExactArgs(2).
func TestTagCmd_HasInspectSubcommand(t *testing.T) {
	c := NewTagCmd()
	sub, _, err := c.Find([]string{"inspect"})
	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, "inspect <ref>", sub.Use)
}

func TestPrintTagInspect(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		digest string
		arches string
	}{
		{"manifest", "manifest", "sha256:abc", "amd64"},
		{"index", "index", "sha256:idx", "amd64, arm64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, printTagInspect(&buf, tc.kind, tc.digest, tc.arches))
			out := buf.String()
			assert.Contains(t, out, "Kind:")
			assert.Contains(t, out, tc.kind)
			assert.Contains(t, out, "Digest:")
			assert.Contains(t, out, tc.digest)
			assert.Contains(t, out, "Architectures:")
			assert.Contains(t, out, tc.arches)
		})
	}
}

func TestTagInspectFromManifest_Manifest(t *testing.T) {
	body := manifestBody(t, "amd64")
	kind, digest, arches, err := tagInspectFromManifest(
		manifest.MediaTypeManifestV1, body, "sha256:m")
	require.NoError(t, err)
	assert.Equal(t, "manifest", kind)
	assert.Equal(t, "sha256:m", digest)
	assert.Equal(t, "amd64", arches)
}

func TestTagInspectFromManifest_Index(t *testing.T) {
	body := indexBody(t, "amd64", "arm64")
	kind, digest, arches, err := tagInspectFromManifest(
		manifest.MediaTypeIndexV1, body, "sha256:idx")
	require.NoError(t, err)
	assert.Equal(t, "index", kind)
	assert.Equal(t, "sha256:idx", digest)
	assert.Equal(t, "amd64, arm64", arches)
}

func TestTagInspectFromManifest_UnexpectedContentType(t *testing.T) {
	_, _, _, err := tagInspectFromManifest("text/plain", []byte("{}"), "sha256:x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "text/plain")
}

// newInsecureRegistryConfig points config.Load() at a temp dir whose config marks
// host as an insecure registry, so registry.NewClient uses plain http for httptest.
func newInsecureRegistryConfig(t *testing.T, host string) {
	t.Helper()
	dir := t.TempDir()
	config.SetDir(dir)
	t.Cleanup(func() { config.SetDir("") })

	cfg := config.DefaultConfig()
	cfg.Registries.Insecure = []string{host}
	require.NoError(t, config.Save(cfg))
}

func TestInspectTag_Registry_Manifest(t *testing.T) {
	body := manifestBody(t, "amd64")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", manifest.MediaTypeManifestV1)
		w.Header().Set("Docker-Content-Digest", "sha256:manifestdigest")
		w.Write(body)
	}))
	defer srv.Close()

	host := srv.URL[len("http://"):]
	newInsecureRegistryConfig(t, host)

	cmd := NewTagCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	require.NoError(t, inspectTag(cmd, host+"/owner/name:latest"))

	out := buf.String()
	assert.Contains(t, out, "manifest")
	assert.Contains(t, out, "sha256:manifestdigest")
	assert.Contains(t, out, "amd64")
}

func TestInspectTag_Registry_Index(t *testing.T) {
	body := indexBody(t, "amd64", "arm64")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", manifest.MediaTypeIndexV1)
		w.Header().Set("Docker-Content-Digest", "sha256:indexdigest")
		w.Write(body)
	}))
	defer srv.Close()

	host := srv.URL[len("http://"):]
	newInsecureRegistryConfig(t, host)

	cmd := NewTagCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	require.NoError(t, inspectTag(cmd, host+"/owner/name:latest"))

	out := buf.String()
	assert.Contains(t, out, "index")
	assert.Contains(t, out, "sha256:indexdigest")
	assert.Contains(t, out, "amd64, arm64")
}

func TestInspectTag_Local_SingleArch(t *testing.T) {
	imgstore.SetRoot(filepath.Join(t.TempDir(), "store"))
	defer imgstore.SetRoot("")

	body := manifestBody(t, "arm64")
	digest := manifest.DigestBytes(body)
	require.NoError(t, imgstore.PutManifestBytes(digest, body))

	ref, err := imgstore.ParseRef("myimg:1.0")
	require.NoError(t, err)
	require.NoError(t, imgstore.WriteRef(ref, digest))

	cmd := NewTagCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	require.NoError(t, inspectTag(cmd, "myimg:1.0"))

	out := buf.String()
	assert.Contains(t, out, "manifest")
	assert.Contains(t, out, digest)
	assert.Contains(t, out, "arm64")
	assert.NotContains(t, out, "amd64")
}

func TestInspectTag_Local_NotFound(t *testing.T) {
	imgstore.SetRoot(filepath.Join(t.TempDir(), "store"))
	defer imgstore.SetRoot("")

	cmd := NewTagCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	err := inspectTag(cmd, "nope:1.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image not found")
}
