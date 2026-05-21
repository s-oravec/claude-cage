package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadBlobSinglePUT_RefreshAndRetryOn401(t *testing.T) {
	var puts int
	var lastBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/o/n/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/o/n/blobs/uploads/u1","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/o/n/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		puts++
		if puts == 1 {
			assert.Equal(t, "Bearer old", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "Bearer new", r.Header.Get("Authorization"))
		lastBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fp := &fakeProvider{cur: "old", next: "new"}
	c, _ := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	err := c.UploadBlobSinglePUT("o", "n", "sha256:deadbeef", 8, bytes.NewReader([]byte("blobdata")), nil)
	require.NoError(t, err)
	assert.Equal(t, 2, puts)
	assert.Equal(t, 1, fp.rCalls)
	assert.Equal(t, "blobdata", string(lastBody))
}

func TestUploadBlobSinglePUT_TwoPhase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "sha256:abc", body["digest"])
		assert.EqualValues(t, 5, body["size"])
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
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), nil)
	require.NoError(t, err)
}

// TestUploadBlobSinglePUT_UploadURLWithDigestQuery covers the real cage-hub
// response shape where upload_url already carries "?digest=<digest>". The
// client must not append a second "?digest=", which would produce a malformed
// query the server rejects (querystring/digest Invalid).
func TestUploadBlobSinglePUT_UploadURLWithDigestQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		// upload_url already includes ?digest=, exactly like the live server.
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1?digest=sha256:abc","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		// Exactly one digest value must arrive, equal to the blob digest.
		assert.Equal(t, []string{"sha256:abc"}, r.URL.Query()["digest"])
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), nil)
	require.NoError(t, err)
}

// TestUploadBlobSinglePUT_UploadURLWithUnrelatedQuery covers an upload_url that
// carries an unrelated query param but no digest. Setting the digest param must
// preserve the existing param AND add a single correct digest, proving the
// set-the-param approach neither drops the digest nor clobbers other params.
func TestUploadBlobSinglePUT_UploadURLWithUnrelatedQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		// upload_url has a query string but no digest param.
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1?foo=bar","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		// Existing param must survive, and exactly one digest must be added.
		assert.Equal(t, "bar", r.URL.Query().Get("foo"))
		assert.Equal(t, []string{"sha256:abc"}, r.URL.Query()["digest"])
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), nil)
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
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 7, strings.NewReader("payload"), nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), stored)
}
