// Package tokensrc provides TokenProvider implementations for the registry
// client: a static (non-refreshable) source for PATs and a refreshing source
// backed by the OAuth refresh_token grant and persisted to auth.yaml.
package tokensrc

import (
	"fmt"
	"sync"
	"time"

	"github.com/s-oravec/cage/internal/auth"
	"github.com/s-oravec/cage/internal/oidcdevice"
)

// skew is how long before expiry we proactively refresh.
const skew = 60 * time.Second

// Static is a non-refreshable token source (e.g. a PAT).
type Static string

func (s Static) Token() (string, error) { return string(s), nil }

func (s Static) Refresh() (string, error) {
	return "", fmt.Errorf("session expired and this token cannot be refreshed; run `cage login` again")
}

// Refreshing is a token source backed by a stored refresh_token. It reads and
// writes the current tokens through the auth store and is safe for concurrent
// use.
type Refreshing struct {
	mu            sync.Mutex
	host          string
	clientID      string
	tokenEndpoint string
}

// NewRefreshing builds a Refreshing source. clientID and tokenEndpoint come
// from the registry's /auth/info; current tokens are read from auth.yaml.
func NewRefreshing(host, clientID, tokenEndpoint string) *Refreshing {
	return &Refreshing{host: host, clientID: clientID, tokenEndpoint: tokenEndpoint}
}

// Token returns a currently-valid access token, proactively refreshing if the
// stored token is within the skew window of expiry.
func (r *Refreshing) Token() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.load()
	if !ok {
		return "", fmt.Errorf("not logged in to %s", r.host)
	}
	if e.RefreshToken != "" && r.nearExpiry(e.ExpiresAt) {
		return r.doRefresh(e)
	}
	return e.Token, nil
}

// Refresh forces a token refresh regardless of expiry (called after a 401).
func (r *Refreshing) Refresh() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.load()
	if !ok || e.RefreshToken == "" {
		return "", fmt.Errorf("session expired and cannot be refreshed; run `cage login %s` again", r.host)
	}
	return r.doRefresh(e)
}

func (r *Refreshing) load() (auth.Entry, bool) {
	a, err := auth.Load()
	if err != nil {
		return auth.Entry{}, false
	}
	e, ok := a.Registries[r.host]
	return e, ok
}

// nearExpiry reports whether the token expires within the skew window. An empty
// or unparseable expiry is treated as "refresh now" so we never sit on a token
// of unknown age.
func (r *Refreshing) nearExpiry(expiresAt string) bool {
	if expiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	return time.Until(t) < skew
}

func (r *Refreshing) doRefresh(e auth.Entry) (string, error) {
	tok, err := oidcdevice.Refresh(r.tokenEndpoint, r.clientID, e.RefreshToken)
	if err != nil {
		return "", err
	}
	refresh := tok.RefreshToken
	if refresh == "" {
		refresh = e.RefreshToken // some servers do not rotate the refresh token
	}
	var expiresAt time.Time
	if tok.ExpiresIn > 0 {
		expiresAt = time.Now().Add(tok.ExpiresIn)
	}
	if err := auth.AddHostFull(r.host, tok.AccessToken, refresh, e.Username, expiresAt); err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}
