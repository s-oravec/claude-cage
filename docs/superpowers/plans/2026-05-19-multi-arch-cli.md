# Multi-Arch Images - CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the cage CLI arch-aware. The manifest's existing `config.arch` field is the source of truth for the image's architecture (no new top-level field). `cage build` resolves the base distro URL by host arch (or `--platform`). `cage push` does NOT compose indexes - the server handles auto-composition on the existing `PUT /manifests/:tag` endpoint and returns the resulting tag state in the response. `cage pull` dispatches on the response Content-Type (manifest vs index) and selects the matching arch from the index. A `--platform` flag overrides host-arch detection on build/push/pull.

**Spec:** `../cage-hub/docs/superpowers/specs/2026-05-19-multi-arch-images-design.md` (CLI work covers items 5-7 in the "Scope split for implementation" section; items 1-4 ship in cage-hub).

**Server state (as of 2026-05-19, branch `feature/multi-arch`):** Server has shipped. Key facts that shape this plan and DIVERGE from the spec:

1. **Architecture wire values are `amd64` / `arm64`** (Go GOARCH, Docker/OCI standard), NOT `x86_64` / `aarch64` from the spec. Mirrored in `packages/shared/src/dtos.ts:124` `ArchitectureSchema = z.enum(["amd64","arm64"])`.
2. **Server reads `parsed.config.arch`** from the manifest (`apps/api/src/routes/manifests.ts:107`), NOT a new top-level `architecture` field. No manifest wire-shape change is needed.
3. **Server auto-composes the index inside `PUT /manifests/:tag`** (manifests.ts:106-183). There is NO separate `PUT /indexes/:digest`, NO `PUT /tags/:tag`, NO `GET /tags/:tag`. The CLI does NOT do 5-case tag composition - it pushes the manifest and reads the resulting state from the response.
4. **`ManifestPutResponse`** returns `tag_target_kind` (`"manifest" | "index"`) and `tag_target_digest`, so the CLI knows what the tag now points at after a push.
5. **`X-As-Latest` header is preserved** - `cage push --latest` continues to use the header (Open Q #4 resolved).
6. **`GET /manifests/:reference`** dispatches by Content-Type (`application/vnd.cage.manifest.v1+json` or `.index.v1+json`) - this is the only piece needed for pull.
7. **Index body** uses `schemaVersion` (camelCase) and a top-level `mediaType` field - NOT `schema_version` (snake_case) from the spec.

**Tech Stack:** Go 1.21+, cobra, stdlib `runtime`, existing `internal/manifest`, `internal/registry`, `internal/images`, `internal/imgstore`.

---

## File map

**New files:**
- `internal/images/arch.go` + `_test.go` - `HostArchitecture()` + `SupportedArchitectures` (read-only constants)
- `internal/manifest/index.go` + `_test.go` - `IndexBody` type, `MediaTypeIndexV1` (read-only - used for pull dispatch only)

**Files modified:**
- `internal/images/base-aliases.json` - schema change: `url`+`sha256` -> `urls{arch->url}` + `sha256{arch->null|hex}` (keys: `amd64`, `arm64`)
- `internal/images/sources.go` - `BaseAliasEntry` and `ImageSource` adapt; `GetSource` takes an arch
- `internal/images/manager.go` - arch-keyed cache directory layout (`images/<name>/<arch>/...`)
- `internal/images/base_aliases_sync_test.go` - new pinned SHA-256 after schema change
- `internal/manifest/manifest.go` - `SupportedArch` whitelist already exists; ensure Validate stays correct (no top-level field added)
- `internal/registry/manifest.go` - extend `GetManifest` to return `(body, contentType, dockerDigest, err)` so pull can dispatch
- `internal/cmd/build.go` - `--platform` flag, plumb to executor
- `internal/build/executor.go` - resolve base by host arch (or `--platform`); set manifest `Config.Arch` from arg instead of hardcoded `runtime.GOARCH`
- `internal/images/operations.go` - `SaveLayered` accepts arch param instead of hardcoded `runtime.GOARCH`
- `internal/cmd/push.go` - `--platform` flag, after push read `tag_target_kind`/`tag_target_digest` from response and print summary
- `internal/cmd/pull.go` - `--platform` flag, Content-Type dispatch (manifest vs index)
- `internal/cmd/tag.go` - `inspect` subcommand (uses `GET /manifests/:tag`)
- `docs/cage-hub.md` - new arch-aware quickstart paragraph + `--platform` reference

**Files NOT touched:**
- Auth, login, logout, blob upload paths - arch is metadata on the manifest, not on layers.
- `cage start` materialization path - works off whatever manifest digest the ref resolves to; arch dispatch happens at pull time before the ref is written, not at start time.
- Local `internal/imgstore/` - no arch metadata stored locally; refs always point at a single-arch manifest digest after pull dispatch.
- `PutManifest`'s `X-As-Latest` header - stays as-is.

---

## Phase A - Schema foundations

### Task 1: Base-aliases schema migration (urls/sha256 maps, amd64/arm64 keys)

**Files:**
- Modify: `internal/images/base-aliases.json` (canonical, CLI-owned)
- Modify: `internal/images/sources.go`
- Modify: `internal/images/base_aliases_sync_test.go` (new pinned hash)

- [ ] **Step 1: Failing test for new schema shape**

```go
func TestBaseAliasEntry_ParsesArchMaps(t *testing.T) {
    var e BaseAliasEntry
    json.Unmarshal([]byte(`{
      "name":"ubuntu-24.04",
      "urls":{"amd64":"https://a","arm64":"https://b"},
      "sha256":{"amd64":null,"arm64":null},
      "description":"x"
    }`), &e)
    assert.Equal(t, "https://a", e.URLs["amd64"])
    assert.Equal(t, "https://b", e.URLs["arm64"])
}
```

- [ ] **Step 2: Rewrite `BaseAliasEntry`**

```go
type BaseAliasEntry struct {
    Name        string             `json:"name"`
    URLs        map[string]string  `json:"urls"`             // arch -> URL
    SHA256      map[string]*string `json:"sha256,omitempty"` // arch -> hex|null
    Description string             `json:"description"`
}
```

- [ ] **Step 3: Rewrite `base-aliases.json`**

Each of the 15+ entries gets `urls.amd64` (existing URL) plus `urls.arm64`. For Alpine, Ubuntu, Debian, Rocky, Alma, Fedora, openSUSE, CentOS Stream - locate the matching arm64 cloud-image URL by following the same naming convention used by the amd64 URL (`...-amd64.img` -> `...-arm64.img`, `x86_64.qcow2` -> `aarch64.qcow2`, etc.). Aliases without an arm64 cloud-image set `urls.arm64: null` and the build flow errors with a hint at use time.

NOTE: The qcow2 URLs themselves still contain `x86_64` / `aarch64` substrings (that's the upstream naming convention) - only the JSON KEYS use `amd64` / `arm64` to match wire/Go conventions.

- [ ] **Step 4: Update `GetSource` signature**

```go
func GetSource(name, arch string) (*ImageSource, error)
```

Callers in `images/manager.go` (Download/Setup) and `cmd/pull.go` (base-image pull) need `arch`. Default to `HostArchitecture()` at the call site.

- [ ] **Step 5: Recompute pinned SHA-256**

```bash
sha256sum internal/images/base-aliases.json
# paste into expectedBaseAliasesSHA256
```

Open a sister-PR in cage-hub: `cp internal/images/base-aliases.json ../cage-hub/apps/api/src/config/base-aliases.json` and update `EXPECTED_BASE_ALIASES_SHA256` there too. Sync test in both repos must pass.

- [ ] **Step 6: Run sync test + all images tests; commit**

```
go test ./internal/images/...
```

### Task 2: HostArchitecture helper

**Files:**
- Create: `internal/images/arch.go`
- Test: `internal/images/arch_test.go`

- [ ] **Step 1: Test**

```go
func TestHostArchitecture_ReturnsGOARCH(t *testing.T) {
    assert.Equal(t, runtime.GOARCH, HostArchitecture())
}

func TestSupportedArchitectures_HasAmd64AndArm64(t *testing.T) {
    assert.Contains(t, SupportedArchitectures, "amd64")
    assert.Contains(t, SupportedArchitectures, "arm64")
}
```

- [ ] **Step 2: Implement**

```go
package images

import "runtime"

// HostArchitecture returns the Go GOARCH of the current host (e.g. "amd64", "arm64").
// Matches the wire values used by cage-hub (see packages/shared/src/dtos.ts ArchitectureSchema)
// and the existing manifest.Config.Arch values.
func HostArchitecture() string { return runtime.GOARCH }

// SupportedArchitectures is the closed whitelist enforced by both CLI and server.
var SupportedArchitectures = []string{"amd64", "arm64"}
```

NOTE: No `MapGOARCH` helper. Server already uses GOARCH-style values, so no mapping is needed. The whole point of having "wire value != GOARCH" went away with the server decision.

- [ ] **Step 3: Commit**

### Task 3: Manifest arch already correct - verify only

**Files:**
- Verify: `internal/manifest/manifest.go` (no edit expected)
- Verify: `internal/manifest/manifest_test.go`

The manifest's existing `Config.Arch` (line 45) IS the source of truth for the image architecture. Server reads it directly. The existing `SupportedArch = []string{"amd64", "arm64"}` whitelist (line 90) and the existing `Validate` check (line 129) are already correct.

- [ ] **Step 1: Confirm tests cover the validation**

```go
func TestManifest_RejectsUnknownArch(t *testing.T)   // already exists? add if missing
func TestManifest_RejectsMissingArch(t *testing.T)   // already exists? add if missing
```

If missing, add. No code changes to `manifest.go` expected.

- [ ] **Step 2: Confirm + commit (or no-op)**

NOTE: NO top-level `Architecture` field is added. Spec mentions one; server doesn't use it; CLI doesn't need it. Skip.

### Task 4: IndexBody type + media type (read-only)

**Files:**
- Create: `internal/manifest/index.go`
- Test: `internal/manifest/index_test.go`

CLI only ever DESERIALIZES an index (during pull). It never composes one. Keep the type minimal.

- [ ] **Step 1: Failing tests for IndexBody round-trip**

```go
func TestIndexBody_RoundTrip(t *testing.T) {
    raw := []byte(`{"schemaVersion":1,"mediaType":"application/vnd.cage.index.v1+json","manifests":[
        {"digest":"sha256:a","platform":{"architecture":"amd64"}},
        {"digest":"sha256:b","platform":{"architecture":"arm64"}}
    ]}`)
    var idx IndexBody
    require.NoError(t, json.Unmarshal(raw, &idx))
    assert.Equal(t, 1, idx.SchemaVersion)
    assert.Equal(t, MediaTypeIndexV1, idx.MediaType)
    assert.Len(t, idx.Manifests, 2)
    assert.Equal(t, "amd64", idx.Manifests[0].Platform.Architecture)
}
```

- [ ] **Step 2: Implement**

```go
package manifest

// MediaTypeIndexV1 is the wire Content-Type returned by GET /manifests/:reference
// when the reference resolves to a multi-arch index.
const MediaTypeIndexV1 = "application/vnd.cage.index.v1+json"

// IndexBody is the deserialized form of a multi-arch index. CLI never composes one
// (server does that automatically in PUT /manifests/:tag); we only deserialize on
// pull. Field names match cage-hub packages/shared/src/dtos.ts ManifestIndexBodySchema.
type IndexBody struct {
    SchemaVersion int          `json:"schemaVersion"`
    MediaType     string       `json:"mediaType"`
    Manifests     []IndexEntry `json:"manifests"`
}

type IndexEntry struct {
    Digest   string   `json:"digest"`
    Platform Platform `json:"platform"`
}

type Platform struct {
    Architecture string `json:"architecture"` // "amd64" | "arm64"
}
```

NOTE on naming: spec showed `schema_version` (snake_case) but server shipped `schemaVersion` (camelCase). Match server.

NOTE: No `Validate()`, `CanonicalIndex`, `DigestIndex` - those would be needed only if CLI built indexes. It does not.

- [ ] **Step 3: Commit**

---

## Phase B - Build flow arch-aware

### Task 5: `--platform` flag on build/push/pull commands

**Files:**
- Modify: `internal/cmd/build.go`, `internal/cmd/push.go`, `internal/cmd/pull.go`

- [ ] **Step 1: Add flag with empty default = host arch**

```go
var platform string
c.Flags().StringVar(&platform, "platform", "",
    "Target architecture (amd64|arm64). Defaults to host architecture.")
```

- [ ] **Step 2: Resolve at runtime**

```go
arch := platform
if arch == "" { arch = images.HostArchitecture() }
if !slices.Contains(images.SupportedArchitectures, arch) {
    return fmt.Errorf("--platform: must be one of %v, got %q",
        images.SupportedArchitectures, arch)
}
```

- [ ] **Step 3: Plumb `arch` into existing function signatures**

`runBuild(arch)`, `runPush(arch)`, `runPull(arch)`. The flag itself is opt-in; absence means "host".

- [ ] **Step 4: Commit**

### Task 6: Build flow uses arch-aware base resolution

**Files:**
- Modify: `internal/build/executor.go`
- Modify: `internal/images/manager.go` (Download/Setup signature adopts arch)
- Modify: `internal/images/operations.go:SaveLayered` (Config.Arch from param)

- [ ] **Step 1: Failing test for base resolution by arch**

```go
func TestExecutor_ResolveBase_MissingArchURL(t *testing.T) {
    // base alias whose urls.arm64 == nil
    // Resolve(name, "arm64") -> error mentioning "no arm64 cloud-image"
}
```

- [ ] **Step 2: Lookup base URL by arch in executor**

```go
src, err := images.GetSource(e.cagefile.BaseImage, arch)
if err != nil { return err }
if src.URL == "" {
    return fmt.Errorf("base %q has no %s cloud-image; pick a different base or edit base-aliases.json",
        e.cagefile.BaseImage, arch)
}
```

`ImageSource.URL` becomes the single URL for the resolved arch (set by `GetSource` from the map).

- [ ] **Step 3: Arch-keyed cache directory**

Update `images.ImagePath` to take arch:

```go
func ImagePath(name, arch string) string {
    name = ResolveAlias(name)
    return filepath.Join(Dir(), name, arch, "image.qcow2")
}
```

Migration concern: existing caches live at `images/<name>.qcow2` (flat). For the MVP, treat the new layout as fresh and re-download on first arch-aware use. Document this in `docs/cage-hub.md` as a known migration cost.

- [ ] **Step 4: SaveLayered sets manifest.Config.Arch from param**

Today at `internal/images/operations.go:139` and `internal/build/executor.go:593` the code hardcodes `Arch: goruntime.GOARCH`. Replace with the `arch` argument plumbed from `runBuild`.

```go
m := &manifest.Manifest{
    // ...
    Config: manifest.Config{ OS: "linux", Arch: arch, /* ... */ },
}
```

- [ ] **Step 5: Tests for end-to-end build path**

In `internal/build/executor_test.go` add a fixture that asserts the saved manifest has `Config.Arch == arch` for the test's host (or with `--platform`).

- [ ] **Step 6: Commit**

---

## Phase C - Push: surface server's auto-composition

> Originally this phase included 4 tasks for CLI-side index composition (GetTag, PutTag, PutIndex, GetIndex + 5-case state machine). Server shipped auto-composition inside `PUT /manifests/:tag` instead, so all of that is unnecessary. Phase C is now a single small task.

### Task 7: Print server-reported tag state after push

**Files:**
- Modify: `internal/cmd/push.go`
- Modify (maybe): `internal/registry/manifest.go` (PutManifestResult already has `tag` and `manifest_digest`; add `tag_target_kind`, `tag_target_digest`)

- [ ] **Step 1: Extend `PutManifestResult` to match server response**

Server response shape (from `packages/shared/src/dtos.ts:127` `ManifestPutResponseSchema`):

```json
{
  "tag": "1.0",
  "manifest_digest": "sha256:...",
  "tag_target_kind": "manifest" | "index",
  "tag_target_digest": "sha256:...",
  "latest_updated": false
}
```

```go
type PutManifestResult struct {
    Tag             string `json:"tag"`
    ManifestDigest  string `json:"manifest_digest"`
    TagTargetKind   string `json:"tag_target_kind"`
    TagTargetDigest string `json:"tag_target_digest"`
    LatestUpdated   bool   `json:"latest_updated"`
}
```

- [ ] **Step 2: Test the new fields decode**

```go
func TestPutManifest_DecodesTagTargetFields(t *testing.T) {
    // mock server returns full ManifestPutResponse JSON;
    // assert TagTargetKind and TagTargetDigest are populated
}
```

- [ ] **Step 3: Push prints resulting state**

After successful push:

```
Pushed: sha256:<short> (amd64)
Tag <name>:1.0 -> manifest sha256:<short>
```

When the server composed an index:

```
Pushed: sha256:<short> (amd64)
Tag <name>:1.0 -> index sha256:<idx-short> (auto-composed by server)
```

Initial impl does NOT enumerate the other arches in the index - that needs an extra call (see Task 11 inspect). Open a follow-up issue for the richer "Tag X now supports: amd64 (you, just now), arm64 (alice, 3d ago)" message; the server would need a per-arch authorship enrichment for that.

- [ ] **Step 4: Commit**

NOTE on `--latest`: existing `PutManifest(..., asLatest bool)` keeps using `X-As-Latest: true` header. Server reads it (manifests.ts:193) and upserts the `latest` tag in parallel. NO change. NO separate PutTag call needed.

---

## Phase D - Pull: arch dispatch

### Task 8: GetManifest returns Content-Type

**Files:**
- Modify: `internal/registry/manifest.go`
- Test: `internal/registry/manifest_test.go`

Existing signature: `GetManifest(owner, name, tag) ([]byte, string, error)` where the third return is `Docker-Content-Digest`. Extend to return Content-Type too.

- [ ] **Step 1: Failing test - Content-Type is preserved**

```go
func TestGetManifest_IndexContentType(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w, r) {
        w.Header().Set("Content-Type", manifest.MediaTypeIndexV1)
        w.Header().Set("Docker-Content-Digest", "sha256:idx")
        w.Write([]byte(`{"schemaVersion":1,"mediaType":"application/vnd.cage.index.v1+json","manifests":[]}`))
    }))
    c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
    body, ct, digest, err := c.GetManifest("o", "n", "1.0")
    require.NoError(t, err)
    assert.Equal(t, manifest.MediaTypeIndexV1, ct)
    assert.Equal(t, "sha256:idx", digest)
    assert.NotEmpty(t, body)
}
```

- [ ] **Step 2: Implement**

```go
func (c *Client) GetManifest(owner, name, ref string) (body []byte, contentType, dockerDigest string, err error) {
    path := fmt.Sprintf("/api/v1/repos/%s/%s/manifests/%s", owner, name, ref)
    resp, err := c.do(http.MethodGet, path, nil, nil)
    if err != nil { return nil, "", "", err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { return nil, "", "", parseError(resp) }
    body, err = io.ReadAll(resp.Body)
    if err != nil { return nil, "", "", err }
    return body, resp.Header.Get("Content-Type"), resp.Header.Get("Docker-Content-Digest"), nil
}
```

- [ ] **Step 3: Update existing callers**

`grep -rn "registry.*GetManifest\|c\.GetManifest" internal/ cmd/` and add the new return slot (ignore with `_` where not needed).

- [ ] **Step 4: Commit**

### Task 9: Pull command - Content-Type dispatch + `--platform`

**Files:**
- Modify: `internal/cmd/pull.go`

- [ ] **Step 1: Failing tests**

```go
func TestRunPull_SingleManifest_ArchMatch(t *testing.T)
func TestRunPull_SingleManifest_ArchMismatch_NoOverride_Errors(t *testing.T)
func TestRunPull_SingleManifest_ArchMismatch_WithPlatform_Proceeds(t *testing.T)
func TestRunPull_Index_PicksMatchingArch(t *testing.T)
func TestRunPull_Index_NoMatchingArch_Errors(t *testing.T)
```

- [ ] **Step 2: Update `runRegistryPull(ref, arch)`**

```go
body, ct, _, err := rc.GetManifest(ref.Owner, ref.Name, ref.Tag)
if err != nil { return err }

switch ct {
case manifest.MediaTypeManifestV1:
    var m manifest.Manifest
    if err := json.Unmarshal(body, &m); err != nil { return err }
    if m.Config.Arch != arch {
        return fmt.Errorf("tag %s is %s, host is %s; retry with --platform %s",
            refStr, m.Config.Arch, arch, m.Config.Arch)
    }
    return pullManifest(rc, ref, &m, body)
case manifest.MediaTypeIndexV1:
    var idx manifest.IndexBody
    if err := json.Unmarshal(body, &idx); err != nil { return err }
    var pick *manifest.IndexEntry
    for i, e := range idx.Manifests {
        if e.Platform.Architecture == arch { pick = &idx.Manifests[i]; break }
    }
    if pick == nil {
        return fmt.Errorf("tag %s does not support %s. Available: %s",
            refStr, arch, archesOf(&idx))
    }
    // Fetch the specific arch's manifest by digest, then proceed
    mBody, _, _, err := rc.GetManifest(ref.Owner, ref.Name, pick.Digest)
    if err != nil { return err }
    var m manifest.Manifest
    if err := json.Unmarshal(mBody, &m); err != nil { return err }
    return pullManifest(rc, ref, &m, mBody)
default:
    return fmt.Errorf("unexpected Content-Type %q on manifest endpoint", ct)
}
```

`--platform` flag (Task 5) sets `arch`; absence defaults to `images.HostArchitecture()`. Pass `--platform <arch>` to force-accept an arch-mismatched single manifest (cross-arch debugging).

- [ ] **Step 3: Local ref write**

Once a manifest is pulled, `imgstore.WriteRef` records the ref -> manifest digest. Arch is captured in the manifest's `Config.Arch`, so the local store needs no schema change; subsequent `cage start` against the ref boots whichever manifest the ref points at.

- [ ] **Step 4: Commit**

---

## Phase E - Polish & hand-off

### Task 10: `cage tag inspect` + arch column in listing

**Files:**
- Modify: `internal/cmd/tag.go`

Server has NO `GET /tags/:tag` endpoint. Use `GET /manifests/:tag` instead (returns either manifest or index body with Content-Type indicating which).

- [ ] **Step 1: Add `inspect` subcommand**

```
cage tag inspect localhost/owner/name:1.0
> Kind:          index
> Digest:        sha256:idx...
> Architectures: amd64, arm64
```

For a registry ref:
- Call `rc.GetManifest(owner, name, tag)`.
- If Content-Type = manifest: print `Kind: manifest`, `Digest: <Docker-Content-Digest>`, `Architectures: <Config.Arch>`.
- If Content-Type = index: print `Kind: index`, `Digest: <Docker-Content-Digest>`, `Architectures: <join indexEntry.Platform.Architecture>`.

For a local ref: read `imgstore.ReadRef` -> manifest digest -> `imgstore.GetManifestBytes` -> single-arch only (local always points at single manifest after pull dispatch).

- [ ] **Step 2: Arch column in `cage image ls`**

`imgstore.ListRefs()` + manifest lookup already exposes everything needed; add a column showing the manifest's `Config.Arch`. Indexes don't exist locally (only on the server), so this column is always a single value locally.

- [ ] **Step 3: Commit**

### Task 11: Smoke test against a local cage-hub

**Files:**
- Modify: `test/e2e/multi_arch_test.go` (new)

- [ ] **Step 1: E2E - push from amd64, pull into amd64**

Trivial regression: must not break.

- [ ] **Step 2: E2E - cross-arch push via `--platform`**

Push with `--platform arm64` from an amd64 host. Then push again without flag (= amd64). Verify the response of the second push reports `tag_target_kind: "index"`. Pull with `--platform arm64` returns the arm64 manifest digest; pull without override returns amd64.

The harness needs a real cage-hub instance (gated on `CAGE_HUB_URL` env var as today's e2e is). Skip if not set.

- [ ] **Step 3: Commit**

### Task 12: Docs

**Files:**
- Modify: `docs/cage-hub.md`
- Modify: `README.md` (registry commands section)

- [ ] **Step 1: New section "Architecture support"**

Cover `--platform`, host-arch detection, cross-arch push (mentioning that the server auto-composes the index - the CLI just pushes and reads the response), and the cache layout migration (existing flat caches need re-download).

- [ ] **Step 2: Commit**

---

## Open questions (resolved by server)

1. **Backward-compat for `Config.Arch`?** Resolved: `Config.Arch` IS the arch field on the wire (server reads it). No top-level `Architecture` field exists or is needed. Spec's idea of "move to top-level" was not implemented and the plan no longer needs it.

2. **`base-aliases.json` arm64 URL list - who curates?** Plan asks the implementer to find arm64 URLs alongside existing amd64 URLs. Some distros (e.g. CentOS Stream) may not have a clean per-arch URL pattern. Set `urls.arm64: null` for those and document in the JSON. The build error message ("no arm64 cloud-image, pick a different base") is the user-facing escape hatch.

3. **Local imgstore arch metadata?** Today `imgstore` stores refs without per-arch info. Spec doesn't require local indexes (those live on the server). After a `cage pull` against an index, the local ref points at the specific arch's manifest digest, so locally everything is single-arch. **Decision: no local index storage.**

4. **Tag-set body schema / X-As-Latest?** Resolved: server kept `X-As-Latest` header. There is no separate `PUT /tags/:tag` endpoint. `cage push --latest` continues to set the header as today. No changes.

5. **Arch wire values (`amd64`/`arm64` vs `x86_64`/`aarch64`)?** Resolved: server shipped `amd64`/`arm64`. This plan and any future doc updates use those values. Spec text uses the OCI names in places; treat the spec text as outdated on this point.

---

## Phase ordering and merge gates

- Phases A + B can ship to `main` ahead of any further server work (local-only changes that depend only on already-merged cage-hub endpoints).
- Phase C (Task 7) and Phase D depend on cage-hub's already-shipped multi-arch endpoints on `feature/multi-arch`. Land them once cage-hub's branch is merged to its `main`. The CLI feature branch (`feature/multi-arch`) is already the right home until then.
- Phase E is independent polish; can land in any order after D is in.

Track the cage-hub server PR in this plan's PR description so the merge gate is visible.
