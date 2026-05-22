package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/s-oravec/cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetManifest_ReturnsBodyAndDigest(t *testing.T) {
	body := []byte(`{"schemaVersion":1}`)
	digest := "sha256:" + hex.EncodeToString(sha256Sum(body))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/s/d/manifests/v1", r.URL.Path)
		w.Header().Set("Content-Type", "application/vnd.cage.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", digest)
		w.Write(body)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	got, gotContentType, gotDigest, err := c.GetManifest("s", "d", "v1")
	require.NoError(t, err)
	assert.Equal(t, body, got)
	assert.Equal(t, manifest.MediaTypeManifestV1, gotContentType)
	assert.Equal(t, digest, gotDigest)
}

func TestGetManifest_ReturnsIndexContentType(t *testing.T) {
	body := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.cage.index.v1+json","manifests":[]}`)
	digest := "sha256:idx" + hex.EncodeToString(sha256Sum(body))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/s/d/manifests/v1", r.URL.Path)
		w.Header().Set("Content-Type", manifest.MediaTypeIndexV1)
		w.Header().Set("Docker-Content-Digest", digest)
		w.Write(body)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	got, gotContentType, gotDigest, err := c.GetManifest("s", "d", "v1")
	require.NoError(t, err)
	assert.NotEmpty(t, got)
	assert.Equal(t, manifest.MediaTypeIndexV1, gotContentType)
	assert.Equal(t, digest, gotDigest)
}

func TestPutManifest_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "application/vnd.cage.manifest.v1+json", r.Header.Get("Content-Type"))
		assert.Equal(t, "true", r.Header.Get("X-As-Latest"))
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(201)
		w.Write([]byte(`{"tag":"v1","manifest_digest":"sha256:abc","latest_updated":true}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	res, err := c.PutManifest("s", "d", "v1", []byte(`{}`), true)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc", res.ManifestDigest)
	assert.True(t, res.LatestUpdated)
}

func TestPutManifest_DecodesTagTarget(t *testing.T) {
	const targetDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"tag": "1.0",
			"manifest_digest": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			"tag_target_kind": "index",
			"tag_target_digest": "` + targetDigest + `",
			"latest_updated": false
		}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	res, err := c.PutManifest("s", "d", "1.0", []byte(`{}`), false)
	require.NoError(t, err)
	assert.Equal(t, "index", res.TagTargetKind)
	assert.Equal(t, targetDigest, res.TagTargetDigest)
}

func sha256Sum(b []byte) []byte { s := sha256.Sum256(b); return s[:] }

// Unused but useful helper for future tests:
func bodyString(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	return strings.TrimSpace(string(b))
}
