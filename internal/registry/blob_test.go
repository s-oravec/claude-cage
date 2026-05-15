package registry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadBlob_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		assert.Equal(t, "/api/v1/repos/s/d/blobs/sha256:abc", r.URL.Path)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	ok, err := c.HeadBlob("s", "d", "sha256:abc")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestHeadBlob_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	ok, err := c.HeadBlob("s", "d", "sha256:abc")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGetBlob_StreamsBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("layerbytes"))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	rc, err := c.GetBlob("s", "d", "sha256:abc", 0)
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, []byte("layerbytes"), b)
}

func TestGetBlob_RangeHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Range")
		w.WriteHeader(206)
		w.Write([]byte("partial"))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	rc, err := c.GetBlob("s", "d", "sha256:abc", 100)
	require.NoError(t, err)
	rc.Close()
	assert.Equal(t, "bytes=100-", got)
}
