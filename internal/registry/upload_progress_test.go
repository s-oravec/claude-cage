package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertMonotonic verifies the recorded cumulative values are non-decreasing.
func assertMonotonic(t *testing.T, vals []int64) {
	t.Helper()
	for i := 1; i < len(vals); i++ {
		assert.GreaterOrEqual(t, vals[i], vals[i-1], "progress must be non-decreasing")
	}
}

func TestUploadBlobSinglePUT_ReportsProgress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var vals []int64
	cb := func(uploaded int64) { vals = append(vals, uploaded) }

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), cb)
	require.NoError(t, err)

	require.NotEmpty(t, vals, "callback must be called at least once")
	assertMonotonic(t, vals)
	assert.Equal(t, int64(5), vals[len(vals)-1], "final cumulative value must equal blob size")
}

func TestUploadBlobMultipart_ReportsProgress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		w.Write([]byte(`{
			"upload_id":"u1",
			"part_size": 5,
			"part_count": 2,
			"parts_url_template":"/api/v1/repos/s/d/blobs/uploads/u1/parts/{n}/url",
			"expires_at":"x"
		}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1/parts/1/url", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"url":"/storage/put?part=1","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1/parts/2/url", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"url":"/storage/put?part=2","expires_at":"x"}`))
	})
	mux.HandleFunc("/storage/put", func(w http.ResponseWriter, r *http.Request) {
		part := 0
		fmt.Sscanf(r.URL.Query().Get("part"), "%d", &part)
		w.Header().Set("ETag", fmt.Sprintf("etag%d", part))
		w.WriteHeader(200)
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1/complete", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Parts []struct {
				N    int    `json:"n"`
				Etag string `json:"etag"`
			} `json:"parts"`
		}
		json.NewDecoder(r.Body).Decode(&p)
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	var vals []int64
	cb := func(uploaded int64) { vals = append(vals, uploaded) }

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobMultipart("s", "d", "sha256:abc", 10, strings.NewReader("abcdefghij"), cb)
	require.NoError(t, err)

	require.NotEmpty(t, vals, "callback must be called at least once")
	assertMonotonic(t, vals)
	assert.Equal(t, int64(10), vals[len(vals)-1], "final cumulative value must equal blob size")
}

func TestUploadBlob_NilProgressIsNoop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1","expires_at":"x"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), nil)
	require.NoError(t, err)
}
