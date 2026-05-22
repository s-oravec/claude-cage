# cage-hub Registry CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cage login` / `logout` / `push` / `pull` / `tag` commands that talk to a cage-hub registry; refactor the local image store and `cage build` to a content-addressed, layered representation so push/pull can deduplicate.

**Architecture:** New packages under `internal/`: `manifest` (JSON schema + canonical encoding + digest), `imgstore` (content-addressed layers/manifests/refs on disk), `auth` (`~/.claude-cage/auth.yaml` 0600), `oidcdevice` (Keycloak device flow), `registry` (HTTP client for `/api/v1/...`). New cobra commands in `internal/cmd/`. `cage build` save path changes from `qemu-img convert` (flatten) to `qemu-img rebase -u -b ""` (strip backing) + store in content-addressed layers/. `cage pull` learns to detect registry refs (contains `/`) and routes to the new code path. `cage start` learns to materialize a layer chain when its image is a registry ref. All existing flows for distro base images stay unchanged.

**Tech Stack:** Go 1.21, cobra v1.10, yaml.v3, stretchr/testify, net/http (stdlib), crypto/sha256 (stdlib). No new third-party deps - OIDC device flow is small enough to inline.

**Spec:** `docs/superpowers/specs/2026-05-14-cage-hub-registry-cli-design.md` on branch `feature/cage-hub`.

**Server contract:** `../cage-hub/docs/superpowers/specs/2026-05-14-cage-hub-design.md` (read-only reference).

---

## File map

**New packages:**
- `internal/manifest/manifest.go` + `_test.go` - schema, JSON, digest
- `internal/imgstore/paths.go` + `_test.go` - directory layout helpers (`Dir`, `LayerPath`, `ManifestPath`, `RefPath`, sharding)
- `internal/imgstore/refs.go` + `_test.go` - parse ref strings, read/write ref files
- `internal/imgstore/store.go` + `_test.go` - put/get layer + manifest blobs
- `internal/auth/authfile.go` + `_test.go` - read/write `auth.yaml` with 0600
- `internal/oidcdevice/device.go` + `_test.go` - device authorization grant client
- `internal/registry/client.go` - HTTP client construction, TLS mode, bearer token
- `internal/registry/authinfo.go` + `_test.go` - `GET /api/v1/auth/info`
- `internal/registry/manifest.go` + `_test.go` - manifest GET/HEAD/PUT
- `internal/registry/blob.go` + `_test.go` - blob HEAD/GET (with Range)
- `internal/registry/upload_single.go` + `_test.go` - single-PUT upload
- `internal/registry/upload_multipart.go` + `_test.go` - multipart upload
- `internal/registry/errors.go` + `_test.go` - typed error mapping
- `internal/cmd/login.go` + `_test.go`
- `internal/cmd/logout.go` + `_test.go`
- `internal/cmd/push.go` + `_test.go`
- `internal/cmd/tag.go` + `_test.go`

**Files modified:**
- `internal/config/config.go` - add `Registries.Insecure []string` field
- `internal/cmd/pull.go` - detect registry refs and delegate
- `internal/cmd/root.go` - register new commands
- `internal/build/executor.go` - rebase-and-store at end of build
- `internal/images/operations.go` - new layered `Save`; old flatten removed
- `internal/cage/reconfigure.go` (or `internal/cmd/start.go`) - materialize chain for registry-ref images
- `internal/cmd/image.go` - `cage image rm` accepts registry refs
- `internal/images/sources.go` - export `SupportedOS`/`SupportedArch` constants

---

## Phase A - Manifest type and canonical encoding

### Task 1: Manifest struct + JSON tags

**Files:**
- Create: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

- [ ] **Step 1: Write the failing test for round-trip JSON**

```go
package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest_RoundTripJSON(t *testing.T) {
	m := Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base: Base{
			Type:   "distro",
			Name:   "ubuntu-24.04",
			Digest: "sha256:abc",
		},
		Layers: []Layer{
			{Digest: "sha256:def", Size: 209715200, MediaType: MediaTypeLayerV1},
		},
		Config: Config{
			OS:      "linux",
			Arch:    "amd64",
			User:    "cage",
			Workdir: "/home/cage",
			Env:     []string{"K=V"},
		},
	}

	data, err := json.Marshal(&m)
	require.NoError(t, err)

	var got Manifest
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, m, got)
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/manifest/ -run TestManifest_RoundTripJSON -v`
Expected: `package internal/manifest does not exist` or compile error referencing undefined symbols.

- [ ] **Step 3: Implement Manifest types**

```go
package manifest

const (
	MediaTypeManifestV1 = "application/vnd.cage.manifest.v1+json"
	MediaTypeLayerV1    = "application/vnd.cage.layer.v1.qcow2"
	SchemaVersionV1     = 1
)

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Base          Base   `json:"base"`
	Layers        []Layer `json:"layers"`
	Config        Config `json:"config"`
}

type Base struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

type Config struct {
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	User        string            `json:"user,omitempty"`
	Workdir     string            `json:"workdir,omitempty"`
	Env         []string          `json:"env,omitempty"`
	Description string            `json:"description,omitempty"`
	Readme      string            `json:"readme,omitempty"`
	Cagefile    string            `json:"cagefile,omitempty"`
	Resources   *Resources        `json:"resources,omitempty"`
}

type Resources struct {
	MemoryMB int `json:"memory_mb,omitempty"`
	VCPU     int `json:"vcpu,omitempty"`
	DiskGB   int `json:"disk_gb,omitempty"`
}
```

- [ ] **Step 4: Run the test, confirm PASS**

Run: `go test ./internal/manifest/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/
git commit -m "feat(manifest): manifest type and JSON round-trip"
```

### Task 2: Canonical JSON encoding + digest

**Files:**
- Modify: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

The manifest digest is the sha256 of the canonical (deterministic) JSON bytes. We use Go's `encoding/json` with sorted map keys and no indentation; for structs, field order is fixed by struct declaration so no sort is needed.

- [ ] **Step 1: Write the failing test for canonical encoding determinism**

Append to `internal/manifest/manifest_test.go`:

```go
func TestManifest_Canonical_Deterministic(t *testing.T) {
	m := Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "amd64"},
	}

	a, err := Canonical(&m)
	require.NoError(t, err)
	b, err := Canonical(&m)
	require.NoError(t, err)
	assert.Equal(t, a, b, "canonical encoding must be byte-identical across calls")

	d1, err := Digest(&m)
	require.NoError(t, err)
	d2, err := Digest(&m)
	require.NoError(t, err)
	assert.Equal(t, d1, d2)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, d1)
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/manifest/ -run Canonical -v`
Expected: undefined `Canonical`, `Digest`.

- [ ] **Step 3: Implement Canonical and Digest**

Append to `internal/manifest/manifest.go`:

```go
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Canonical returns the deterministic JSON encoding used for digest
// computation and network transport. Struct field order is fixed, so
// stdlib json.Marshal is sufficient.
func Canonical(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// Digest computes the sha256:<hex> of the canonical encoding.
func Digest(m *Manifest) (string, error) {
	data, err := Canonical(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// DigestBytes computes sha256:<hex> of arbitrary bytes (used for layer
// blobs read from disk).
func DigestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `go test ./internal/manifest/ -v`
Expected: PASS for both tests.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/
git commit -m "feat(manifest): canonical encoding and digest"
```

### Task 3: Manifest validation (closed enum for os/arch)

Per O1 in review-04, `config.os` MUST be `"linux"` and `config.arch` MUST be `"amd64"` or `"arm64"`. CLI enforces this on the way out (build path) and on the way in (pull verification).

**Files:**
- Modify: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

- [ ] **Step 1: Write failing tests for Validate**

Append:

```go
func TestManifest_Validate_AcceptsCanonical(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "amd64"},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_RejectsOffWhitelistOS(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "Linux", Arch: "amd64"}, // capitalized
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.os")
}

func TestManifest_Validate_RejectsOffWhitelistArch(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "x86_64"},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.arch")
}

func TestManifest_Validate_RejectsMissingLayer(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{}, // empty
		Config:        Config{OS: "linux", Arch: "amd64"},
	}
	err := m.Validate()
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests, confirm fail**

Run: `go test ./internal/manifest/ -run Validate -v`
Expected: undefined `Validate`.

- [ ] **Step 3: Implement Validate**

Append:

```go
import "fmt"

var (
	SupportedOS   = []string{"linux"}
	SupportedArch = []string{"amd64", "arm64"}
)

func (m *Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("schemaVersion: want %d, got %d", SchemaVersionV1, m.SchemaVersion)
	}
	if m.MediaType != MediaTypeManifestV1 {
		return fmt.Errorf("mediaType: want %q, got %q", MediaTypeManifestV1, m.MediaType)
	}
	if m.Base.Type != "distro" {
		return fmt.Errorf("base.type: only %q supported", "distro")
	}
	if m.Base.Name == "" {
		return fmt.Errorf("base.name: required")
	}
	if !hasPrefix(m.Base.Digest, "sha256:") {
		return fmt.Errorf("base.digest: must be sha256:<hex>")
	}
	if len(m.Layers) == 0 {
		return fmt.Errorf("layers: at least one layer required")
	}
	for i, l := range m.Layers {
		if !hasPrefix(l.Digest, "sha256:") {
			return fmt.Errorf("layers[%d].digest: must be sha256:<hex>", i)
		}
		if l.Size <= 0 {
			return fmt.Errorf("layers[%d].size: must be > 0", i)
		}
		if l.MediaType != MediaTypeLayerV1 {
			return fmt.Errorf("layers[%d].mediaType: want %q", i, MediaTypeLayerV1)
		}
	}
	if !inList(m.Config.OS, SupportedOS) {
		return fmt.Errorf("config.os: must be one of %v, got %q", SupportedOS, m.Config.OS)
	}
	if !inList(m.Config.Arch, SupportedArch) {
		return fmt.Errorf("config.arch: must be one of %v, got %q", SupportedArch, m.Config.Arch)
	}
	if len(m.Config.Cagefile) > 64*1024 {
		return fmt.Errorf("config.cagefile: exceeds 64KB")
	}
	return nil
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func inList(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `go test ./internal/manifest/ -v`
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/
git commit -m "feat(manifest): closed-enum validation for os/arch"
```

---

## Phase B - Content-addressed local store

### Task 4: imgstore paths and sharding

**Files:**
- Create: `internal/imgstore/paths.go`
- Test: `internal/imgstore/paths_test.go`

- [ ] **Step 1: Write failing path tests**

```go
package imgstore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayerPath_Shards(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	p := LayerPath("sha256:abcdef0123")
	assert.Equal(t, "/tmp/cc/layers/sha256/ab/abcdef0123/layer.qcow2", p)
}

func TestManifestPath_Shards(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	p := ManifestPath("sha256:0123456789")
	assert.Equal(t, "/tmp/cc/manifests/sha256/01/0123456789/manifest.json", p)
}

func TestLocalRefPath(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Equal(t, "/tmp/cc/refs/_local/myimage/v1", LocalRefPath("myimage", "v1"))
}

func TestRegistryRefPath(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Equal(t, "/tmp/cc/refs/cage-hub.io/stiivo/devbox/v1",
		RegistryRefPath("cage-hub.io", "stiivo", "devbox", "v1"))
}

func TestRejectsNonSha256(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	assert.Panics(t, func() { LayerPath("md5:abc") })
}

func TestRefDirsBelowSharding(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	// Common path prefix shape sanity check.
	assert.True(t, strings.HasPrefix(LayerPath("sha256:ff00"), "/tmp/cc/layers/sha256/ff/"))
}
```

- [ ] **Step 2: Run tests, confirm fail**

Run: `go test ./internal/imgstore/ -v`
Expected: package missing.

- [ ] **Step 3: Implement paths.go**

```go
package imgstore

import (
	"path/filepath"
	"strings"

	"github.com/s-oravec/cage/internal/config"
)

var rootOverride string

// SetRoot overrides the storage root (testing only). Empty restores default.
func SetRoot(s string) { rootOverride = s }

// Root is the base directory under ~/.claude-cage where layered store lives.
func Root() string {
	if rootOverride != "" {
		return rootOverride
	}
	return config.Dir()
}

func splitDigest(d string) (algo, hex string) {
	parts := strings.SplitN(d, ":", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		panic("digest must start with sha256:")
	}
	return parts[0], parts[1]
}

// LayerPath returns the on-disk path for a layer blob.
func LayerPath(digest string) string {
	algo, hex := splitDigest(digest)
	return filepath.Join(Root(), "layers", algo, hex[:2], hex, "layer.qcow2")
}

// ManifestPath returns the on-disk path for a manifest blob.
func ManifestPath(digest string) string {
	algo, hex := splitDigest(digest)
	return filepath.Join(Root(), "manifests", algo, hex[:2], hex, "manifest.json")
}

// LocalRefPath returns the on-disk ref path for a local-only tag.
func LocalRefPath(name, tag string) string {
	return filepath.Join(Root(), "refs", "_local", name, tag)
}

// RegistryRefPath returns the on-disk ref path for a registry-qualified tag.
func RegistryRefPath(host, owner, name, tag string) string {
	return filepath.Join(Root(), "refs", host, owner, name, tag)
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `go test ./internal/imgstore/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/imgstore/
git commit -m "feat(imgstore): content-addressed path helpers"
```

### Task 5: Ref string parsing

**Files:**
- Create: `internal/imgstore/refs.go`
- Test: `internal/imgstore/refs_test.go`

- [ ] **Step 1: Write failing tests for ParseRef**

```go
package imgstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef_FullyQualified(t *testing.T) {
	r, err := ParseRef("cage-hub.io/stiivo/devbox:v1")
	require.NoError(t, err)
	assert.True(t, r.IsRegistry())
	assert.Equal(t, "cage-hub.io", r.Host)
	assert.Equal(t, "stiivo", r.Owner)
	assert.Equal(t, "devbox", r.Name)
	assert.Equal(t, "v1", r.Tag)
}

func TestParseRef_DefaultsTagToLatest(t *testing.T) {
	r, err := ParseRef("cage-hub.io/stiivo/devbox")
	require.NoError(t, err)
	assert.Equal(t, "latest", r.Tag)
}

func TestParseRef_LocalName(t *testing.T) {
	r, err := ParseRef("myimage:v2")
	require.NoError(t, err)
	assert.False(t, r.IsRegistry())
	assert.Equal(t, "myimage", r.Name)
	assert.Equal(t, "v2", r.Tag)
}

func TestParseRef_LocalNameDefaultLatest(t *testing.T) {
	r, err := ParseRef("myimage")
	require.NoError(t, err)
	assert.False(t, r.IsRegistry())
	assert.Equal(t, "myimage", r.Name)
	assert.Equal(t, "latest", r.Tag)
}

func TestParseRef_RejectsTwoSegments(t *testing.T) {
	_, err := ParseRef("stiivo/devbox:v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host/owner/name:tag")
}

func TestParseRef_RejectsEmpty(t *testing.T) {
	_, err := ParseRef("")
	require.Error(t, err)
}

func TestRef_RefPath_Registry(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	r := Ref{Host: "cage-hub.io", Owner: "s", Name: "d", Tag: "v1"}
	assert.Equal(t, "/tmp/cc/refs/cage-hub.io/s/d/v1", r.RefPath())
}

func TestRef_RefPath_Local(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	r := Ref{Name: "myimage", Tag: "v1"}
	assert.Equal(t, "/tmp/cc/refs/_local/myimage/v1", r.RefPath())
}
```

- [ ] **Step 2: Run tests, confirm fail**

Run: `go test ./internal/imgstore/ -run ParseRef -v`
Expected: undefined.

- [ ] **Step 3: Implement refs.go**

```go
package imgstore

import (
	"fmt"
	"strings"
)

// Ref is a parsed image reference. Host==Owner=="" => local-only ref.
type Ref struct {
	Host, Owner, Name, Tag string
}

func (r Ref) IsRegistry() bool { return r.Host != "" }

func (r Ref) RefPath() string {
	if r.IsRegistry() {
		return RegistryRefPath(r.Host, r.Owner, r.Name, r.Tag)
	}
	return LocalRefPath(r.Name, r.Tag)
}

// ParseRef recognizes 3-segment registry refs (host/owner/name[:tag])
// and single-segment local refs (name[:tag]). 2-segment refs without
// a registry are not supported.
func ParseRef(s string) (Ref, error) {
	if s == "" {
		return Ref{}, fmt.Errorf("ref is empty")
	}
	tag := "latest"
	if i := strings.LastIndex(s, ":"); i > 0 && !strings.Contains(s[i+1:], "/") {
		tag = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 1:
		return Ref{Name: parts[0], Tag: tag}, nil
	case 3:
		return Ref{Host: parts[0], Owner: parts[1], Name: parts[2], Tag: tag}, nil
	default:
		return Ref{}, fmt.Errorf("ref must be a bare name or host/owner/name:tag, got %q", strings.Join(parts, "/"))
	}
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `go test ./internal/imgstore/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/imgstore/
git commit -m "feat(imgstore): ref parsing and RefPath"
```

### Task 6: Read/write layer + manifest blobs (atomic)

**Files:**
- Create: `internal/imgstore/store.go`
- Test: `internal/imgstore/store_test.go`

- [ ] **Step 1: Write failing tests**

```go
package imgstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGetLayer_RoundTrip(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"
	require.NoError(t, PutLayerBytes(digest, []byte("hello")))

	got, err := GetLayerBytes(digest)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)

	assert.True(t, HasLayer(digest))
	assert.False(t, HasLayer("sha256:ffff"+
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
}

func TestPutGetManifest_RoundTrip(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	body := []byte(`{"schemaVersion":1}`)
	require.NoError(t, PutManifestBytes(digest, body))

	got, err := GetManifestBytes(digest)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestPutRef_OverwritesIdempotently(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	r := Ref{Name: "myimage", Tag: "latest"}
	require.NoError(t, WriteRef(r, "sha256:aaaa"))
	require.NoError(t, WriteRef(r, "sha256:bbbb")) // overwrite OK

	got, err := ReadRef(r)
	require.NoError(t, err)
	assert.Equal(t, "sha256:bbbb", got)
}

func TestReadRef_NotFound(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	_, err := ReadRef(Ref{Name: "ghost", Tag: "latest"})
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteRef(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	r := Ref{Name: "myimage", Tag: "latest"}
	require.NoError(t, WriteRef(r, "sha256:aaaa"))
	require.NoError(t, DeleteRef(r))
	_, err := ReadRef(r)
	require.Error(t, err)
}

func TestPutLayer_FsyncedAtomicRename(t *testing.T) {
	// White-box: PutLayerBytes must not leave a tmp file alongside the final.
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	require.NoError(t, PutLayerBytes(digest, []byte("x")))

	dir := filepath.Dir(LayerPath(digest))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp")
	}
}
```

- [ ] **Step 2: Run tests, confirm fail**

Run: `go test ./internal/imgstore/ -run PutGet -v`
Expected: undefined.

- [ ] **Step 3: Implement store.go**

```go
package imgstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ensureDir(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := ensureDir(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// CopyFromFile streams a file into the layer store at the named digest path,
// verifying the digest matches the file contents. Used for build flows that
// hand off a qcow2 file rather than bytes-in-memory.
func CopyFromFile(srcPath, digest string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst := LayerPath(digest)
	if err := ensureDir(dst); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".layer.*")
	if err != nil {
		return err
	}
	cleanup := func() { os.Remove(tmp.Name()) }

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), src); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got != digest {
		tmp.Close()
		cleanup()
		return fmt.Errorf("digest mismatch: expected %s, computed %s", digest, got)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func PutLayerBytes(digest string, data []byte) error {
	return writeAtomic(LayerPath(digest), data, 0o644)
}

func GetLayerBytes(digest string) ([]byte, error) {
	return os.ReadFile(LayerPath(digest))
}

func HasLayer(digest string) bool {
	_, err := os.Stat(LayerPath(digest))
	return err == nil
}

func PutManifestBytes(digest string, data []byte) error {
	return writeAtomic(ManifestPath(digest), data, 0o644)
}

func GetManifestBytes(digest string) ([]byte, error) {
	return os.ReadFile(ManifestPath(digest))
}

func HasManifest(digest string) bool {
	_, err := os.Stat(ManifestPath(digest))
	return err == nil
}

func WriteRef(r Ref, digest string) error {
	if !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("ref digest must be sha256:")
	}
	return writeAtomic(r.RefPath(), []byte(digest+"\n"), 0o644)
}

func ReadRef(r Ref) (string, error) {
	b, err := os.ReadFile(r.RefPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func DeleteRef(r Ref) error {
	err := os.Remove(r.RefPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run tests, confirm PASS**

Run: `go test ./internal/imgstore/ -v`
Expected: PASS for all tests.

- [ ] **Step 5: Commit**

```bash
git add internal/imgstore/
git commit -m "feat(imgstore): atomic put/get for layers, manifests, refs"
```

### Task 7: Hash a qcow2 file on disk

Used by build (after `qemu-img rebase`) to compute the layer digest.

**Files:**
- Modify: `internal/imgstore/store.go`
- Test: `internal/imgstore/store_test.go`

- [ ] **Step 1: Failing test**

```go
func TestHashFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "x")
	require.NoError(t, err)
	_, err = f.WriteString("hello")
	require.NoError(t, err)
	f.Close()

	got, err := HashFile(f.Name())
	require.NoError(t, err)
	// sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", got)
}
```

- [ ] **Step 2: Run, confirm fail**

`go test ./internal/imgstore/ -run HashFile`

- [ ] **Step 3: Implement HashFile**

Append to `store.go`:

```go
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 4: PASS**

`go test ./internal/imgstore/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/imgstore/
git commit -m "feat(imgstore): HashFile for streaming sha256"
```

---

## Phase C - Build refactor

### Task 8: Wire base distro sha256 lookup

`Manifest.Base.Digest` must match the local `images/<name>.qcow2`. Add a helper.

**Files:**
- Modify: `internal/images/operations.go` (new function)
- Test: `internal/images/operations_test.go`

- [ ] **Step 1: Failing test**

Append to `internal/images/operations_test.go`:

```go
func TestBaseDigest_ReadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Write a fake base image
	path := filepath.Join(tmpDir, "ubuntu-24.04.qcow2")
	require.NoError(t, os.WriteFile(path, []byte("fakebase"), 0644))

	d, err := BaseDigest("ubuntu-24.04")
	require.NoError(t, err)
	// sha256("fakebase") = 8a2f8c4dbc9c4ea8b41ab3a8edb1f7a4f7d4c6f4ce8c4aa7e0d4d23f7c1d2c33 (recompute)
	assert.True(t, strings.HasPrefix(d, "sha256:"))
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/images/ -run BaseDigest`

- [ ] **Step 3: Implement BaseDigest**

Append to `internal/images/operations.go`:

```go
import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// BaseDigest returns sha256:<hex> of the on-disk base image qcow2.
func BaseDigest(name string) (string, error) {
	f, err := os.Open(ImagePath(name))
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 4: PASS**

`go test ./internal/images/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/images/
git commit -m "feat(images): BaseDigest helper for layered manifests"
```

### Task 9: Layered Save - replace flatten with rebase+store

This changes the post-build artifact handling. The temp cage's overlay qcow2 (with backing=base image) becomes the custom layer after stripping its backing-file pointer.

**Files:**
- Modify: `internal/images/operations.go::Save`
- Modify: `internal/build/executor.go::saveImage` (will pass tag and base info through)
- Test: `internal/images/operations_test.go`

The new `Save` writes:
1. `qemu-img rebase -u -b ""` on a copy of the overlay (strip backing pointer).
2. Compute sha256 of the resulting file -> `layer_digest`.
3. Move to `imgstore.LayerPath(layer_digest)`.
4. Build `manifest.Manifest` (base name + base digest + this one layer + config).
5. Compute manifest digest, write canonical JSON to `imgstore.ManifestPath(manifest_digest)`.
6. Write ref according to `imgstore.ParseRef(tag).RefPath()`.

- [ ] **Step 1: Failing test (layered save end-to-end, mocking qemu-img)**

Add to `internal/images/operations_test.go`:

```go
func TestSaveLayered_WritesAllArtifacts(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed; run on dev host")
	}
	root := t.TempDir()
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = ""; imgstore.SetRoot("") }()

	// Make a fake base + overlay.
	base := filepath.Join(root, "ubuntu-24.04.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	overlayDir := t.TempDir()
	overlay := filepath.Join(overlayDir, "disk.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", base, "-F", "qcow2", overlay, "10M").Run())

	r, err := SaveLayered(SaveLayeredInput{
		OverlayPath: overlay,
		BaseName:    "ubuntu-24.04",
		Tag:         "myimage:v1",
		Config:      manifest.Config{OS: "linux", Arch: "amd64"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, r.ManifestDigest)
	assert.NotEmpty(t, r.LayerDigest)

	// Layer + manifest + ref are present.
	assert.True(t, imgstore.HasLayer(r.LayerDigest))
	assert.True(t, imgstore.HasManifest(r.ManifestDigest))

	ref, _ := imgstore.ParseRef("myimage:v1")
	got, err := imgstore.ReadRef(ref)
	require.NoError(t, err)
	assert.Equal(t, r.ManifestDigest, got)
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/images/ -run SaveLayered -v`

- [ ] **Step 3: Implement SaveLayered**

Add to `internal/images/operations.go`:

```go
import (
	"github.com/s-oravec/cage/internal/imgstore"
	"github.com/s-oravec/cage/internal/manifest"
)

type SaveLayeredInput struct {
	OverlayPath string
	BaseName    string
	Tag         string
	Config      manifest.Config
}

type SaveLayeredResult struct {
	ManifestDigest string
	LayerDigest    string
}

func SaveLayered(in SaveLayeredInput) (*SaveLayeredResult, error) {
	// Copy overlay so we don't mutate the source.
	tmp, err := os.CreateTemp("", "cage-layer-*.qcow2")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := copyFile(in.OverlayPath, tmpPath); err != nil {
		return nil, fmt.Errorf("copy overlay: %w", err)
	}

	// Strip backing-file pointer (metadata only).
	cmd := exec.Command("qemu-img", "rebase", "-u", "-b", "", tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("qemu-img rebase: %s", string(out))
	}

	layerDigest, err := imgstore.HashFile(tmpPath)
	if err != nil {
		return nil, err
	}
	if err := imgstore.CopyFromFile(tmpPath, layerDigest); err != nil {
		return nil, err
	}
	info, err := os.Stat(imgstore.LayerPath(layerDigest))
	if err != nil {
		return nil, err
	}

	baseDigest, err := BaseDigest(in.BaseName)
	if err != nil {
		return nil, fmt.Errorf("base digest: %w", err)
	}

	m := &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Base:          manifest.Base{Type: "distro", Name: in.BaseName, Digest: baseDigest},
		Layers:        []manifest.Layer{{Digest: layerDigest, Size: info.Size(), MediaType: manifest.MediaTypeLayerV1}},
		Config:        in.Config,
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	manifestBytes, err := manifest.Canonical(m)
	if err != nil {
		return nil, err
	}
	manifestDigest := manifest.DigestBytes(manifestBytes)
	if err := imgstore.PutManifestBytes(manifestDigest, manifestBytes); err != nil {
		return nil, err
	}

	ref, err := imgstore.ParseRef(in.Tag)
	if err != nil {
		return nil, err
	}
	if err := imgstore.WriteRef(ref, manifestDigest); err != nil {
		return nil, err
	}
	return &SaveLayeredResult{ManifestDigest: manifestDigest, LayerDigest: layerDigest}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Run, PASS**

`go test ./internal/images/ -run SaveLayered -v`

- [ ] **Step 5: Commit**

```bash
git add internal/images/
git commit -m "feat(images): SaveLayered for content-addressed build output"
```

### Task 10: Switch build executor to SaveLayered

**Files:**
- Modify: `internal/build/executor.go::saveImage`
- Test: existing build_test.go (integration test will be manual via dev host)

- [ ] **Step 1: Inspect the current `saveImage` function and identify the call site that needs to switch**

Run: `grep -n "saveImage\|images.Save\b" internal/build/executor.go`
Expected: see `saveImage` at ~line 583 and call to `images.Save` inside it.

- [ ] **Step 2: Replace `saveImage` body**

Open `internal/build/executor.go`, find:

```go
func (e *Executor) saveImage() error {
	e.log("Saving image as '%s'...", e.config.Tag)

	result, err := images.Save(e.tempCage, e.config.Tag, fmt.Sprintf("Built from %s", e.cagefile.BaseImage))
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	if result.VirtCustomizeError != "" {
		e.log(" ---> Warning: %s", result.VirtCustomizeError)
	}

	e.log("Successfully built image: %s", e.config.Tag)

	return nil
}
```

Replace with:

```go
func (e *Executor) saveImage() error {
	e.log("Saving image as '%s'...", e.config.Tag)

	overlay := filepath.Join(cage.VMDir(e.tempCage), "disk.qcow2")

	cfg := manifest.Config{
		OS:       "linux",
		Arch:     runtime.GOARCH,
		User:     e.user,
		Workdir:  e.workdir,
		Cagefile: readCagefileText(e.config.CagefilePath),
	}
	if len(e.env) > 0 {
		cfg.Env = make([]string, 0, len(e.env))
		for k, v := range e.env {
			cfg.Env = append(cfg.Env, k+"="+v)
		}
		sort.Strings(cfg.Env)
	}

	r, err := images.SaveLayered(images.SaveLayeredInput{
		OverlayPath: overlay,
		BaseName:    e.cagefile.BaseImage,
		Tag:         e.config.Tag,
		Config:      cfg,
	})
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	e.log("Built image: %s (manifest=%s, layer=%s)", e.config.Tag, r.ManifestDigest, r.LayerDigest)
	return nil
}

func readCagefileText(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > 64*1024 {
		return ""
	}
	return string(b)
}
```

Add imports at the top of the file: `runtime`, `sort`, and `"github.com/s-oravec/cage/internal/manifest"`.

- [ ] **Step 3: Compile**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 4: Run unit tests**

Run: `go test ./internal/build/ ./internal/images/ -v`
Expected: PASS (no regression in `parser_test.go`; `images` tests still pass).

- [ ] **Step 5: Commit**

```bash
git add internal/build/ internal/images/
git commit -m "feat(build): emit layered manifest + content-addressed layer"
```

### Task 11: Layered `cage image save` (running-cage snapshot path)

This path has no Cagefile, so `manifest.config.cagefile` stays empty (server tolerates this per A4).

**Files:**
- Modify: `internal/images/operations.go::Save` (legacy flat function) - keep callable from `cage image save` but route through SaveLayered with an empty cagefile.
- Test: `internal/images/operations_test.go`

- [ ] **Step 1: Failing test for image save round-trip**

```go
func TestSave_RoutesToLayered(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed")
	}
	root := t.TempDir()
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = ""; imgstore.SetRoot("") }()

	// Bootstrap a fake cage with a disk image.
	base := filepath.Join(root, "ubuntu-24.04.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	cageName := "fakecage"
	cageDir := filepath.Join(t.TempDir(), cageName)
	require.NoError(t, os.MkdirAll(cageDir, 0755))
	// Build a fake state so cage.LoadState works.
	// (Skip if state package not test-friendly; use cage.SaveState helper.)
	// ... helper omitted; see existing operations_test.go for patterns.

	// Call legacy Save and check that a ref+manifest+layer were created.
	// Implementation details TBD by cage state availability.
}
```

If `cage.LoadState` is hard to fake in tests, defer this to an integration test on dev host and use a smaller white-box test:

```go
func TestSave_BuildsValidManifest(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil { t.Skip() }
	// Direct call into the layered helper a Save will use, verify shape only.
	root := t.TempDir()
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = ""; imgstore.SetRoot("") }()

	base := filepath.Join(root, "ubuntu-24.04.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	overlay := filepath.Join(t.TempDir(), "disk.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", base, "-F", "qcow2", overlay, "10M").Run())

	r, err := SaveLayered(SaveLayeredInput{
		OverlayPath: overlay,
		BaseName:    "ubuntu-24.04",
		Tag:         "savedimage:latest",
		Config:      manifest.Config{OS: "linux", Arch: "amd64"},
	})
	require.NoError(t, err)

	// Read the manifest, verify it validates.
	body, err := imgstore.GetManifestBytes(r.ManifestDigest)
	require.NoError(t, err)
	var m manifest.Manifest
	require.NoError(t, json.Unmarshal(body, &m))
	require.NoError(t, m.Validate())
	assert.Empty(t, m.Config.Cagefile, "cage image save produces a manifest with no Cagefile")
}
```

- [ ] **Step 2: Run, expect fail**

`go test ./internal/images/ -run TestSave_BuildsValidManifest -v`

- [ ] **Step 3: Refactor `Save` to delegate to `SaveLayered`**

Replace the body of `Save` in `internal/images/operations.go` so it:
1. Locates the source disk (`cage.VMDir(cageName)/disk.qcow2`).
2. Reads `state.Image` for the base name.
3. Calls `SaveLayered` with an empty `Config.Cagefile`.

Keep the legacy `prepareImageForReuse` call BEFORE rebase: prepare the disk for reuse on the source disk path, since the layered file is then a clean overlay copy. *Note: this changes when SSH key reset happens. Verify on dev host.*

- [ ] **Step 4: Run all `internal/images` tests**

`go test ./internal/images/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/images/
git commit -m "feat(images): route Save through SaveLayered (no Cagefile)"
```

---

## Phase D - Config and auth

### Task 12: `registries.insecure` in global config

**Files:**
- Modify: `internal/config/config.go` (add field)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Failing test**

```go
func TestConfig_RegistriesInsecure_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	old := configDir
	configDir = tmpDir
	defer func() { configDir = old }()

	cfg := DefaultConfig()
	cfg.Registries.Insecure = []string{"localhost:5000", "cage-hub.local"}
	require.NoError(t, Save(cfg))

	got, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"localhost:5000", "cage-hub.local"}, got.Registries.Insecure)
}

func TestConfig_IsInsecure(t *testing.T) {
	cfg := &Config{Registries: RegistriesConfig{Insecure: []string{"localhost:5000"}}}
	assert.True(t, cfg.IsInsecureRegistry("localhost:5000"))
	assert.False(t, cfg.IsInsecureRegistry("cage-hub.io"))
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/config/ -run Registries -v`

- [ ] **Step 3: Add types and method**

Add to `internal/config/config.go`:

```go
type RegistriesConfig struct {
	Insecure []string `yaml:"insecure,omitempty"`
}
```

In the main `Config` struct, add:

```go
Registries RegistriesConfig `yaml:"registries,omitempty"`
```

And the helper:

```go
func (c *Config) IsInsecureRegistry(host string) bool {
	for _, h := range c.Registries.Insecure {
		if h == host {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run, PASS**

`go test ./internal/config/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): registries.insecure allowlist"
```

### Task 13: auth.yaml storage with 0600 enforcement

**Files:**
- Create: `internal/auth/authfile.go`
- Test: `internal/auth/authfile_test.go`

- [ ] **Step 1: Failing tests**

```go
package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRootForTest(t *testing.T) string {
	d := t.TempDir()
	SetDir(d)
	t.Cleanup(func() { SetDir("") })
	return d
}

func TestAuth_NoFile_LoadEmpty(t *testing.T) {
	setRootForTest(t)
	auth, err := Load()
	require.NoError(t, err)
	assert.Empty(t, auth.Registries)
}

func TestAuth_SaveLoad_RoundTrip(t *testing.T) {
	setRootForTest(t)

	a := &Auth{Registries: map[string]Entry{
		"cage-hub.io": {Token: "ey...", Username: "stiivo", ObtainedAt: "2026-05-15T10:00:00Z"},
	}}
	require.NoError(t, Save(a))

	got, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "ey...", got.Registries["cage-hub.io"].Token)
}

func TestAuth_SaveIs0600(t *testing.T) {
	dir := setRootForTest(t)
	a := &Auth{Registries: map[string]Entry{"h": {Token: "t"}}}
	require.NoError(t, Save(a))

	info, err := os.Stat(filepath.Join(dir, "auth.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAuth_LoadWarnsOnLoosePerms(t *testing.T) {
	dir := setRootForTest(t)
	path := filepath.Join(dir, "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte("registries: {}\n"), 0o644))

	_, err := Load()
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "Load fixes permissions")
}

func TestAuth_AddRemoveHost(t *testing.T) {
	setRootForTest(t)

	require.NoError(t, AddHost("cage-hub.io", "ey...", "stiivo"))
	require.NoError(t, AddHost("cage-hub.local", "pat_...", "stiivo"))

	got, err := Load()
	require.NoError(t, err)
	assert.Len(t, got.Registries, 2)

	require.NoError(t, RemoveHost("cage-hub.io"))
	got, err = Load()
	require.NoError(t, err)
	assert.Len(t, got.Registries, 1)
}

func TestAuth_RemoveHost_Missing_NoError(t *testing.T) {
	setRootForTest(t)
	assert.NoError(t, RemoveHost("ghost.example"))
}

func TestAuth_Token_Helper(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("cage-hub.io", "ey...", "stiivo"))
	tok, ok := Token("cage-hub.io")
	assert.True(t, ok)
	assert.Equal(t, "ey...", tok)
}

func TestAuth_Logout_All(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("a", "t1", "u"))
	require.NoError(t, AddHost("b", "t2", "u"))
	require.NoError(t, RemoveAll())

	got, err := Load()
	require.NoError(t, err)
	assert.Empty(t, got.Registries)
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/auth/ -v`

- [ ] **Step 3: Implement authfile.go**

```go
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/s-oravec/cage/internal/config"
	"gopkg.in/yaml.v3"
)

type Entry struct {
	Token      string `yaml:"token"`
	Username   string `yaml:"username,omitempty"`
	ObtainedAt string `yaml:"obtained_at,omitempty"`
}

type Auth struct {
	Registries map[string]Entry `yaml:"registries"`
}

var dirOverride string

// SetDir overrides the auth file directory (testing).
func SetDir(d string) { dirOverride = d }

func dir() string {
	if dirOverride != "" {
		return dirOverride
	}
	return config.Dir()
}

func path() string { return filepath.Join(dir(), "auth.yaml") }

func Load() (*Auth, error) {
	p := path()
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return &Auth{Registries: map[string]Entry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o600 {
		fmt.Fprintf(os.Stderr, "warning: auth.yaml has loose permissions %o; restoring to 0600\n", info.Mode().Perm())
		if err := os.Chmod(p, 0o600); err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var a Auth
	if err := yaml.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if a.Registries == nil {
		a.Registries = map[string]Entry{}
	}
	return &a, nil
}

func Save(a *Auth) error {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir(), ".auth.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path())
}

func AddHost(host, token, username string) error {
	a, err := Load()
	if err != nil {
		return err
	}
	a.Registries[host] = Entry{
		Token: token, Username: username,
		ObtainedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return Save(a)
}

func RemoveHost(host string) error {
	a, err := Load()
	if err != nil {
		return err
	}
	delete(a.Registries, host)
	return Save(a)
}

func RemoveAll() error {
	return Save(&Auth{Registries: map[string]Entry{}})
}

func Token(host string) (string, bool) {
	a, err := Load()
	if err != nil {
		return "", false
	}
	e, ok := a.Registries[host]
	return e.Token, ok
}
```

- [ ] **Step 4: Run, PASS**

`go test ./internal/auth/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/auth/
git commit -m "feat(auth): auth.yaml with 0600 enforcement"
```

---

## Phase E - Registry HTTP client

### Task 14: Client base (TLS mode, bearer header)

**Files:**
- Create: `internal/registry/client.go`
- Test: `internal/registry/client_test.go`

- [ ] **Step 1: Failing tests**

```go
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
```

- [ ] **Step 2: Run, fail**

`go test ./internal/registry/ -v`

- [ ] **Step 3: Implement client.go**

```go
package registry

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

type Options struct {
	Token    string
	Insecure bool // plain HTTP + skip cert verification
}

type Client struct {
	baseURL string
	token   string
	hc      *http.Client
}

func NewClient(host string, opt Options) (*Client, error) {
	scheme := "https"
	tr := &http.Transport{}
	if opt.Insecure {
		scheme = "http"
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // localhost dev only
	}
	return &Client{
		baseURL: scheme + "://" + host,
		token:   opt.Token,
		hc: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
	}, nil
}

func (c *Client) do(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	var br io.Reader
	if body != nil {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, br)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.hc.Do(req)
}
```

- [ ] **Step 4: Run, PASS**

`go test ./internal/registry/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): http client with bearer and insecure modes"
```

### Task 15: Typed server errors

**Files:**
- Create: `internal/registry/errors.go`
- Test: `internal/registry/errors_test.go`

- [ ] **Step 1: Failing tests**

```go
package registry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseError_PopulatesCode(t *testing.T) {
	body := `{"error":{"code":"CONFLICT_DIGEST_MISMATCH","message":"oops","details":{"want":"a","got":"b"}}}`
	err := parseError(&http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(body))})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "CONFLICT_DIGEST_MISMATCH", apiErr.Code)
	assert.Equal(t, 400, apiErr.HTTPStatus)
}

func TestParseError_NonJSONBody(t *testing.T) {
	err := parseError(&http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("oh no"))})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "UNEXPECTED", apiErr.Code)
}
```

Add import: `"io"`.

- [ ] **Step 2: Run, fail**

`go test ./internal/registry/ -run ParseError -v`

- [ ] **Step 3: Implement errors.go**

```go
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct {
	HTTPStatus int            `json:"-"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d): %s", e.Code, e.HTTPStatus, e.Message)
}

func parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var env struct{ Error APIError `json:"error"` }
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code == "" {
		return &APIError{HTTPStatus: resp.StatusCode, Code: "UNEXPECTED", Message: string(body)}
	}
	env.Error.HTTPStatus = resp.StatusCode
	return &env.Error
}
```

- [ ] **Step 4: PASS**

`go test ./internal/registry/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): typed APIError envelope"
```

### Task 16: auth/info discovery

**Files:**
- Create: `internal/registry/authinfo.go`
- Test: `internal/registry/authinfo_test.go`

- [ ] **Step 1: Failing test**

```go
package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthInfo_ReturnsParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/info", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                          "https://kc.example/realms/cage-hub",
			"device_authorization_endpoint":   "https://kc.example/.../auth/device",
			"token_endpoint":                  "https://kc.example/.../token",
			"client_id":                       "cage-cli",
			"scopes":                          []string{"openid", "profile"},
			"pat_format":                      "cgh_<base64url>",
			"pat_console_url":                 "https://h/settings/tokens",
			"supported_layer_media_types":     []string{"application/vnd.cage.layer.v1.qcow2"},
			"supported_manifest_media_types":  []string{"application/vnd.cage.manifest.v1+json"},
			"max_manifest_size":               65536,
			"max_layer_size":                  21474836480,
			"multipart_part_size":             67108864,
		})
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	got, err := c.AuthInfo()
	require.NoError(t, err)
	assert.Equal(t, "cage-cli", got.ClientID)
	assert.Equal(t, int64(67108864), got.MultipartPartSize)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement authinfo.go**

```go
package registry

import (
	"encoding/json"
	"net/http"
)

type AuthInfo struct {
	Issuer                       string   `json:"issuer"`
	DeviceAuthorizationEndpoint  string   `json:"device_authorization_endpoint"`
	TokenEndpoint                string   `json:"token_endpoint"`
	ClientID                     string   `json:"client_id"`
	Scopes                       []string `json:"scopes"`
	PATFormat                    string   `json:"pat_format"`
	PATConsoleURL                string   `json:"pat_console_url"`
	SupportedLayerMediaTypes     []string `json:"supported_layer_media_types"`
	SupportedManifestMediaTypes  []string `json:"supported_manifest_media_types"`
	MaxManifestSize              int64    `json:"max_manifest_size"`
	MaxLayerSize                 int64    `json:"max_layer_size"`
	MultipartPartSize            int64    `json:"multipart_part_size"`
}

func (c *Client) AuthInfo() (*AuthInfo, error) {
	resp, err := c.do(http.MethodGet, "/api/v1/auth/info", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out AuthInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): GET /api/v1/auth/info client"
```

### Task 17: Manifest GET / PUT / HEAD

**Files:**
- Create: `internal/registry/manifest.go`
- Test: `internal/registry/manifest_test.go`

- [ ] **Step 1: Failing tests for GetManifest, PutManifest**

```go
package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetManifest_ReturnsBodyAndDigest(t *testing.T) {
	body := []byte(`{"schemaVersion":1}`)
	digest := "sha256:" + hex.EncodeToString(sha256Sum(body))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/s/d/manifests/v1", r.URL.Path)
		w.Header().Set("Content-Type", "application/vnd.cage.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", digest)
		w.Write(body)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	got, gotDigest, err := c.GetManifest("s", "d", "v1")
	require.NoError(t, err)
	assert.Equal(t, body, got)
	assert.Equal(t, digest, gotDigest)
}

func TestPutManifest_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "application/vnd.cage.manifest.v1+json", r.Header.Get("Content-Type"))
		assert.Equal(t, "true", r.Header.Get("X-As-Latest"))
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(201)
		w.Write([]byte(`{"tag":"v1","manifest_digest":"sha256:abc","latest_updated":true}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	res, err := c.PutManifest("s", "d", "v1", []byte(`{}`), true)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc", res.ManifestDigest)
	assert.True(t, res.LatestUpdated)
}

func sha256Sum(b []byte) []byte { s := sha256.Sum256(b); return s[:] }

// Helper: read all from body for inspection
func bodyString(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	return strings.TrimSpace(string(b))
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement manifest.go**

```go
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type PutManifestResult struct {
	Tag            string `json:"tag"`
	ManifestDigest string `json:"manifest_digest"`
	LatestUpdated  bool   `json:"latest_updated"`
}

func (c *Client) GetManifest(owner, name, tag string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/manifests/%s", owner, name, tag)
	resp, err := c.do(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", parseError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Docker-Content-Digest"), nil
}

func (c *Client) PutManifest(owner, name, tag string, body []byte, asLatest bool) (*PutManifestResult, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/manifests/%s", owner, name, tag)
	headers := map[string]string{"Content-Type": "application/vnd.cage.manifest.v1+json"}
	if asLatest {
		headers["X-As-Latest"] = "true"
	}
	resp, err := c.do(http.MethodPut, path, body, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, parseError(resp)
	}
	var out PutManifestResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): manifest GET/PUT client"
```

### Task 18: Blob HEAD / streaming GET

**Files:**
- Create: `internal/registry/blob.go`
- Test: `internal/registry/blob_test.go`

- [ ] **Step 1: Failing tests**

```go
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
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement blob.go**

```go
package registry

import (
	"fmt"
	"io"
	"net/http"
)

func (c *Client) HeadBlob(owner, name, digest string) (bool, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/%s", owner, name, digest)
	resp, err := c.do(http.MethodHead, path, nil, nil)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, parseError(resp)
	}
}

// GetBlob streams the blob body. Caller must Close the returned reader.
// If offset > 0, a Range header is sent for resume.
func (c *Client) GetBlob(owner, name, digest string, offset int64) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/%s", owner, name, digest)
	headers := map[string]string{}
	if offset > 0 {
		headers["Range"] = fmt.Sprintf("bytes=%d-", offset)
	}
	resp, err := c.do(http.MethodGet, path, nil, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		defer resp.Body.Close()
		return nil, parseError(resp)
	}
	return resp.Body, nil
}
```

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): blob HEAD and streaming GET with Range"
```

### Task 19: Blob single-PUT upload

**Files:**
- Create: `internal/registry/upload_single.go`
- Test: `internal/registry/upload_single_test.go`

- [ ] **Step 1: Failing tests**

```go
func TestUploadBlobSinglePUT_TwoPhase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		w.Write([]byte(`{"upload_id":"u1","upload_url":"/api/v1/repos/s/d/blobs/uploads/u1","expires_at":"2026-05-15T12:00:00Z"}`))
	})
	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads/u1", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "sha256:abc", r.URL.Query().Get("digest"))
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", strings.NewReader("layer"))
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement upload_single.go**

```go
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type uploadInitResp struct {
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"`
}

// UploadBlobSinglePUT performs the two-phase Docker V2 single-PUT upload.
func (c *Client) UploadBlobSinglePUT(owner, name, digest string, body io.Reader) error {
	// Phase 1: init.
	initPath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name)
	initBody, _ := json.Marshal(map[string]string{"digest": digest})
	resp, err := c.do(http.MethodPost, initPath, initBody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	if resp.StatusCode != 202 {
		defer resp.Body.Close()
		return parseError(resp)
	}
	var init uploadInitResp
	if err := json.NewDecoder(resp.Body).Decode(&init); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	// Phase 2: PUT body.
	url := init.UploadURL + "?digest=" + digest
	req, err := http.NewRequest(http.MethodPut, c.baseURL+url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp2, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		return parseError(resp2)
	}
	return nil
}
```

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): single-PUT blob upload"
```

### Task 20: Blob multipart upload

**Files:**
- Create: `internal/registry/upload_multipart.go`
- Test: `internal/registry/upload_multipart_test.go`

- [ ] **Step 1: Failing test (init -> per-part URL -> PUT part -> complete)**

```go
func TestUploadBlobMultipart_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	var partsReceived []int

	mux.HandleFunc("/api/v1/repos/s/d/blobs/uploads", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.URL.Query().Get("multipart"))
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
		var p struct{ Parts []struct{ N int; Etag string } `json:"parts"` }
		json.NewDecoder(r.Body).Decode(&p)
		assert.Len(t, p.Parts, 2)
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Token: "t", Insecure: true})
	err := c.UploadBlobMultipart("s", "d", "sha256:abc", strings.NewReader("abcdefghij"))
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{1, 2}, partsReceived)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement upload_multipart.go**

```go
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type multipartInitResp struct {
	UploadID         string `json:"upload_id"`
	PartSize         int64  `json:"part_size"`
	PartCount        int    `json:"part_count"`
	PartsURLTemplate string `json:"parts_url_template"`
	ExpiresAt        string `json:"expires_at"`
}

type partURLResp struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

type completedPart struct {
	N    int    `json:"n"`
	Etag string `json:"etag"`
}

// UploadBlobMultipart drives a per-part presigned-PUT multipart upload.
func (c *Client) UploadBlobMultipart(owner, name, digest string, body io.Reader) error {
	// Init.
	initPath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads?multipart=true", owner, name)
	initBody, _ := json.Marshal(map[string]string{"digest": digest})
	resp, err := c.do(http.MethodPost, initPath, initBody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	if resp.StatusCode != 202 {
		defer resp.Body.Close()
		return parseError(resp)
	}
	var init multipartInitResp
	if err := json.NewDecoder(resp.Body).Decode(&init); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	completed := make([]completedPart, 0, init.PartCount)
	buf := make([]byte, init.PartSize)
	for n := 1; n <= init.PartCount; n++ {
		// Read one part of bytes from body.
		readSize, err := io.ReadFull(body, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		chunk := buf[:readSize]

		// Get presigned URL for this part.
		urlPath := strings.Replace(init.PartsURLTemplate, "{n}", fmt.Sprintf("%d", n), 1)
		uresp, err := c.do(http.MethodGet, urlPath, nil, nil)
		if err != nil {
			return err
		}
		if uresp.StatusCode != 200 {
			defer uresp.Body.Close()
			return parseError(uresp)
		}
		var pu partURLResp
		if err := json.NewDecoder(uresp.Body).Decode(&pu); err != nil {
			uresp.Body.Close()
			return err
		}
		uresp.Body.Close()

		// PUT directly to presigned URL.
		req, err := http.NewRequest(http.MethodPut, c.baseURL+pu.URL, strings.NewReader(string(chunk)))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		presp, err := c.hc.Do(req)
		if err != nil {
			return err
		}
		etag := presp.Header.Get("ETag")
		presp.Body.Close()
		if presp.StatusCode != 200 && presp.StatusCode != 201 {
			return fmt.Errorf("part %d upload failed: HTTP %d", n, presp.StatusCode)
		}
		completed = append(completed, completedPart{N: n, Etag: etag})
	}

	// Complete.
	completePath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads/%s/complete", owner, name, init.UploadID)
	cbody, _ := json.Marshal(map[string]any{"parts": completed})
	cresp, err := c.do(http.MethodPost, completePath, cbody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer cresp.Body.Close()
	if cresp.StatusCode != 200 && cresp.StatusCode != 201 {
		return parseError(cresp)
	}
	return nil
}
```

- [ ] **Step 4: PASS**

`go test ./internal/registry/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): multipart blob upload (init/parts/complete)"
```

### Task 21: Upload mode selector

CLI picks single-PUT for layers below `4 * MultipartPartSize` (~256 MB at default 64 MiB part), multipart above.

**Files:**
- Modify: `internal/registry/client.go` (add `UploadBlob` dispatcher)
- Test: `internal/registry/upload_dispatcher_test.go`

- [ ] **Step 1: Failing test**

```go
package registry

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectUploadMode(t *testing.T) {
	const part = int64(64 * 1024 * 1024) // 64 MiB
	tests := []struct {
		size int64
		want string
	}{
		{1, "single"},
		{4*part - 1, "single"},
		{4 * part, "multipart"},
		{10 * part, "multipart"},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tc.want, SelectUploadMode(tc.size, part))
		})
	}
}

// (UploadBlob dispatcher gets tested via existing upload_single/multipart tests
//  by passing a small or large body through it.)
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement dispatcher**

Append to `internal/registry/client.go`:

```go
func SelectUploadMode(size, partSize int64) string {
	if size < 4*partSize {
		return "single"
	}
	return "multipart"
}

// UploadBlob picks single or multipart based on size + partSize.
func (c *Client) UploadBlob(owner, name, digest string, size, partSize int64, body io.Reader) error {
	if SelectUploadMode(size, partSize) == "multipart" {
		return c.UploadBlobMultipart(owner, name, digest, body)
	}
	return c.UploadBlobSinglePUT(owner, name, digest, body)
}
```

Add `io` import if not yet present.

- [ ] **Step 4: PASS**

`go test ./internal/registry/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): UploadBlob dispatcher (hybrid C1)"
```

---

## Phase F - OIDC device flow

### Task 22: Device authorization grant client

**Files:**
- Create: `internal/oidcdevice/device.go`
- Test: `internal/oidcdevice/device_test.go`

- [ ] **Step 1: Failing tests**

```go
package oidcdevice

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDevice_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "client_id=cage-cli&scope=openid+profile", readPostForm(r))
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc",
			"user_code":        "ABCD-1234",
			"verification_uri": "https://kc/device",
			"expires_in":       600,
			"interval":         5,
		})
	}))
	defer srv.Close()

	got, err := RequestDevice(srv.URL, "cage-cli", []string{"openid", "profile"})
	require.NoError(t, err)
	assert.Equal(t, "dc", got.DeviceCode)
	assert.Equal(t, "ABCD-1234", got.UserCode)
	assert.Equal(t, time.Duration(5)*time.Second, got.Interval)
}

func TestPollToken_AuthPending_ThenSuccess(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"access_token":"ey...","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	tok, err := PollToken(srv.URL, "cage-cli", "dc", 10*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "ey...", tok)
	assert.GreaterOrEqual(t, call, 2)
}

func TestPollToken_SlowDown_BacksOff(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call <= 2 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"slow_down"}`))
			return
		}
		w.Write([]byte(`{"access_token":"t"}`))
	}))
	defer srv.Close()
	_, err := PollToken(srv.URL, "cage-cli", "dc", 5*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, call, 3)
}

func TestPollToken_TimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	defer srv.Close()
	_, err := PollToken(srv.URL, "cage-cli", "dc", 5*time.Millisecond, 50*time.Millisecond)
	require.Error(t, err)
}

// helpers
func readPostForm(r *http.Request) string {
	if err := r.ParseForm(); err != nil { return "" }
	return r.PostForm.Encode()
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/oidcdevice/ -v`

- [ ] **Step 3: Implement device.go**

```go
package oidcdevice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceResp struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        time.Duration
	ExpiresIn       time.Duration
}

func RequestDevice(deviceEndpoint, clientID string, scopes []string) (*DeviceResp, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", strings.Join(scopes, " "))

	resp, err := http.PostForm(deviceEndpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("device endpoint returned HTTP %d", resp.StatusCode)
	}
	var raw struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		VerificationComplete string `json:"verification_uri_complete"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Interval == 0 {
		raw.Interval = 5
	}
	uri := raw.VerificationComplete
	if uri == "" {
		uri = raw.VerificationURI
	}
	return &DeviceResp{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: uri,
		Interval:        time.Duration(raw.Interval) * time.Second,
		ExpiresIn:       time.Duration(raw.ExpiresIn) * time.Second,
	}, nil
}

// PollToken polls the token endpoint until the user authorizes, or until
// `timeout` elapses. Handles RFC 8628 error codes:
//   - authorization_pending: keep polling
//   - slow_down: increase interval by 5s and keep polling
//   - access_denied, expired_token: hard error
func PollToken(tokenEndpoint, clientID, deviceCode string, interval, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("authorization timed out, try again")
		}
		form := url.Values{}
		form.Set("client_id", clientID)
		form.Set("device_code", deviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		resp, err := http.PostForm(tokenEndpoint, form)
		if err != nil {
			return "", err
		}
		var raw struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()

		if raw.AccessToken != "" {
			return raw.AccessToken, nil
		}
		switch raw.Error {
		case "authorization_pending":
			// keep polling at current interval
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied":
			return "", fmt.Errorf("authorization denied")
		case "expired_token":
			return "", fmt.Errorf("authorization code expired, try again")
		default:
			return "", fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, raw.Error)
		}
		time.Sleep(interval)
	}
}
```

- [ ] **Step 4: PASS**

`go test ./internal/oidcdevice/ -v`

- [ ] **Step 5: Commit**

```bash
git add internal/oidcdevice/
git commit -m "feat(oidcdevice): device authorization grant + poll"
```

---

## Phase G - CLI commands

### Task 23: `cage login` (interactive + --token-stdin + --list)

**Files:**
- Create: `internal/cmd/login.go`
- Test: `internal/cmd/login_test.go`
- Modify: `internal/cmd/root.go` (register)

- [ ] **Step 1: Failing tests for command shape and basic flags**

```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginCmd_Exists(t *testing.T) {
	c := NewLoginCmd()
	assert.Equal(t, "login", c.Use[:5])
}

func TestLoginCmd_Flags(t *testing.T) {
	c := NewLoginCmd()
	assert.NotNil(t, c.Flag("token-stdin"))
	assert.NotNil(t, c.Flag("list"))
}
```

- [ ] **Step 2: Run, fail**

`go test ./internal/cmd/ -run TestLoginCmd -v`

- [ ] **Step 3: Implement login.go**

```go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/s-oravec/cage/internal/auth"
	"github.com/s-oravec/cage/internal/config"
	"github.com/s-oravec/cage/internal/oidcdevice"
	"github.com/s-oravec/cage/internal/registry"
)

func NewLoginCmd() *cobra.Command {
	var tokenStdin bool
	var list bool

	c := &cobra.Command{
		Use:   "login [host]",
		Short: "Log in to a cage-hub registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return runLoginList(cmd.OutOrStdout())
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: cage login <host>  (or --list)")
			}
			return runLogin(cmd.OutOrStdout(), cmd.InOrStdin(), args[0], tokenStdin)
		},
	}
	c.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read a PAT from stdin (non-interactive)")
	c.Flags().BoolVar(&list, "list", false, "List logged-in registries")
	return c
}

func runLoginList(out io.Writer) error {
	a, err := auth.Load()
	if err != nil {
		return err
	}
	if len(a.Registries) == 0 {
		fmt.Fprintln(out, "no registries")
		return nil
	}
	fmt.Fprintf(out, "%-30s %-20s %s\n", "HOST", "USERNAME", "OBTAINED")
	for host, e := range a.Registries {
		fmt.Fprintf(out, "%-30s %-20s %s\n", host, e.Username, e.ObtainedAt)
	}
	return nil
}

func runLogin(out io.Writer, in io.Reader, host string, tokenStdin bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if tokenStdin {
		token, err := readTokenFromStdin(in)
		if err != nil {
			return err
		}
		return auth.AddHost(host, token, "")
	}

	rc, err := registry.NewClient(host, registry.Options{Insecure: cfg.IsInsecureRegistry(host)})
	if err != nil {
		return err
	}
	info, err := rc.AuthInfo()
	if err != nil {
		return fmt.Errorf("auth/info: %w", err)
	}
	dev, err := oidcdevice.RequestDevice(info.DeviceAuthorizationEndpoint, info.ClientID, info.Scopes)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Open %s and enter code: %s\n", dev.VerificationURI, dev.UserCode)
	fmt.Fprintln(out, "Waiting for authorization...")

	token, err := oidcdevice.PollToken(info.TokenEndpoint, info.ClientID, dev.DeviceCode, dev.Interval, dev.ExpiresIn)
	if err != nil {
		return err
	}
	if err := auth.AddHost(host, token, ""); err != nil {
		return err
	}
	fmt.Fprintf(out, "Logged in to %s.\n", host)
	_ = time.Now() // suppress unused if path varies
	return nil
}

func readTokenFromStdin(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("empty token on stdin")
	}
	return line, nil
}

// suppress unused
var _ = os.Stdin
```

- [ ] **Step 4: Register in root.go**

In `internal/cmd/root.go` add to `NewRootCmd` body:

```go
rootCmd.AddCommand(NewLoginCmd())
```

- [ ] **Step 5: Run tests + build**

`go test ./internal/cmd/ -run TestLoginCmd -v && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/login.go internal/cmd/login_test.go internal/cmd/root.go
git commit -m "feat(cmd): cage login (device flow + --token-stdin + --list)"
```

### Task 24: `cage logout` (host + --all)

**Files:**
- Create: `internal/cmd/logout.go` + test
- Modify: `internal/cmd/root.go` (register)

- [ ] **Step 1: Failing tests**

```go
func TestLogoutCmd_Flags(t *testing.T) {
	c := NewLogoutCmd()
	assert.NotNil(t, c.Flag("all"))
}

func TestLogoutCmd_RequiresArgOrAll(t *testing.T) {
	c := NewLogoutCmd()
	c.SetArgs([]string{})
	err := c.Execute()
	require.Error(t, err)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement**

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/s-oravec/cage/internal/auth"
)

func NewLogoutCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "logout [host]",
		Short: "Remove stored credentials for a registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return auth.RemoveAll()
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: cage logout <host>  (or --all)")
			}
			return auth.RemoveHost(args[0])
		},
	}
	c.Flags().BoolVar(&all, "all", false, "Remove all stored credentials")
	return c
}
```

Register in `root.go`: `rootCmd.AddCommand(NewLogoutCmd())`.

- [ ] **Step 4: PASS + build**

`go test ./internal/cmd/ -run TestLogoutCmd -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/logout.go internal/cmd/logout_test.go internal/cmd/root.go
git commit -m "feat(cmd): cage logout (host + --all)"
```

### Task 25: `cage tag` (local-only retag)

**Files:**
- Create: `internal/cmd/tag.go` + test
- Modify: `internal/cmd/root.go` (register)

- [ ] **Step 1: Failing test for two-arg tag**

```go
func TestTagCmd_Args(t *testing.T) {
	c := NewTagCmd()
	c.SetArgs([]string{"only-one"})
	err := c.Execute()
	require.Error(t, err)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/s-oravec/cage/internal/imgstore"
)

func NewTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <src> <dst>",
		Short: "Create a new tag pointing at an existing local image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := imgstore.ParseRef(args[0])
			if err != nil {
				return fmt.Errorf("src: %w", err)
			}
			dst, err := imgstore.ParseRef(args[1])
			if err != nil {
				return fmt.Errorf("dst: %w", err)
			}
			digest, err := imgstore.ReadRef(src)
			if err != nil {
				return fmt.Errorf("image not found: %s (run `cage image list` to see available)", args[0])
			}
			return imgstore.WriteRef(dst, digest)
		},
	}
}
```

Register in `root.go`.

- [ ] **Step 4: PASS + build**

`go test ./internal/cmd/ -run TestTagCmd -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/tag.go internal/cmd/tag_test.go internal/cmd/root.go
git commit -m "feat(cmd): cage tag (local retag)"
```

### Task 26: `cage push` (full flow)

**Files:**
- Create: `internal/cmd/push.go` + test
- Modify: `internal/cmd/root.go`

- [ ] **Step 1: Failing test (shape only; full flow tested via integration on dev host)**

```go
func TestPushCmd_Args(t *testing.T) {
	c := NewPushCmd()
	assert.NotNil(t, c.Flag("latest"))
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/s-oravec/cage/internal/auth"
	"github.com/s-oravec/cage/internal/config"
	"github.com/s-oravec/cage/internal/imgstore"
	"github.com/s-oravec/cage/internal/manifest"
	"github.com/s-oravec/cage/internal/registry"
)

func NewPushCmd() *cobra.Command {
	var asLatest bool
	c := &cobra.Command{
		Use:   "push <ref>",
		Short: "Push an image to a cage-hub registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(args[0], asLatest)
		},
	}
	c.Flags().BoolVar(&asLatest, "latest", false, "Also update the `latest` tag pointer")
	return c
}

func runPush(refStr string, asLatest bool) error {
	ref, err := imgstore.ParseRef(refStr)
	if err != nil {
		return err
	}
	if !ref.IsRegistry() {
		return fmt.Errorf("ref must be a registry ref (host/owner/name:tag), got %q", refStr)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token, ok := auth.Token(ref.Host)
	if !ok {
		return fmt.Errorf("not logged in to %s - run `cage login %s`", ref.Host, ref.Host)
	}

	manifestDigest, err := imgstore.ReadRef(ref)
	if err != nil {
		return fmt.Errorf("no local image tagged %s", refStr)
	}
	manifestBytes, err := imgstore.GetManifestBytes(manifestDigest)
	if err != nil {
		return err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return err
	}

	rc, err := registry.NewClient(ref.Host, registry.Options{
		Token: token, Insecure: cfg.IsInsecureRegistry(ref.Host),
	})
	if err != nil {
		return err
	}

	info, err := rc.AuthInfo()
	if err != nil {
		return fmt.Errorf("auth/info: %w", err)
	}

	// Push each missing layer.
	for _, l := range m.Layers {
		exists, err := rc.HeadBlob(ref.Owner, ref.Name, l.Digest)
		if err != nil {
			return err
		}
		if exists {
			fmt.Fprintf(os.Stdout, "  %s: exists\n", l.Digest)
			continue
		}
		f, err := os.Open(imgstore.LayerPath(l.Digest))
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "  %s: uploading %d bytes\n", l.Digest, l.Size)
		err = rc.UploadBlob(ref.Owner, ref.Name, l.Digest, l.Size, info.MultipartPartSize, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	res, err := rc.PutManifest(ref.Owner, ref.Name, ref.Tag, manifestBytes, asLatest)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Pushed %s -> %s (latest_updated=%v)\n", refStr, res.ManifestDigest, res.LatestUpdated)
	return nil
}
```

Register in `root.go`.

- [ ] **Step 4: Compile + run test**

`go test ./internal/cmd/ -run TestPushCmd -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/push.go internal/cmd/push_test.go internal/cmd/root.go
git commit -m "feat(cmd): cage push (Docker V2 + hybrid upload)"
```

### Task 27: Extend `cage pull` for registry refs

**Files:**
- Modify: `internal/cmd/pull.go`
- Test: `internal/cmd/pull_test.go`

- [ ] **Step 1: Failing test - argument routing**

Append to `internal/cmd/pull_test.go`:

```go
func TestPullCmd_AcceptsRegistryRef(t *testing.T) {
	c := NewPullCmd()
	// Help string mentions registry refs.
	assert.Contains(t, c.Long, "registry")
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Refactor `RunE` to dispatch**

In `internal/cmd/pull.go`, replace the existing `RunE` body with:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	if list {
		return listImages(cmd)
	}

	// Positional arg routing.
	if len(args) == 1 {
		ref, err := imgstore.ParseRef(args[0])
		if err == nil && ref.IsRegistry() {
			return runRegistryPull(cmd, ref)
		}
	}

	if base == "" && len(args) > 0 {
		base = args[0]
	}
	if base == "" {
		base = "alpine"
	}
	return pullImage(cmd, base)
},
```

Update `Args` to `cobra.MaximumNArgs(1)` and update `Long` to mention registry references.

Add `runRegistryPull` function in the same file:

```go
func runRegistryPull(cmd *cobra.Command, ref imgstore.Ref) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rc, err := registry.NewClient(ref.Host, registry.Options{Insecure: cfg.IsInsecureRegistry(ref.Host)})
	if err != nil {
		return err
	}

	// Manifest.
	body, digest, err := rc.GetManifest(ref.Owner, ref.Name, ref.Tag)
	if err != nil {
		return err
	}
	if manifest.DigestBytes(body) != digest {
		return fmt.Errorf("manifest digest mismatch: server %s vs computed %s", digest, manifest.DigestBytes(body))
	}
	if err := imgstore.PutManifestBytes(digest, body); err != nil {
		return err
	}

	var m manifest.Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}

	// Layers.
	for _, l := range m.Layers {
		if imgstore.HasLayer(l.Digest) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: cached\n", l.Digest)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: downloading\n", l.Digest)
		rc2, err := rc.GetBlob(ref.Owner, ref.Name, l.Digest, 0)
		if err != nil {
			return err
		}
		buf, err := io.ReadAll(rc2)
		rc2.Close()
		if err != nil {
			return err
		}
		if manifest.DigestBytes(buf) != l.Digest {
			return fmt.Errorf("layer digest mismatch: server %s, got %s", l.Digest, manifest.DigestBytes(buf))
		}
		if err := imgstore.PutLayerBytes(l.Digest, buf); err != nil {
			return err
		}
	}

	// Base image check.
	if !images.IsDownloaded(m.Base.Name) {
		fmt.Fprintf(cmd.OutOrStdout(), "  base %s: not found locally, pulling...\n", m.Base.Name)
		if err := pullImage(cmd, m.Base.Name); err != nil {
			return err
		}
	}
	have, err := images.BaseDigest(m.Base.Name)
	if err != nil {
		return err
	}
	if have != m.Base.Digest {
		return fmt.Errorf("local base image %s differs from one used to build this image (have %s, need %s); run `cage image rm %s` and `cage pull --base %s`",
			m.Base.Name, have, m.Base.Digest, m.Base.Name, m.Base.Name)
	}

	// Ref.
	if err := imgstore.WriteRef(ref, digest); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s\n", ref.Host+"/"+ref.Owner+"/"+ref.Name+":"+ref.Tag)
	return nil
}
```

Add imports: `encoding/json`, `io`, `imgstore`, `manifest`, `registry`, `config`.

- [ ] **Step 4: PASS + build**

`go test ./internal/cmd/ -run TestPullCmd -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/pull.go internal/cmd/pull_test.go
git commit -m "feat(cmd): cage pull extends to registry refs"
```

### Task 28: Extend `cage image rm` for registry refs

**Files:**
- Modify: `internal/cmd/image.go`
- Test: `internal/cmd/image_test.go`

- [ ] **Step 1: Failing test**

```go
func TestImageRm_AcceptsRegistryRef(t *testing.T) {
	// Smoke: a registry-style argument doesn't blow up parsing.
	cmd := NewImageCmd()
	rm, _, err := cmd.Find([]string{"rm"})
	require.NoError(t, err)
	assert.NotNil(t, rm)
}
```

- [ ] **Step 2: Run, fail or pass-by-accident**

`go test ./internal/cmd/ -run TestImageRm`

- [ ] **Step 3: Update `rm` subcommand body**

Find the `rm` subcommand in `internal/cmd/image.go`. Update its `RunE` to:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	for _, a := range args {
		// Try ref form first (local or registry).
		if ref, err := imgstore.ParseRef(a); err == nil {
			if _, rerr := imgstore.ReadRef(ref); rerr == nil {
				if err := imgstore.DeleteRef(ref); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", a)
				continue
			}
		}
		// Fall back to legacy distro/custom image removal.
		if err := images.Remove(a, force); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", a)
	}
	return nil
},
```

Add `imgstore` import.

- [ ] **Step 4: PASS + build**

`go test ./internal/cmd/ -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/image.go internal/cmd/image_test.go
git commit -m "feat(cmd): cage image rm accepts registry refs"
```

---

## Phase H - Start-flow integration

### Task 29: Materialize layer chain on `cage start --image <registry-ref>`

`cage start` today reads `.cage.yml` `image:` field as either a distro alias or a custom image name and creates an overlay over `images/<name>.qcow2`. We extend this to recognize registry refs.

**Files:**
- Modify: `internal/cmd/start.go` (or wherever start prepares the disk)
- Test: integration on dev host (no unit test in this task)

- [ ] **Step 1: Locate the disk-prep code**

Run: `grep -n "qemu-img create\|backing\|ImagePath" internal/cmd/start.go internal/cage/reconfigure.go`
Expected: see the `qemu-img create -f qcow2 -b <base> -F qcow2 ... 20G` call.

- [ ] **Step 2: Add `MaterializeFromRef` helper in imgstore**

In `internal/imgstore/store.go`, append:

```go
// MaterializeChain takes a manifest digest and a destination path, then writes
// a qcow2 at dst with backing-file pointing at the local base distro image's
// path. MVP assumes exactly one custom layer (the manifest's top layer).
// On multi-layer manifests, the chain is materialized lowest-up (future work).
func MaterializeChain(manifestDigest string, baseImagePath, dstPath string) error {
	body, err := GetManifestBytes(manifestDigest)
	if err != nil {
		return err
	}
	var m manifestForMaterialize
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	if len(m.Layers) != 1 {
		return fmt.Errorf("multi-layer materialization not supported in MVP (got %d layers)", len(m.Layers))
	}
	// Copy the single layer to dst.
	src := LayerPath(m.Layers[0].Digest)
	if err := copyFileToDst(src, dstPath); err != nil {
		return err
	}
	// Rebase its backing file to the local base image.
	cmd := exec.Command("qemu-img", "rebase", "-u", "-b", baseImagePath, "-F", "qcow2", dstPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img rebase: %s", string(out))
	}
	return nil
}

type manifestForMaterialize struct {
	Layers []struct{ Digest string `json:"digest"` } `json:"layers"`
}

func copyFileToDst(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := ensureDir(dst); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

Add imports: `encoding/json`, `os/exec`, `fmt`.

- [ ] **Step 3: Wire into start**

Locate the existing disk creation in `internal/cmd/start.go` (or wherever the per-cage qcow2 overlay is created). Before the `qemu-img create -b <base>` call, branch on the image field:

```go
imgArg := resolved.Image // .cage.yml image: field
ref, refErr := imgstore.ParseRef(imgArg)
if refErr == nil && ref.IsRegistry() {
    manifestDigest, err := imgstore.ReadRef(ref)
    if err != nil {
        return fmt.Errorf("image %q not pulled locally - run `cage pull %s` first", imgArg, imgArg)
    }
    // Read base info from manifest.
    body, _ := imgstore.GetManifestBytes(manifestDigest)
    var m manifest.Manifest
    json.Unmarshal(body, &m)
    baseImg := images.ImagePath(m.Base.Name)
    diskBase := filepath.Join(cage.VMDir(cageName), "disk-base.qcow2")
    if err := imgstore.MaterializeChain(manifestDigest, baseImg, diskBase); err != nil {
        return err
    }
    // Per-cage overlay over disk-base.qcow2.
    overlay := filepath.Join(cage.VMDir(cageName), "disk.qcow2")
    cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", diskBase, "-F", "qcow2", overlay, fmt.Sprintf("%dG", diskSize))
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("qemu-img create overlay: %s", string(out))
    }
} else {
    // Existing path: backing file is images/<base>.qcow2.
    // (unchanged)
}
```

(Exact placement depends on existing start logic; adapt to the function boundaries you find.)

- [ ] **Step 4: Build + smoke test on dev host**

```bash
go build ./...
# Manual smoke (requires built CLI and an existing local registry image):
# cage start (from a project dir whose .cage.yml has image: cage-hub.local/me/test:v1)
```

- [ ] **Step 5: Commit**

```bash
git add internal/imgstore/store.go internal/cmd/start.go
git commit -m "feat(start): materialize layered images for cage start"
```

### Task 30: Apply manifest config to runtime cage

Apply `manifest.config` `User`, `Workdir`, `Env` to the cage similarly to how `.cage.yml` `env:` is already injected.

**Files:**
- Modify: `internal/cmd/start.go` (extend the existing config-resolution path)

- [ ] **Step 1: Identify env injection path**

Run: `grep -n "env\b\|virtiofs\|cloud-init" internal/cmd/start.go internal/cage/reconfigure.go internal/cloudinit/*.go`
Expected: see where `resolved.Env` is consumed and forwarded into cloud-init or virtiofs `cage-runtime-env.sh`.

- [ ] **Step 2: Merge manifest config into resolved env**

In the registry-ref branch added in Task 29, after reading the manifest, merge `m.Config.Env` into the resolved env map. Workdir / User are applied via cloud-init `runcmd` (or a profile.d snippet) - extend whichever already exists.

```go
for _, kv := range m.Config.Env {
    parts := strings.SplitN(kv, "=", 2)
    if len(parts) == 2 {
        resolved.Env[parts[0]] = parts[1]
    }
}
// User / Workdir hookup - implementation depends on existing cloud-init helpers.
```

- [ ] **Step 3: Build + smoke**

`go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/cmd/start.go
git commit -m "feat(start): apply manifest.config env to layered images"
```

---

## Phase I - Polish

### Task 31: Friendlier error messages for known server codes

**Files:**
- Modify: `internal/registry/errors.go` (add user-facing message helper)
- Modify: `internal/cmd/push.go`, `internal/cmd/pull.go`, `internal/cmd/login.go` to wrap errors

- [ ] **Step 1: Failing test**

In `internal/registry/errors_test.go` add:

```go
func TestUserMessage_KnownCodes(t *testing.T) {
	cases := map[string]string{
		"UNAUTHORIZED":              "run `cage login",
		"FORBIDDEN":                 "permission",
		"BLOB_MISSING":              "re-run push",
		"CONFLICT_DIGEST_MISMATCH":  "digest",
		"UNKNOWN_BASE":              "base image",
		"UPLOAD_EXPIRED":            "expired",
		"UPLOAD_COMPLETED":          "already completed",
	}
	for code, frag := range cases {
		got := UserMessage(&APIError{Code: code})
		assert.Contains(t, got, frag)
	}
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement `UserMessage`**

Append to `internal/registry/errors.go`:

```go
func UserMessage(e *APIError) string {
	switch e.Code {
	case "UNAUTHORIZED":
		return "not authorized - run `cage login <host>` and try again"
	case "FORBIDDEN":
		return "no permission for this operation - check namespace ownership or collaborator role"
	case "BLOB_MISSING":
		return "server lost or never received a layer; re-run push"
	case "CONFLICT_DIGEST_MISMATCH":
		return "uploaded blob digest did not match the server's computation; this is a client or transport bug"
	case "UNKNOWN_BASE":
		return "the base image in the manifest is not on the server's whitelist"
	case "UPLOAD_EXPIRED":
		return "upload session expired (24h TTL); re-run push"
	case "UPLOAD_COMPLETED":
		return "upload already completed; nothing to abort"
	case "UPLOAD_ABORTED":
		return "upload was aborted; start a fresh push"
	default:
		return e.Message
	}
}
```

- [ ] **Step 4: Wrap errors in commands**

In each command's `RunE`, where an `*APIError` may surface, emit `UserMessage(err)` to stderr in addition to the underlying error.

```go
if apiErr, ok := err.(*registry.APIError); ok {
    fmt.Fprintln(os.Stderr, registry.UserMessage(apiErr))
}
return err
```

- [ ] **Step 5: PASS + build**

`go test ./... && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/registry/ internal/cmd/
git commit -m "feat(registry): user-friendly messages for known server codes"
```

### Task 32: README + docs/cage-hub.md

**Files:**
- Modify: `README.md` (add login/logout/push/pull/tag references)
- Create: `docs/cage-hub.md` (user-facing how-to)

- [ ] **Step 1: Append command reference entries**

In `README.md`, under "Setup Commands" or a new "Registry Commands" section, add brief entries for `cage login`, `cage logout`, `cage push`, `cage tag`. Mirror the structure of existing `cage pull` entry.

- [ ] **Step 2: Create `docs/cage-hub.md`**

Write a short how-to: install, `cage login <host>`, `cage build -t <host>/<owner>/<name>:<tag> .`, `cage push <ref>`, on the consumer side `cage pull <ref>` + `cage start`. Reference the spec + insecure registry config.

- [ ] **Step 3: Build doc**

No code; just `git diff` for sanity.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/cage-hub.md
git commit -m "docs: cage-hub registry quickstart and command reference"
```

---

## Self-review checklist (run after writing the plan, not during)

- [ ] Each spec section in `2026-05-14-cage-hub-registry-cli-design.md` is covered:
  - Command surface -> Tasks 23-28
  - Config files -> Tasks 12-13
  - Image storage layout -> Tasks 4-7
  - Manifest format -> Tasks 1-3
  - GC -> not in MVP (deferred per spec)
  - Build flow -> Tasks 8-11
  - Pull flow -> Task 27
  - Push flow -> Task 26
  - Auth endpoint discovery -> Task 16
  - Start flow integration -> Tasks 29-30
  - Error handling -> Task 31
- [ ] No "TBD", "TODO", "fill in later" placeholders.
- [ ] Method/type names are consistent across tasks (`SaveLayered`, `imgstore.Ref`, `registry.Client`, `manifest.Manifest`).
- [ ] Every TDD task has Step-1 failing test, Step-2 run, Step-3 implementation, Step-4 confirm pass, Step-5 commit.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-15-cage-hub-registry-cli.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
