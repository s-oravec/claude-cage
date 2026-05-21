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

func TestUploadBlobMultipart_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	var partsReceived []int

	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.URL.Query().Get("multipart"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "sha256:abc", body["digest"])
		assert.EqualValues(t, 10, body["size"])
		w.WriteHeader(202)
		w.Write([]byte(`{
			"upload_id":"u1",
			"part_size": 5,
			"part_count": 2,
			"parts_url_template":"/api/v1/repos/s/d/blobs/uploads/u1/parts/{n}/url",
			"expires_at":"2026-05-15T12:00:00Z"
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
		partsReceived = append(partsReceived, part)
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
		assert.Len(t, p.Parts, 2)
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobMultipart("s", "d", "sha256:abc", 10, strings.NewReader("abcdefghij"), nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{1, 2}, partsReceived)
}
