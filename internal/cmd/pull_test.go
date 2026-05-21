package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestBody builds a JSON-encoded single-arch manifest with the given arch.
func manifestBody(t *testing.T, arch string) []byte {
	t.Helper()
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Config:        manifest.Config{OS: "linux", Arch: arch},
	}
	b, err := json.Marshal(&m)
	require.NoError(t, err)
	return b
}

// indexBody builds a JSON-encoded multi-arch index over the given arches.
func indexBody(t *testing.T, arches ...string) []byte {
	t.Helper()
	idx := manifest.IndexBody{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeIndexV1,
	}
	for _, a := range arches {
		idx.Manifests = append(idx.Manifests, manifest.IndexEntry{
			Digest:   "sha256:" + a,
			Platform: manifest.Platform{Architecture: a},
		})
	}
	b, err := json.Marshal(&idx)
	require.NoError(t, err)
	return b
}

func TestSelectArchManifest_SingleManifest_ArchMatch(t *testing.T) {
	body := manifestBody(t, "amd64")
	fetchCalled := false
	fetch := func(reference string) ([]byte, string, error) {
		fetchCalled = true
		return nil, "", errors.New("fetch should not be called")
	}

	selBody, selDigest, m, err := selectArchManifest(
		"amd64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.False(t, fetchCalled, "fetch must not be called for a single manifest")
	assert.Equal(t, body, selBody)
	assert.Equal(t, "sha256:idx", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "amd64", m.Config.Arch)
}

func TestSelectArchManifest_SingleManifest_ArchMismatch_NoOverride_Errors(t *testing.T) {
	body := manifestBody(t, "arm64")
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"amd64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--platform arm64")
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_SingleManifest_ArchMismatch_WithPlatform_Proceeds(t *testing.T) {
	body := manifestBody(t, "arm64")
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	selBody, selDigest, m, err := selectArchManifest(
		"arm64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.Equal(t, body, selBody)
	assert.Equal(t, "sha256:idx", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "arm64", m.Config.Arch)
}

func TestSelectArchManifest_Index_PicksMatchingArch(t *testing.T) {
	body := indexBody(t, "amd64", "arm64")
	arm64Manifest := manifestBody(t, "arm64")

	var fetchedRef string
	fetch := func(reference string) ([]byte, string, error) {
		fetchedRef = reference
		return arm64Manifest, "sha256:arm64-docker-digest", nil
	}

	selBody, selDigest, m, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.Equal(t, "sha256:arm64", fetchedRef, "fetch must be called with the arm64 entry digest")
	assert.Equal(t, arm64Manifest, selBody)
	assert.Equal(t, "sha256:arm64-docker-digest", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "arm64", m.Config.Arch)
}

func TestSelectArchManifest_Index_NoMatchingArch_Errors(t *testing.T) {
	body := indexBody(t, "amd64")
	fetchCalled := false
	fetch := func(reference string) ([]byte, string, error) {
		fetchCalled = true
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.False(t, fetchCalled, "fetch must not be called when no arch matches")
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_Index_EntryArchMismatch_Errors(t *testing.T) {
	body := indexBody(t, "arm64")
	// The index entry claims arm64, but the fetched manifest is actually amd64.
	mismatchedManifest := manifestBody(t, "amd64")
	fetch := func(reference string) ([]byte, string, error) {
		return mismatchedManifest, "sha256:amd64-docker-digest", nil
	}

	_, _, _, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_UnexpectedContentType_Errors(t *testing.T) {
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"amd64", "text/plain", []byte("{}"), "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "text/plain")
}

func TestPullCmd_Exists(t *testing.T) {
	cmd := NewPullCmd()

	assert.Equal(t, "pull", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestPullCmd_HasBaseFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("base")
	assert.NotNil(t, flag)
}

func TestPullCmd_HasListFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("list")
	assert.NotNil(t, flag)
}

func TestPullCmd_LongMentionsRegistry(t *testing.T) {
	c := NewPullCmd()
	assert.Contains(t, c.Long, "registry")
}

func TestPullCmd_HasPlatformFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("platform")
	assert.NotNil(t, flag)
}

func TestPullCmd_HasConcurrencyFlag(t *testing.T) {
	cmd := NewPullCmd()
	flag := cmd.Flag("concurrency")
	require.NotNil(t, flag)
	assert.Equal(t, "j", flag.Shorthand)
	assert.Equal(t, "3", flag.DefValue)
}

// fakePutLayer returns a putLayer hook that calls fetch(0), copies the returned
// reader into an in-memory buffer (exercising progressReadCloser.Read so the bar
// advances), and records digest -> downloaded bytes in a mutex-guarded map.
func fakePutLayer(mu *sync.Mutex, got map[string][]byte) func(string, imgstore.FetchFn) error {
	return func(digest string, fetch imgstore.FetchFn) error {
		rc, err := fetch(0)
		if err != nil {
			return err
		}
		defer rc.Close()
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, rc); err != nil {
			return err
		}
		mu.Lock()
		got[digest] = buf.Bytes()
		mu.Unlock()
		return nil
	}
}

func TestPullLayers_DownloadsAllAndReportsCached(t *testing.T) {
	const owner, name = "o", "n"
	cached := makeDigest('a')
	dl1 := makeDigest('b')
	dl2 := makeDigest('c')

	blobBody := map[string][]byte{
		dl1: []byte("layer-one-bytes"),
		dl2: []byte("layer-two-longer-bytes"),
	}

	var fetched int32 // server-side counter of blob GETs
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetched, 1)
		digest := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name))
		body, ok := blobBody[digest]
		if !ok {
			w.WriteHeader(404)
			return
		}
		w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: cached, Size: 4},
		{Digest: dl1, Size: int64(len(blobBody[dl1]))},
		{Digest: dl2, Size: int64(len(blobBody[dl2]))},
	}
	hasLayer := func(d string) bool { return d == cached }

	var mu sync.Mutex
	got := map[string][]byte{}

	var buf bytes.Buffer
	err := pullLayers(&buf, rc, owner, name, layers, 3, hasLayer, fakePutLayer(&mu, got))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	_, cachedDownloaded := got[cached]
	assert.False(t, cachedDownloaded, "cached layer must not be downloaded")
	assert.Equal(t, blobBody[dl1], got[dl1], "dl1 bytes mismatch")
	assert.Equal(t, blobBody[dl2], got[dl2], "dl2 bytes mismatch")
	assert.Equal(t, int32(2), atomic.LoadInt32(&fetched), "exactly the two missing blobs should be fetched")

	out := buf.String()
	assert.Contains(t, out, fmt.Sprintf("  %s: cached\n", shortDigest(cached)))
	assert.Contains(t, out, fmt.Sprintf("  %s: downloading done\n", shortDigest(dl1)))
	assert.Contains(t, out, fmt.Sprintf("  %s: downloading done\n", shortDigest(dl2)))
}

func TestPullLayers_FirstErrorPropagates(t *testing.T) {
	const owner, name = "o", "n"
	good := makeDigest('1')
	bad := makeDigest('2')

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		digest := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name))
		if digest == bad {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: good, Size: 2},
		{Digest: bad, Size: 2},
	}
	hasLayer := func(string) bool { return false }

	var mu sync.Mutex
	got := map[string][]byte{}

	var buf bytes.Buffer
	err := pullLayers(&buf, rc, owner, name, layers, 2, hasLayer, fakePutLayer(&mu, got))
	require.Error(t, err)
}

func TestPullLayers_RespectsConcurrencyLimit(t *testing.T) {
	const owner, name = "o", "n"

	var inflight int32
	var maxInflight int32

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			old := atomic.LoadInt32(&maxInflight)
			if cur <= old || atomic.CompareAndSwapInt32(&maxInflight, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		w.Write([]byte("blob"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: makeDigest('1'), Size: 4},
		{Digest: makeDigest('2'), Size: 4},
		{Digest: makeDigest('3'), Size: 4},
		{Digest: makeDigest('4'), Size: 4},
		{Digest: makeDigest('5'), Size: 4},
	}
	hasLayer := func(string) bool { return false }

	var mu sync.Mutex
	got := map[string][]byte{}

	var buf bytes.Buffer
	err := pullLayers(&buf, rc, owner, name, layers, 2, hasLayer, fakePutLayer(&mu, got))
	require.NoError(t, err)
	assert.LessOrEqual(t, int(atomic.LoadInt32(&maxInflight)), 2, "observed in-flight downloads must not exceed concurrency limit")
	assert.GreaterOrEqual(t, int(atomic.LoadInt32(&maxInflight)), 2, "expected at least 2 concurrent downloads (parallelism not happening?)")
}
