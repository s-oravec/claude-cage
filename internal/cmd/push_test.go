package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/s-oravec/claude-cage/internal/registry"
)

// makeDigest builds a valid "sha256:<64hex>" digest from a single hex nibble c.
func makeDigest(c byte) string {
	return "sha256:" + strings.Repeat(string(c), 64)
}

// newTestClient builds a registry.Client pointed at srv (plain http test server).
func newTestClient(t *testing.T, srv *httptest.Server) *registry.Client {
	t.Helper()
	c, err := registry.NewClient(srv.URL[len("http://"):], registry.Options{Insecure: true})
	require.NoError(t, err)
	return c
}

func TestPushLayers_TransfersAllAndReportsExists(t *testing.T) {
	const owner, name = "o", "n"
	existing := makeDigest('a')
	missing1 := makeDigest('b')
	missing2 := makeDigest('c')

	var mu sync.Mutex
	uploaded := map[string]bool{}

	mux := http.NewServeMux()
	// HEAD: existing -> 200, others -> 404.
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			digest := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name))
			if digest == existing {
				w.WriteHeader(200)
				return
			}
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	})
	// POST init -> 202 with upload_url keyed by the digest from the body.
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"upload_id":"u1","upload_url":"/api/v1/repos/%s/%s/blobs/uploads/u1","expires_at":"x"}`, owner, name)
	})
	// PUT -> record the digest, 201.
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads/u1", owner, name), func(w http.ResponseWriter, r *http.Request) {
		digest := r.URL.Query().Get("digest")
		mu.Lock()
		uploaded[digest] = true
		mu.Unlock()
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: existing, Size: 4},
		{Digest: missing1, Size: 5},
		{Digest: missing2, Size: 6},
	}
	open := func(d string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("blobbody")), nil
	}

	var buf bytes.Buffer
	err := pushLayers(&buf, rc, owner, name, layers, 1024, 3, open)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, uploaded[missing1], "missing1 should have been uploaded")
	assert.True(t, uploaded[missing2], "missing2 should have been uploaded")
	assert.False(t, uploaded[existing], "existing layer must not be uploaded")

	out := buf.String()
	assert.Contains(t, out, fmt.Sprintf("  %s: exists\n", shortDigest(existing)))
	assert.Contains(t, out, fmt.Sprintf("  %s: uploading done\n", shortDigest(missing1)))
	assert.Contains(t, out, fmt.Sprintf("  %s: uploading done\n", shortDigest(missing2)))
}

func TestPushLayers_FirstErrorPropagates(t *testing.T) {
	const owner, name = "o", "n"
	bad := makeDigest('d')

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404) // everything missing
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"upload_id":"u1","upload_url":"/api/v1/repos/%s/%s/blobs/uploads/u1","expires_at":"x"}`, owner, name)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads/u1", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500) // upload fails
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: bad, Size: 5},
		{Digest: makeDigest('e'), Size: 5},
	}
	open := func(d string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("blob")), nil
	}

	var buf bytes.Buffer
	err := pushLayers(&buf, rc, owner, name, layers, 1024, 2, open)
	require.Error(t, err)
}

func TestPushLayers_RespectsConcurrencyLimit(t *testing.T) {
	const owner, name = "o", "n"

	var inflight int32
	var maxInflight int32

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404) // all missing
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprintf(w, `{"upload_id":"u1","upload_url":"/api/v1/repos/%s/%s/blobs/uploads/u1","expires_at":"x"}`, owner, name)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads/u1", owner, name), func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			old := atomic.LoadInt32(&maxInflight)
			if cur <= old || atomic.CompareAndSwapInt32(&maxInflight, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	rc := newTestClient(t, srv)

	layers := []manifest.Layer{
		{Digest: makeDigest('1'), Size: 5},
		{Digest: makeDigest('2'), Size: 5},
		{Digest: makeDigest('3'), Size: 5},
		{Digest: makeDigest('4'), Size: 5},
		{Digest: makeDigest('5'), Size: 5},
	}
	open := func(d string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("blob")), nil
	}

	var buf bytes.Buffer
	err := pushLayers(&buf, rc, owner, name, layers, 1024, 2, open)
	require.NoError(t, err)
	assert.LessOrEqual(t, int(atomic.LoadInt32(&maxInflight)), 2, "observed in-flight uploads must not exceed concurrency limit")
}

func TestPushCmd_Args(t *testing.T) {
	c := NewPushCmd()
	assert.NotNil(t, c.Flag("latest"))
}

func TestPrintPushResult(t *testing.T) {
	const (
		manifestDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		targetDigest   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	)
	tests := []struct {
		name     string
		tagLabel string
		arch     string
		res      *registry.PutManifestResult
		want     string
	}{
		{
			name:     "manifest kind, latest not updated",
			tagLabel: "acme/widget:v1",
			arch:     "amd64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "manifest",
				TagTargetDigest: targetDigest,
				LatestUpdated:   false,
			},
			want: "Pushed: sha256:111111111111 (amd64)\n" +
				"Tag acme/widget:v1 -> manifest sha256:222222222222\n",
		},
		{
			name:     "index kind, latest not updated",
			tagLabel: "acme/widget:v1",
			arch:     "arm64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "index",
				TagTargetDigest: targetDigest,
				LatestUpdated:   false,
			},
			want: "Pushed: sha256:111111111111 (arm64)\n" +
				"Tag acme/widget:v1 -> index sha256:222222222222 (auto-composed by server)\n",
		},
		{
			name:     "manifest kind, latest updated",
			tagLabel: "acme/widget:v1",
			arch:     "amd64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "manifest",
				TagTargetDigest: targetDigest,
				LatestUpdated:   true,
			},
			want: "Pushed: sha256:111111111111 (amd64)\n" +
				"Tag acme/widget:v1 -> manifest sha256:222222222222\n" +
				"Updated latest -> sha256:222222222222\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printPushResult(&buf, tt.tagLabel, tt.arch, tt.res)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
