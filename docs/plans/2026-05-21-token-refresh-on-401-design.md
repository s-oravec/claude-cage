# Token refresh on 401 during long uploads - design

Date: 2026-05-21

## Problem

A long-running registry push (multipart upload of large layers) can outlive the
OAuth access token obtained via `cage login` (device flow). When the token
expires mid-upload, the next bearer-carrying request returns HTTP 401 and the
whole push dies. There is currently no retry and no token refresh.

Scope: only the device-flow token is handled. PAT tokens (`cgh_`, via
`--token-stdin`) are long-lived and cannot be refreshed; on 401 they get a clean
"re-login" error.

## Root cause

The bearer token is attached in two places:

- every call through `Client.do()` (`internal/registry/client.go:79-80`):
  upload init, GET per-part presigned URL, multipart complete, manifest PUT
- single-PUT phase 2 (`internal/registry/upload_single.go:64-65`)

In a multipart upload the data PUTs go to presigned storage URLs without a
bearer token (`internal/registry/upload_multipart.go:83`). Only the small
control calls carry the bearer, but there are many of them spread across the
whole upload (one GET per part). When the access token expires partway through,
one of these returns 401.

Preconditions that do not exist today:

- `oidcdevice.PollToken` returns only `access_token` and discards
  `refresh_token` + `expires_in` (`internal/oidcdevice/device.go:99-100`).
- `auth.Entry` stores only `Token` + `ObtainedAt` - no `RefreshToken`, no
  `ExpiresAt` (`internal/auth/authfile.go:14-18`).

## Strategy

Both mechanisms (the precondition is the same for either, so we do both):

- Proactive: before each bearer-carrying request, if the token has less than
  ~60s left, refresh it first. Avoids the 401 entirely and covers the streamed
  single-PUT case (refresh happens before the stream starts).
- Reactive (401 fallback): if a request still returns 401, force a refresh and
  retry the request once. Safety net for clock skew / early server-side
  invalidation.

## Design

### 1. Data model (`internal/auth/authfile.go`)

Add two backward-compatible fields to `Entry`:

```go
RefreshToken string `yaml:"refresh_token,omitempty"`
ExpiresAt    string `yaml:"expires_at,omitempty"` // RFC3339
```

PAT entries simply leave `RefreshToken` empty; that is how we tell them apart
from refreshable device-flow entries.

### 2. `internal/oidcdevice`

- `PollToken` returns a struct instead of a bare string:

  ```go
  type Token struct {
      AccessToken  string
      RefreshToken string
      ExpiresIn    time.Duration
  }
  ```

- New `Refresh(tokenEndpoint, clientID, refreshToken string) (Token, error)`:
  POST with `grant_type=refresh_token`.

### 3. Login (`internal/cmd/login.go`)

Persist the refresh token + computed `ExpiresAt` (via an extended
`auth.AddHost` / new save helper).

### 4. `TokenProvider` (mirrors `oauth2.TokenSource`)

```go
type TokenProvider interface {
    Token() (string, error)   // valid token; proactively refreshes if near expiry
    Refresh() (string, error) // forced refresh (called on 401)
}
```

Two implementations:

- refreshing - holds access/refresh/expiry plus `clientID` + `tokenEndpoint`
  (from `AuthInfo`); proactively refreshes when under the skew window,
  persists the new token to `auth.yaml`, guarded by a mutex.
- static - for PAT: `Token()` returns the token, `Refresh()` returns a clean
  error ("session expired, run `cage login <host>`").

### 5. Registry client integration

- `registry.Options` gains `TokenProvider TokenProvider`. `Token string` stays
  for backward compatibility: if `TokenProvider` is nil, the client builds a
  static provider from `Token`. No change for other `NewClient` callers.

- `Client.do()`:
  1. `tok, _ := provider.Token()` -> set `Bearer`
  2. send request
  3. on 401: `tok, err = provider.Refresh()`; if it succeeds, rebuild the
     request from the same `[]byte` body and send once more. Body is `[]byte`,
     so replay is trivial.

- single-PUT phase 2 (`upload_single.go:59`): same logic, but body is an
  `io.Reader`. Retry only if the body implements `io.Seeker` (push passes
  `*os.File` -> `Seek(0,0)` and resend). If not seekable, return a clear error -
  but proactive refresh before the stream starts covers this in practice.

- multipart: the bearer is carried only by the small `c.do()` calls (init,
  GET per-part URL, complete), so it is covered automatically. Data PUTs go to
  presigned URLs without a token and are left untouched.

Retry happens at most once per request; after a failed refresh the error
propagates (no infinite loop).

### 6. Error handling

A 401 with a non-refreshable token (PAT, or a refresh that failed) surfaces as
"session expired, run `cage login <host>`".

## Testing

- unit: `oidcdevice.Refresh` grant request/response
- unit: refreshing provider (proactive refresh under skew window, mutex)
- unit: retry-on-401 in `Client.do()` via `httptest` server (first 401, refresh,
  second request succeeds; and the no-refresh-token error path)
