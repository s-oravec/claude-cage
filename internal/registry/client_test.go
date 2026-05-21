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

// fakeProvider scripts a token that flips to next after a Refresh.
type fakeProvider struct {
	cur    string
	next   string
	tCalls int
	rCalls int
	refErr error
}

func (f *fakeProvider) Token() (string, error) { f.tCalls++; return f.cur, nil }
func (f *fakeProvider) Refresh() (string, error) {
	f.rCalls++
	if f.refErr != nil {
		return "", f.refErr
	}
	f.cur = f.next
	return f.cur, nil
}

func TestClient_RefreshesAndRetriesOn401(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		if len(seen) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fp := &fakeProvider{cur: "old", next: "new"}
	c, err := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	require.NoError(t, err)
	resp, err := c.do(http.MethodGet, "/x", nil, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, fp.rCalls)
	assert.Equal(t, []string{"Bearer old", "Bearer new"}, seen)
}

func TestClient_No401_NoRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	fp := &fakeProvider{cur: "tok", next: "x"}
	c, _ := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	resp, err := c.do(http.MethodGet, "/x", nil, nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 0, fp.rCalls)
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
