package registry

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadBlobSinglePUT_TwoPhase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1","expires_at":"2026-05-15T12:00:00Z"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "sha256:abc", r.URL.Query().Get("digest"))
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", strings.NewReader("layer"))
	require.NoError(t, err)
}

func TestUploadBlobSinglePUT_AbsoluteUploadURL(t *testing.T) {
	// Two servers: api and "storage"; init returns an absolute URL to storage.
	var stored []byte
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "sha256:abc", r.URL.Query().Get("digest"))
		stored, _ = io.ReadAll(r.Body)
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(201)
	}))
	defer storage.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/s/d/blobs/uploads", r.URL.Path)
		w.WriteHeader(202)
		// Absolute upload_url pointing at the storage server
		fmt.Fprintf(w, `{"upload_id":"u1","upload_url":"%s/store/u1","expires_at":"x"}`, storage.URL)
	}))
	defer api.Close()

	c, _ := NewClient(api.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", strings.NewReader("payload"))
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), stored)
}
