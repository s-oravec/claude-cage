package tokensrc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-oravec/claude-cage/internal/auth"
)

func TestStatic_NeverRefreshes(t *testing.T) {
	s := Static("pat")
	tok, err := s.Token()
	require.NoError(t, err)
	assert.Equal(t, "pat", tok)
	_, err = s.Refresh()
	require.Error(t, err) // PAT cannot refresh
}

func TestRefreshing_ProactiveRefreshNearExpiry(t *testing.T) {
	d := t.TempDir()
	auth.SetDir(d)
	t.Cleanup(func() { auth.SetDir("") })

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fresh", "refresh_token": "ref2", "expires_in": 3600,
		})
	}))
	defer srv.Close()

	// Stored token already expired -> Token() must proactively refresh.
	require.NoError(t, auth.AddHostFull("h", "stale", "ref1", "u", time.Now().Add(-time.Minute)))

	p := NewRefreshing("h", "cage-cli", srv.URL)
	tok, err := p.Token()
	require.NoError(t, err)
	assert.Equal(t, "fresh", tok)
	assert.Equal(t, 1, calls)

	// Persisted back to auth.yaml.
	e, _ := auth.Load()
	assert.Equal(t, "fresh", e.Registries["h"].Token)
	assert.Equal(t, "ref2", e.Registries["h"].RefreshToken)

	// Second Token() within validity window does NOT refresh again.
	_, err = p.Token()
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRefreshing_ForceRefresh(t *testing.T) {
	d := t.TempDir()
	auth.SetDir(d)
	t.Cleanup(func() { auth.SetDir("") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "forced", "refresh_token": "ref2", "expires_in": 3600,
		})
	}))
	defer srv.Close()

	// Token still valid for an hour, but Refresh() must force a new one.
	require.NoError(t, auth.AddHostFull("h", "valid", "ref1", "u", time.Now().Add(time.Hour)))
	p := NewRefreshing("h", "cage-cli", srv.URL)
	tok, err := p.Refresh()
	require.NoError(t, err)
	assert.Equal(t, "forced", tok)
}
