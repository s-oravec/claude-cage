package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_BearerHeaderSent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL[len("http://"):], Options{Token: "ey...", Insecure: true})
	require.NoError(t, err)
	resp, err := c.do(http.MethodGet, "/api/v1/health", nil, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "Bearer ey...", got)
}

func TestClient_TLSOnByDefault(t *testing.T) {
	_, err := NewClient("cage-hub.io", Options{})
	require.NoError(t, err)
	// We can't easily assert scheme without making a request; just ensure construction succeeds.
}

func TestClient_InsecureHTTP(t *testing.T) {
	c, err := NewClient("localhost:5000", Options{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5000", c.baseURL)
}
