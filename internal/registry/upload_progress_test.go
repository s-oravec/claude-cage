package registry

import (
	"bytes"
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
//
// Monotonicity holds ONLY absent a rewind. The cumulative ProgressFunc contract
// explicitly permits a reset to a lower value when a single-PUT upload is
// rewound on a 401 and retried: progressReader.Seek(0) resets the counter to 0
// and re-reports, producing a non-monotonic trajectory like [8, 0, 8]. Such
// retry paths must NOT use this helper - see
// TestUploadBlobSinglePUT_ReportsProgressAcrossRetry.
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

// TestUploadBlobSinglePUT_ReportsProgressAcrossRetry locks in the subtlest part
// of the cumulative ProgressFunc contract: on a 401 the single-PUT upload
// refreshes the token, rewinds the seekable body via Seek(0), and the
// progressReader resets its counter to 0 and re-reports. The cumulative
// trajectory is therefore NON-monotonic across the retry (e.g. [8, 0, 8]) - it
// drops to 0 then re-climbs. This test deliberately does NOT use assertMonotonic
// (which would correctly reject such a trajectory); instead it proves both that
// the upload still succeeds with a correct final total and that the reset/rewind
// re-report path actually executed.
func TestUploadBlobSinglePUT_ReportsProgressAcrossRetry(t *testing.T) {
	// Same server behavior as TestUploadBlobSinglePUT_RefreshAndRetryOn401:
	// first PUT -> 401 (forces token refresh + rewind), second PUT -> 201.
	var puts int
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
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var vals []int64
	cb := func(uploaded int64) { vals = append(vals, uploaded) }

	// Refreshing/seekable wiring: a seekable body and a provider that flips
	// "old" -> "new" on Refresh, so the 401 triggers the seek-retry path.
	fp := &fakeProvider{cur: "old", next: "new"}
	c, _ := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	err := c.UploadBlobSinglePUT("o", "n", "sha256:deadbeef", 8, bytes.NewReader([]byte("blobdata")), cb)
	require.NoError(t, err)

	// The retry must have happened.
	assert.Equal(t, 2, puts)
	assert.Equal(t, 1, fp.rCalls)

	require.NotEmpty(t, vals, "callback must be called at least once")
	// (a) Final cumulative value equals the blob size.
	assert.Equal(t, int64(8), vals[len(vals)-1], "final cumulative value must equal blob size")
	// (b) At least one later value is strictly less than an earlier value. This
	// proves the rewind reset (Seek(0) -> re-report of a lower total) executed,
	// i.e. the trajectory is non-monotonic exactly because of the 401 retry.
	assert.True(t, hasReset(vals), "expected a reset (a later value < an earlier value) proving the rewind re-report path ran: %v", vals)
}

// hasReset reports whether any value is strictly smaller than some value that
// preceded it - the signature of a rewind reset in the cumulative trajectory.
func hasReset(vals []int64) bool {
	var maxSeen int64
	for i, v := range vals {
		if i > 0 && v < maxSeen {
			return true
		}
		if v > maxSeen {
			maxSeen = v
		}
	}
	return false
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
