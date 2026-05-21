# Token Refresh on 401 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep long-running registry pushes alive when the device-flow OAuth access token expires mid-upload, by refreshing the token (proactively before expiry, and reactively on a 401) and retrying the request.

**Architecture:** Capture and persist `refresh_token` + `expires_at` at login. Introduce a `TokenProvider` abstraction the registry client consults for every bearer-carrying request: it proactively refreshes when the token is near expiry and force-refreshes on a 401, then retries the request once. PAT tokens use a static provider that cannot refresh.

**Tech Stack:** Go, `net/http`, `gopkg.in/yaml.v3`, testify (`assert`/`require`), `net/http/httptest`.

**Design doc:** `docs/plans/2026-05-21-token-refresh-on-401-design.md`

---

## Conventions (read first)

- Tests use `github.com/stretchr/testify/assert` + `require`, and `net/http/httptest` servers. Match the style in `internal/oidcdevice/device_test.go` and `internal/registry/client_test.go`.
- Auth tests isolate the auth dir with `setRootForTest(t)` (see `internal/auth/authfile_test.go:12`).
- Run a single test: `go test ./internal/<pkg>/ -run TestName -v`
- Run a package: `go test ./internal/<pkg>/`
- Full build: `go build ./...`  Full tests: `go test ./...`
- ASCII only in all source/docs comments (no section sign, em dash, or arrow glyphs).

---

## Task 1: auth.Entry gains RefreshToken + ExpiresAt, add AddHostFull

**Files:**
- Modify: `internal/auth/authfile.go` (Entry struct ~13-18; AddHost ~102-114)
- Test: `internal/auth/authfile_test.go`

**Step 1: Write the failing test**

Add to `internal/auth/authfile_test.go`:

```go
func TestAuth_AddHostFull_RoundTrip(t *testing.T) {
	setRootForTest(t)
	exp := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	require.NoError(t, AddHostFull("cage-hub.io", "acc", "ref", "stiivo", exp))

	got, err := Load()
	require.NoError(t, err)
	e := got.Registries["cage-hub.io"]
	assert.Equal(t, "acc", e.Token)
	assert.Equal(t, "ref", e.RefreshToken)
	assert.Equal(t, "2026-05-21T12:00:00Z", e.ExpiresAt)
}

func TestAuth_AddHost_NoRefreshFields(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("h", "t", "u"))
	got, _ := Load()
	assert.Empty(t, got.Registries["h"].RefreshToken)
	assert.Empty(t, got.Registries["h"].ExpiresAt)
}
```

Add `"time"` to the test imports.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestAuth_AddHostFull -v`
Expected: FAIL (compile error - `AddHostFull` undefined, `RefreshToken`/`ExpiresAt` fields missing).

**Step 3: Write minimal implementation**

In `internal/auth/authfile.go`, extend `Entry`:

```go
type Entry struct {
	Token        string `yaml:"token"`
	RefreshToken string `yaml:"refresh_token,omitempty"`
	Username     string `yaml:"username,omitempty"`
	ObtainedAt   string `yaml:"obtained_at,omitempty"`
	ExpiresAt    string `yaml:"expires_at,omitempty"` // RFC3339; empty when unknown (e.g. PAT)
}
```

Replace `AddHost` and add `AddHostFull`:

```go
// AddHost stores a non-refreshable credential (e.g. a PAT) for a host.
func AddHost(host, token, username string) error {
	return AddHostFull(host, token, "", username, time.Time{})
}

// AddHostFull stores credentials for a host, replacing any existing entry.
// refreshToken and expiresAt may be empty/zero for non-refreshable tokens.
// ObtainedAt is set to the current time.
func AddHostFull(host, token, refreshToken, username string, expiresAt time.Time) error {
	a, err := Load()
	if err != nil {
		return err
	}
	e := Entry{
		Token:        token,
		RefreshToken: refreshToken,
		Username:     username,
		ObtainedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if !expiresAt.IsZero() {
		e.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}
	a.Registries[host] = e
	return Save(a)
}
```

**Step 4: Run tests**

Run: `go test ./internal/auth/`
Expected: PASS (existing tests still green).

**Step 5: Commit**

```bash
git add internal/auth/authfile.go internal/auth/authfile_test.go
git commit -m "feat(auth): store refresh_token and expires_at in auth.yaml"
```

---

## Task 2: oidcdevice returns a Token struct + Refresh grant

**Files:**
- Modify: `internal/oidcdevice/device.go` (PollToken ~77-116)
- Modify: `internal/cmd/login.go:102` (adapt to new return type - keep build green)
- Test: `internal/oidcdevice/device_test.go`

**Step 1: Write the failing tests**

In `internal/oidcdevice/device_test.go`, update the two PollToken success assertions to use the struct and add a Refresh test:

```go
// in TestPollToken_AuthPending_ThenSuccess: replace the success assertions
	tok, err := PollToken(srv.URL, "cage-cli", "dc", 10*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "ey...", tok.AccessToken)
	assert.Equal(t, time.Hour, tok.ExpiresIn)
	assert.GreaterOrEqual(t, call, 2)
```

In `TestPollToken_SlowDown_BacksOff` replace `_, err := PollToken(...)` usage is unchanged (still `_,`).

Add:

```go
func TestRefresh_PostsRefreshGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.PostForm.Get("grant_type"))
		assert.Equal(t, "cage-cli", r.PostForm.Get("client_id"))
		assert.Equal(t, "old-refresh", r.PostForm.Get("refresh_token"))
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-acc",
			"refresh_token": "new-ref",
			"expires_in":    300,
		})
	}))
	defer srv.Close()

	got, err := Refresh(srv.URL, "cage-cli", "old-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-acc", got.AccessToken)
	assert.Equal(t, "new-ref", got.RefreshToken)
	assert.Equal(t, 300*time.Second, got.ExpiresIn)
}

func TestRefresh_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	_, err := Refresh(srv.URL, "cage-cli", "bad")
	require.Error(t, err)
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/oidcdevice/ -v`
Expected: FAIL (compile error - `Token` type / `Refresh` undefined, `tok.AccessToken` invalid).

**Step 3: Write minimal implementation**

In `internal/oidcdevice/device.go`:

Add the struct + an internal decoder helper and change `PollToken` to return `(Token, error)`:

```go
// Token is an OAuth token response (access + optional refresh + lifetime).
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    time.Duration
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}
```

Change `PollToken` signature to `(Token, error)`; in the loop decode into `tokenResp`; on success return:

```go
		if raw.AccessToken != "" {
			return Token{
				AccessToken:  raw.AccessToken,
				RefreshToken: raw.RefreshToken,
				ExpiresIn:    time.Duration(raw.ExpiresIn) * time.Second,
			}, nil
		}
```

Replace every `return "", ...` in PollToken with `return Token{}, ...`.

Add `Refresh`:

```go
// Refresh exchanges a refresh_token for a new access (and refresh) token via
// the OAuth 2.0 refresh_token grant (RFC 6749 section 6).
func Refresh(tokenEndpoint, clientID, refreshToken string) (Token, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	resp, err := http.PostForm(tokenEndpoint, form)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()

	var raw tokenResp
	json.NewDecoder(resp.Body).Decode(&raw)
	if raw.AccessToken == "" {
		return Token{}, fmt.Errorf("token refresh failed: HTTP %d: %s", resp.StatusCode, raw.Error)
	}
	return Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn:    time.Duration(raw.ExpiresIn) * time.Second,
	}, nil
}
```

In `internal/cmd/login.go:102`, keep the build green by adapting the call site (full storage comes in Task 3):

```go
	tok, err := oidcdevice.PollToken(info.TokenEndpoint, info.ClientID, dev.DeviceCode, dev.Interval, dev.ExpiresIn)
	if err != nil {
		return err
	}
	if err := auth.AddHost(host, tok.AccessToken, ""); err != nil {
		return err
	}
```

**Step 4: Run tests**

Run: `go test ./internal/oidcdevice/ ./internal/cmd/` and `go build ./...`
Expected: PASS / build ok.

**Step 5: Commit**

```bash
git add internal/oidcdevice/device.go internal/oidcdevice/device_test.go internal/cmd/login.go
git commit -m "feat(oidcdevice): return Token struct from PollToken and add Refresh grant"
```

---

## Task 3: login persists refresh token + expiry

**Files:**
- Modify: `internal/cmd/login.go` (runLogin device-flow branch ~102-108)
- Test: existing `internal/cmd` login tests (adapt if any assert stored fields)

**Step 1: Write the failing test**

Add to the cmd login test file (find it: `grep -rl runLogin internal/cmd/*_test.go`; if none, create `internal/cmd/login_test.go` with package `cmd`). The device flow needs a fake registry + token server. Keep it focused on the storage wiring by testing the `--token-stdin` path is unchanged and add a small unit around the device branch only if a harness already exists. If wiring a full device-flow test is heavy, instead assert at the unit boundary already covered by Task 2 and verify storage via Task 1; document that the device-flow store path is exercised by the e2e push test.

Minimum: ensure existing login tests still pass after the change.

**Step 2: Implement**

In `internal/cmd/login.go`, device-flow branch:

```go
	tok, err := oidcdevice.PollToken(info.TokenEndpoint, info.ClientID, dev.DeviceCode, dev.Interval, dev.ExpiresIn)
	if err != nil {
		return err
	}
	var expiresAt time.Time
	if tok.ExpiresIn > 0 {
		expiresAt = time.Now().Add(tok.ExpiresIn)
	}
	if err := auth.AddHostFull(host, tok.AccessToken, tok.RefreshToken, "", expiresAt); err != nil {
		return err
	}
```

Add `"time"` to imports.

**Step 3: Run tests**

Run: `go test ./internal/cmd/` and `go build ./...`
Expected: PASS.

**Step 4: Commit**

```bash
git add internal/cmd/login.go internal/cmd/login_test.go 2>/dev/null; git add -A internal/cmd/
git commit -m "feat(login): persist refresh_token and computed expires_at"
```

---

## Task 4: tokensrc package - Static + Refreshing providers

**Files:**
- Create: `internal/tokensrc/tokensrc.go`
- Create: `internal/tokensrc/tokensrc_test.go`

This package imports `internal/auth` and `internal/oidcdevice` only (NOT registry), so there is no import cycle.

**Step 1: Write the failing tests**

Create `internal/tokensrc/tokensrc_test.go`:

```go
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
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/tokensrc/ -v`
Expected: FAIL (package/symbols do not exist).

**Step 3: Write minimal implementation**

Create `internal/tokensrc/tokensrc.go`:

```go
// Package tokensrc provides TokenProvider implementations for the registry
// client: a static (non-refreshable) source for PATs and a refreshing source
// backed by the OAuth refresh_token grant and persisted to auth.yaml.
package tokensrc

import (
	"fmt"
	"sync"
	"time"

	"github.com/s-oravec/claude-cage/internal/auth"
	"github.com/s-oravec/claude-cage/internal/oidcdevice"
)

// skew is how long before expiry we proactively refresh.
const skew = 60 * time.Second

// Static is a non-refreshable token source (e.g. a PAT).
type Static string

func (s Static) Token() (string, error) { return string(s), nil }
func (s Static) Refresh() (string, error) {
	return "", fmt.Errorf("session expired and this token cannot be refreshed; run `cage login` again")
}

// Refreshing is a token source backed by a stored refresh_token.
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
```

**Step 4: Run tests**

Run: `go test ./internal/tokensrc/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tokensrc/
git commit -m "feat(tokensrc): static and refreshing token providers"
```

---

## Task 5: registry TokenProvider interface + retry-on-401 in do()

**Files:**
- Modify: `internal/registry/client.go` (Options ~12-16; Client ~18-24; NewClient ~26-44; do ~68-86)
- Test: `internal/registry/client_test.go`

**Step 1: Write the failing tests**

Add to `internal/registry/client_test.go`:

```go
// fakeProvider counts Token/Refresh calls and returns scripted tokens.
type fakeProvider struct {
	tokens   []string // returned by successive Token() calls; last repeats
	refresh  string
	tCalls   int
	rCalls   int
	refErr   error
}

func (f *fakeProvider) Token() (string, error) {
	t := f.tokens[min(f.tCalls, len(f.tokens)-1)]
	f.tCalls++
	return t, nil
}
func (f *fakeProvider) Refresh() (string, error) {
	f.rCalls++
	if f.refErr != nil {
		return "", f.refErr
	}
	return f.refresh, nil
}
func min(a, b int) int { if a < b { return a }; return b }

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

	fp := &fakeProvider{tokens: []string{"old"}, refresh: "new"}
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
	fp := &fakeProvider{tokens: []string{"tok"}, refresh: "x"}
	c, _ := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	resp, err := c.do(http.MethodGet, "/x", nil, nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 0, fp.rCalls)
}
```

The existing `TestClient_BearerHeaderSent` uses `Options{Token: "ey..."}` and must keep working via the static fallback.

**Step 2: Run to verify it fails**

Run: `go test ./internal/registry/ -run TestClient -v`
Expected: FAIL (compile error - `Options.TokenProvider` missing).

**Step 3: Write minimal implementation**

In `internal/registry/client.go`:

```go
// TokenProvider supplies bearer tokens for registry requests. Token() returns a
// currently-valid token (refreshing proactively if near expiry); Refresh()
// forces a refresh and is called after a 401.
type TokenProvider interface {
	Token() (string, error)
	Refresh() (string, error)
}

// staticToken is the fallback provider for a fixed token (e.g. anonymous or a
// caller that passed Options.Token). It cannot refresh.
type staticToken string

func (s staticToken) Token() (string, error)   { return string(s), nil }
func (s staticToken) Refresh() (string, error) { return string(s), nil }
```

Update `Options`:

```go
type Options struct {
	Token         string
	TokenProvider TokenProvider // optional; overrides Token when set
	Insecure      bool
}
```

Update `Client` struct: replace `token string` with `provider TokenProvider`.

In `NewClient`, set the provider:

```go
	provider := opt.TokenProvider
	if provider == nil {
		provider = staticToken(opt.Token)
	}
	return &Client{
		baseURL:  scheme + "://" + host,
		host:     host,
		provider: provider,
		hc:       &http.Client{Transport: tr, Timeout: 60 * time.Second},
	}, nil
```

Add `SetTokenProvider` (used by push after it learns the endpoints from /auth/info):

```go
// SetTokenProvider swaps the token provider after construction.
func (c *Client) SetTokenProvider(p TokenProvider) { c.provider = p }
```

Refactor `do` into a single-shot helper + retry wrapper:

```go
func (c *Client) do(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	resp, err := c.doOnce(method, path, body, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if _, rerr := c.provider.Refresh(); rerr != nil {
			return nil, rerr
		}
		return c.doOnce(method, path, body, headers)
	}
	return resp, nil
}

func (c *Client) doOnce(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	var br io.Reader
	if body != nil {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.resolveURL(path), br)
	if err != nil {
		return nil, err
	}
	if tok, _ := c.provider.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.transport(req)
}
```

Note: the static fallback's `Refresh()` returns the same token, so a PAT 401 retries once then returns the real 401 to the caller (no infinite loop, no special-casing). `tokensrc.Static.Refresh()` returns an error, which `do` surfaces directly as the clean "session expired" message.

**Step 4: Run tests**

Run: `go test ./internal/registry/`
Expected: PASS (including existing tests).

**Step 5: Commit**

```bash
git add internal/registry/client.go internal/registry/client_test.go
git commit -m "feat(registry): TokenProvider with proactive token + refresh-and-retry on 401"
```

---

## Task 6: single-PUT phase 2 refresh-and-retry (seekable body)

**Files:**
- Modify: `internal/registry/upload_single.go` (phase 2 ~59-75)
- Test: `internal/registry/upload_single_test.go`

**Step 1: Write the failing test**

Add to `internal/registry/upload_single_test.go` a test where the init POST succeeds (202 with an upload_url pointing back at the test server) and the PUT returns 401 the first time, 201 the second; assert the body bytes received on the successful PUT equal the input. Use `*fakeProvider` from Task 5 (same package). Drive it through `UploadBlobSinglePUT` with a `bytes.NewReader([]byte("blobdata"))` (a Reader that is also a Seeker).

Sketch:

```go
func TestUploadSinglePUT_RefreshAndRetryOn401(t *testing.T) {
	var puts int
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost: // init
			w.WriteHeader(202)
			json.NewEncoder(w).Encode(map[string]any{
				"upload_id": "u1", "upload_url": "/upload/u1",
			})
		case r.Method == http.MethodPut:
			puts++
			if puts == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			lastBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(201)
		}
	}))
	defer srv.Close()

	fp := &fakeProvider{tokens: []string{"old"}, refresh: "new"}
	c, _ := NewClient(srv.URL[len("http://"):], Options{TokenProvider: fp, Insecure: true})
	err := c.UploadBlobSinglePUT("o", "n", "sha256:deadbeef", 8, bytes.NewReader([]byte("blobdata")))
	require.NoError(t, err)
	assert.Equal(t, 2, puts)
	assert.Equal(t, 1, fp.rCalls)
	assert.Equal(t, "blobdata", string(lastBody))
}
```

(Imports: `bytes`, `encoding/json`, `io`, `net/http`, `net/http/httptest`.)

**Step 2: Run to verify it fails**

Run: `go test ./internal/registry/ -run TestUploadSinglePUT_RefreshAndRetry -v`
Expected: FAIL (PUT returns 401, no retry, `parseError`).

**Step 3: Implement**

In `internal/registry/upload_single.go`, replace phase-2 send (lines ~59-75) with a refresh-aware helper. Capture the body's start offset so a seekable body can be replayed:

```go
	seeker, _ := body.(io.Seeker)
	resp2, err := c.putWithRefresh(c.resolveURL(uploadURL), body, seeker)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		return parseError(resp2)
	}
	return nil
}

// putWithRefresh PUTs body as octet-stream with the bearer token. On 401 it
// force-refreshes the token and, if the body is seekable, rewinds and retries
// once. A non-seekable body cannot be replayed, so the 401 is returned as-is
// (proactive refresh in Token() makes this path unlikely).
func (c *Client) putWithRefresh(url string, body io.Reader, seeker io.Seeker) (*http.Response, error) {
	resp, err := c.putOnce(url, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && seeker != nil {
		resp.Body.Close()
		if _, rerr := c.provider.Refresh(); rerr != nil {
			return nil, rerr
		}
		if _, serr := seeker.Seek(0, io.SeekStart); serr != nil {
			return nil, serr
		}
		return c.putOnce(url, body)
	}
	return resp, nil
}

func (c *Client) putOnce(url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if tok, _ := c.provider.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return c.transport(req)
}
```

Remove the now-dead manual `req` construction / `c.token` reference in phase 2.

**Step 4: Run tests**

Run: `go test ./internal/registry/`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/registry/upload_single.go internal/registry/upload_single_test.go
git commit -m "feat(registry): refresh-and-retry single-PUT on 401 for seekable bodies"
```

---

## Task 7: wire the refreshing provider into push

**Files:**
- Modify: `internal/cmd/push.go` (client construction ~56-84)
- Test: covered by existing push tests + e2e; add a focused wiring assertion only if a push unit harness exists.

**Step 1: Implement**

After `rc.AuthInfo()` succeeds and when the stored entry has a refresh token, attach a refreshing provider:

```go
	info, err := rc.AuthInfo()
	if err != nil {
		return err
	}

	// If we logged in via the device flow we have a refresh token; attach a
	// refreshing provider so a long push survives access-token expiry.
	if a, _ := auth.Load(); a != nil {
		if e, ok := a.Registries[ref.Host]; ok && e.RefreshToken != "" {
			rc.SetTokenProvider(tokensrc.NewRefreshing(ref.Host, info.ClientID, info.TokenEndpoint))
		}
	}
```

Add imports: `github.com/s-oravec/claude-cage/internal/tokensrc` (auth is already imported).

PAT pushes keep the static fallback built from `Options.Token` - no behavior change.

**Step 2: Run tests + build**

Run: `go build ./... && go test ./internal/cmd/`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/cmd/push.go
git commit -m "feat(push): attach refreshing token provider for device-flow logins"
```

---

## Task 8: full verification

**Step 1:** `go build ./...` - expect clean.
**Step 2:** `go test ./...` - expect all green.
**Step 3:** `go vet ./...` - expect clean.

Then use superpowers:verification-before-completion before claiming done, and
superpowers:finishing-a-development-branch to decide merge/PR.

Manual smoke (optional, needs a live cage-hub): with a short Keycloak access-token
lifetime, push a multi-part-sized image and confirm the push completes across a
token expiry instead of dying on 401.
