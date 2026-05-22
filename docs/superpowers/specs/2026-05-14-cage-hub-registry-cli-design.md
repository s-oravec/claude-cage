# cage-hub Registry CLI Design

Date: 2026-05-14

## Context

cage-hub is a new project (sibling repo `../cage-hub`) implementing a
Docker-Hub-like registry for cage images. This spec defines the cage
CLI changes needed for users to authenticate against cage-hub registries,
push custom-built cage images, and pull them on other machines.

cage-hub itself (server, web UI, storage backend, OIDC integration with
Keycloak, PAT generation) is out of scope. This document covers only the
cage CLI side and the on-the-wire HTTP contract that cage assumes the
server implements.

## Goals

- First-class `cage login` / `cage logout` / `cage pull` / `cage push` /
  `cage tag` commands that mirror Docker conventions where reasonable.
- Layered image storage from the start (no flat-blob phase): pulled
  images deduplicate by content digest, push only uploads layers the
  server does not yet have.
- Hybrid base-image model: distro base images (Ubuntu, Alpine, ...) are
  NOT stored in the registry. They keep coming from distro cloud-image
  mirrors via the existing `cage pull --base` path. The registry only
  carries the custom layer on top plus a digest reference to the base.
- TLS by default with a per-host insecure opt-in for local development
  against `localhost:5000`, `cage-hub.local`, and similar.
- MVP only: no private images, no automatic token refresh, no CI service
  account flow (PATs generated through the cage-hub web UI cover CI).

## Non-Goals (MVP)

- Private images, per-image ACLs (cage-hub is public-read in MVP; auth
  is required only for push).
- Multi-arch / `--platform` manifests.
- Image signing (cosign, notary).
- Pull/push by explicit digest (`@sha256:...`). MVP is tag-based only;
  digests are an internal concern.
- Resumable layer uploads on the CLI side (full-blob PUT only;
  retry sends whole layer again). The server already implements the
  multipart endpoints needed for resume; the CLI just doesn't drive
  them in MVP.
- Automatic refresh of expired access tokens. Server-issued tokens are
  expected to be long-lived; expiry means the user reruns `cage login`.
- Migration of existing flat custom images into the new layered store.
  Legacy images stay where they are; users rebuild when they want to
  publish.
- Concurrent-operation locking on the same ref.

## Image reference format

References are always fully qualified, including the registry host:

```
<host>/<owner>/<name>:<tag>
```

`<owner>` is the cage-hub username of the repository owner (single
segment, no nested namespaces in MVP). `<name>` is the repository name
under that owner. Examples:

- `cage-hub.io/stiivo/devbox:v1`
- `cage-hub.local/team/runner:latest`

There is no default-registry shortcut. A bare token like `myimage` is
either a distro base alias (e.g. `ubuntu-24.04`) or a local-only tag,
never a registry reference - registry references are recognized by the
presence of a `/`.

If `:<tag>` is omitted, `latest` is assumed.

## Command surface

### Auth

```
cage login   <host>                  # interactive OAuth 2.0 device authorization grant
cage login   <host> --token-stdin    # non-interactive: read PAT from stdin (CI)
cage logout  <host>                  # remove a single host from auth.yaml (idempotent)
cage logout  --all                   # clear all hosts
                                     # bare `cage logout` (no args) is a usage error
cage login   --list                  # list logged-in hosts (host, username, obtained_at)
```

`cage login --token-stdin` accepts a PAT minted on the cage-hub web UI.
The CLI does not distinguish PAT vs Keycloak access token at storage
time - both are opaque bearer strings.

`cage logout` is a **local** operation only - it removes the token
from `auth.yaml` and does not contact the server. Server-side
revocation (so a stolen PAT can no longer authenticate) lives at the
cage-hub web UI under `/settings/tokens` (or
`DELETE /api/v1/me/pats/:id`, which requires a Keycloak JWT and is
therefore not callable from a CLI that only has a PAT). The CLI does
not surface a `cage pat revoke` command in MVP.

### Image transfer

```
cage pull <ref>                      # registry pull when <ref> contains '/'
cage pull --base <name>              # UNCHANGED existing distro base pull
cage push <ref>                      # requires login for the host
cage tag  <src> <dst>                # local-only; no network
```

Detection rule for `cage pull <X>`:

- `X` contains `/` -> registry reference, parse as `host/owner/name:tag`
- `X` is a bare word -> existing distro base alias path (`alpine`,
  `ubuntu-24.04`, ...). Same behavior as today.
- `--base` always selects the distro path explicitly.

`cage tag <src> <dst>` resolves `<src>` to a local manifest digest and
writes a new ref file pointing at the same digest. `<src>` can be a
local name (`refs/_local/<name>/<tag>`) or a registry ref already
present locally (`refs/<host>/<owner>/<name>/<tag>`). `<dst>` is the
new ref. Both forms allowed for `<dst>`. No data is moved or rewritten.
If `<dst>` already exists, the existing ref is overwritten without
prompting (matches `docker tag` behavior).

### Local image management (existing, unchanged surface)

`cage image list / inspect / rm / save` continues to operate, with `rm`
extended to accept a registry-style ref and treat it as a ref deletion
(see Garbage collection below).

## Config files

### `~/.cage/config.yaml` (existing, additive change)

One new optional section:

```yaml
registries:
  insecure:
    - localhost:5000
    - cage-hub.local
```

Hosts listed under `registries.insecure` are reached over plain HTTP and
skip TLS verification. Every other host is HTTPS with a fully validated
chain. No tokens live in `config.yaml`. `cage config show` and `cage
config edit` are unchanged.

### `~/.cage/auth.yaml` (new file, mode `0600`)

Created on first `cage login`, cleared on `cage logout`:

```yaml
registries:
  cage-hub.io:
    token: "ey..."
    username: "stiivo"
    obtained_at: "2026-05-14T10:30:00Z"
  cage-hub.local:
    token: "pat_..."
    username: "stiivo"
    obtained_at: "2026-05-14T11:05:00Z"
```

- Cage writes with `0600`. On read, if the mode is more permissive, it
  prints a warning and chmods back to `0600`.
- No refresh tokens. If Keycloak issues one, cage ignores it.
- No encryption beyond file permissions.
- `cage login --list` reads this file and prints only `host`,
  `username`, `obtained_at`. Tokens never appear in any cage output.
- `cage config show` does not read `auth.yaml` and never reveals tokens.

Auth operations all run in user mode. Sudo handling is irrelevant - the
sudo path is only for VM runtime (start, stop, snapshot under root mode)
and never touches auth files.

## Image storage layout (local)

```
~/.cage/
  images/                            # base distro images (unchanged)
    ubuntu-24.04.qcow2
    metadata/ubuntu-24.04.json

  layers/sha256/<aa>/<digest>/       # NEW: content-addressed custom layers
    layer.qcow2                      #   qcow2 with backing-file POINTER REMOVED
                                     #   via `qemu-img rebase -u -b "" layer.qcow2`

  manifests/sha256/<aa>/<digest>/    # NEW: content-addressed manifests
    manifest.json

  refs/                              # NEW: tag -> manifest digest
    _local/<name>/<tag>              #   for `cage build -t <name>` (no host)
    <host>/<owner>/<name>/<tag>      #   for `cage-hub.io/stiivo/devbox:v1`
                                     #   each ref file contains a single line:
                                     #   sha256:<manifest-digest>\n
```

`<aa>` is the first two hex characters of the digest, mirroring git/OCI
object sharding to keep directories small.

`layers/` and `manifests/` are immutable - blobs are addressed by their
content digest, so two images sharing the same layer write only once.

`refs/` is mutable - tags are created, overwritten, and deleted as
normal user actions.

Custom layers are stored with their backing-file pointer stripped. The
publisher's local path (`/srv/build/...`) would not be valid on the
client, and the manifest itself is the source of truth for the layer
chain. Backing is set up at use time during `cage start`.

## Manifest format (v1)

```json
{
  "schemaVersion": 1,
  "mediaType": "application/vnd.cage.manifest.v1+json",
  "base": {
    "type": "distro",
    "name": "ubuntu-24.04",
    "digest": "sha256:abc..."
  },
  "layers": [
    {
      "digest": "sha256:def...",
      "size": 209715200,
      "mediaType": "application/vnd.cage.layer.v1.qcow2"
    }
  ],
  "config": {
    "os":          "linux",
    "arch":        "amd64",
    "user":        "cage",
    "workdir":     "/home/cage",
    "env":         ["NODE_ENV=development"],
    "description": "...",
    "readme":      "...",
    "cagefile":    "FROM ubuntu-24.04\nRUN ...",
    "resources":   { "memory_mb": 4096, "vcpu": 2, "disk_gb": 20 }
  }
}
```

- `base.type: "distro"`: the only supported base type in MVP. `base.name`
  is the distro alias used by `cage pull --base`; `base.digest` is the
  sha256 of the local `images/<name>.qcow2` the builder used. Client
  verifies its local copy matches; mismatch is a hard error with a
  manual-fix hint.
- `layers`: an ordered list (lowest first). In MVP, every built image
  has exactly one custom layer. The list is plural because a future
  `cage build` may snapshot per RUN/COPY step.
- `config`: metadata and runtime hints. `os`/`arch` describe the cage's
  guest OS. `user`/`workdir`/`env` are applied at `cage start` time
  (via the existing cloud-init / virtiofs env paths). `description`
  and `readme` are the initial values shown on the web UI Image Detail
  page (the repo row owns the live copy - subsequent web edits via
  `PATCH /api/v1/repos/:owner/:name` overwrite the live values without
  rewriting the manifest). `cagefile` is the raw Cagefile text capped
  at 64 KB; omitted for images produced by `cage image save` from a
  running cage (Recipe tab then shows the empty state). `resources`
  carries the recommended `cage init` defaults for downstream pulls.
  All sub-fields are optional except `os` and `arch`.

## Garbage collection

A manifest or layer may be referenced by multiple ref files. Deletion of
a ref (`cage image rm <ref>`) only removes the ref. The underlying
manifest and layers become candidates for prune when their refcount
drops to zero.

MVP does not run automatic GC. Dangling blobs stay on disk until the
user runs `cage image prune` (deferred command; not strictly MVP). They
are visible in `cage image list --all`.

## Build flow

`cage build -t <ref> <context>` continues to use the existing temp-cage
+ overlay strategy for executing RUN/COPY steps. Only the post-build
artifact handling changes.

1. Build runs as today: a temp cage starts with a qcow2 overlay whose
   backing file is the base distro image. RUN/COPY steps execute inside.
2. After the temp cage stops and virt-customize cleanup runs, take the
   overlay disk (`vm-dir/disk.qcow2`).
3. `qemu-img rebase -u -b "" disk.qcow2` to strip the backing-file
   pointer. `-u` updates metadata only; no data rewrite.
4. Compute `layer_digest` = sha256 of the resulting qcow2.
5. Move the file to `layers/sha256/<aa>/<layer_digest>/layer.qcow2`.
6. Compute `base_digest` = sha256 of `images/<base-name>.qcow2`.
7. Build the manifest JSON (base, layers=[layer], config from the
   Cagefile).
8. Compute `manifest_digest`, write
   `manifests/sha256/<aa>/<manifest_digest>/manifest.json`.
9. Write the ref:
   - `-t myimage`              -> `refs/_local/myimage/latest`
   - `-t myimage:v1`            -> `refs/_local/myimage/v1`
   - `-t cage-hub.io/u/r:v1`    -> `refs/cage-hub.io/u/r/v1`

The legacy flatten step (`qemu-img convert -O qcow2 -c`) is removed
from the build path. There is no fallback to the old layout.

## Pull flow

`cage pull <ref>`:

1. Parse `<ref>` as `(host, owner, name, tag)`; default `tag` to
   `latest`.
2. Resolve TLS mode: HTTPS unless `host` is in `registries.insecure`,
   in which case HTTP and no cert validation.
3. Anonymous request. If the server returns 401 (forward-compatible
   with private images, which are not MVP), print: `this image
   requires auth - run cage login <host>`.
4. GET `https://<host>/api/v1/repos/<owner>/<name>/manifests/<tag>`.
   The response body is the canonical manifest JSON. Verify
   `sha256(body) == Docker-Content-Digest` response header. Store at
   `manifests/sha256/<aa>/<manifest-digest>/manifest.json`.
5. For each layer in the manifest:
   - If `layers/sha256/<aa>/<digest>/layer.qcow2` exists locally, skip.
   - Else GET
     `https://<host>/api/v1/repos/<owner>/<name>/blobs/sha256:<digest>`.
     The server streams the blob directly from object storage. Stream to
     a tmp file while computing sha256, verify against the requested
     digest, atomic rename into the final layer path.
   - On network interruption, the next attempt resumes via
     `Range: bytes=<offset>-` against the same endpoint - cage-hub
     forwards `Range:` to the storage backend.
6. Base image check:
   - If `images/<manifest.base.name>.qcow2` is missing, invoke the
     existing distro pull flow (the same code path `cage pull --base`
     uses) to fetch it from the distro cloud-image mirror.
   - If present but its sha256 does not match `manifest.base.digest`,
     fail with: `local base image <name> differs from the one used to
     build this image (have <local-digest>, need <manifest-digest>); run
     cage image rm <name> and cage pull --base <name>`.
7. Write `refs/<host>/<owner>/<name>/<tag>` containing
   `sha256:<manifest-digest>`.

Blobs come straight from `/blobs/sha256:<digest>` rather than via
presigned URLs - simpler and anonymous-friendly. Pull resume is a plain
HTTP Range request against the same endpoint.

## Push flow

`cage push <ref> [--latest]` uses the Docker V2 style protocol that
cage-hub implements: per-layer HEAD, per-layer single-PUT blob upload,
then a manifest PUT that pins the tag to the freshly-uploaded layers.

1. Resolve `refs/<host>/<owner>/<name>/<tag>` to a manifest digest.
   Missing -> `no local image tagged <ref>`.
2. Read `auth.yaml`, look up `token` for `host`. Missing -> `not logged
   in to <host> - run cage login <host>`.
3. For each layer in the manifest:
   a. `HEAD https://<host>/api/v1/repos/<owner>/<name>/blobs/sha256:<digest>`
      with `Authorization: Bearer <token>`.
      - 200 -> server already has the layer (dedup hit), skip to next.
      - 404 -> upload (steps b-c).
   b. `POST https://<host>/api/v1/repos/<owner>/<name>/blobs/uploads`
      with `Authorization: Bearer <token>` and body
      `{"digest":"sha256:<digest>", "size":<bytes>}`.
      Response 202: `{"upload_id":"...", "upload_url":"...",
      "expires_at":"..."}`. The repo is auto-created if missing and the
      caller is the namespace owner.
   c. `PUT <upload_url>?digest=sha256:<digest>` with
      `Authorization: Bearer <token>`,
      `Content-Type: application/octet-stream`, and the layer file
      streamed as the body.
      - 201 + `Docker-Content-Digest: sha256:<digest>` -> done.
      - 400 `CONFLICT_DIGEST_MISMATCH` -> server's computed sha256
        differs from the requested digest; surface as a hard error.
4. `PUT https://<host>/api/v1/repos/<owner>/<name>/manifests/<tag>`
   with:
   - `Authorization: Bearer <token>`
   - `Content-Type: application/vnd.cage.manifest.v1+json`
   - `X-As-Latest: true` when the user passed `--latest` (server then
     additionally upserts the `latest` tag pointer to the same manifest)
   - Body: the canonical manifest JSON (byte-for-byte identical to the
     local `manifests/sha256/<aa>/<manifest-digest>/manifest.json`).
   - 201 with `Docker-Content-Digest` -> new tag.
   - 200 with `latest_updated: false` -> idempotent no-op (same `(repo,
     tag, manifest_digest)` already exists; not an error).
   - 409 `BLOB_MISSING` -> the server lost or never received a layer;
     re-run push to re-upload.

No resumable / chunked uploads in MVP - server-side multipart upload
exists as a parallel endpoint (`POST /blobs/uploads?multipart=true`)
but the CLI MVP does not use it. A push that fails mid-blob re-uploads
the whole layer on retry. Adding multipart selection (based on layer
size and `auth/info.multipart_part_size`) is the first follow-up after
MVP.

403 from any push endpoint is surfaced as `not authorized to push to
<ref> - check namespace ownership or collaborator role, or run cage
login <host>`.

## Auth endpoint discovery

`cage login <host>` (without `--token-stdin`) needs to know the
Keycloak realm endpoints. It fetches them from cage-hub:

```
GET https://<host>/api/v1/auth/info
->
{
  "issuer":                          "https://<host>/auth/realms/cage-hub",
  "device_authorization_endpoint":   "https://.../auth/realms/cage-hub/protocol/openid-connect/auth/device",
  "token_endpoint":                  "https://.../auth/realms/cage-hub/protocol/openid-connect/token",
  "client_id":                       "cage-cli",
  "scopes":                          ["openid", "profile"],
  "pat_format":                      "cgh_<base64url>",
  "pat_console_url":                 "https://<host>/settings/tokens",
  "supported_layer_media_types":     ["application/vnd.cage.layer.v1.qcow2"],
  "supported_manifest_media_types":  ["application/vnd.cage.manifest.v1+json"],
  "max_manifest_size":               65536,
  "max_layer_size":                  21474836480,
  "multipart_part_size":             67108864
}
```

cage then talks directly to the Keycloak device authorization endpoint
(no proxying through cage-hub). After successful auth, cage uses the
access token as a Bearer credential on all `/api/v1/*` calls.

cage-hub recognizes both Keycloak-issued JWTs (verified via JWKS) and
its own PAT strings (DB lookup by SHA256 hash). The CLI does not need
to distinguish.

`pat_console_url` is used only in error hints ("generate a PAT at
<url>") when the CLI rejects an unauthenticated push. `max_layer_size`,
`max_manifest_size`, and `multipart_part_size` are read at build time
to validate that a layer is acceptable before attempting an upload.

## Start flow integration

`cage start --image <ref>` (or via `.cage.yml`):

- If `<ref>` is a distro alias (`ubuntu-24.04`, no `/`): existing
  behavior. Per-cage overlay's backing is `images/<name>.qcow2`.
- Otherwise resolve via `refs/.../<tag>` -> manifest digest -> manifest:
  1. Verify the base image is present locally (same check as Pull step
     6, including digest match).
  2. Materialize the layer chain into the per-cage VM dir:
     - Copy `layers/sha256/<aa>/<top-layer>/layer.qcow2` to
       `<vm-dir>/disk-base.qcow2`.
     - `qemu-img rebase -u -b <base-image-path> -F qcow2
       <vm-dir>/disk-base.qcow2`.
     - For MVP with a single custom layer, this is the whole chain.
       When multi-layer images arrive, repeat copy+rebase from the
       lowest layer up.
  3. Create the usual per-cage overlay `<vm-dir>/disk.qcow2` with
     backing = `<vm-dir>/disk-base.qcow2`.
  4. Apply manifest `config` to the cage runtime (user, workdir, env
     injection via the existing cloud-init / virtiofs env paths).

## Error handling summary

- Device-flow timeout (around 5 min) or user-code expiry (around 10
  min): `authorization timed out, try again`.
- Network failure during poll: exponential backoff (1s, 2s, 4s, 8s, 16s),
  abort after 5 consecutive failures.
- `cage push` 401/403: `not authorized to push to <ref> - check namespace
  ownership or run cage login <host>`.
- Push digest mismatch (server recomputes a different digest): server
  returns 400; cage logs and exits non-zero.
- Base image digest mismatch on pull: hard error with manual-fix hint
  (rm + pull --base).
- `cage tag` on missing source: `image not found: <src> (run cage image
  list to see available)`.
- `cage logout` on a host not in `auth.yaml`: idempotent, exit 0.
- TLS verification failure on a non-insecure host: `TLS verification
  failed for <host>: ...; if this is a dev registry, add it to
  registries.insecure in ~/.cage/config.yaml`.
- Disk full mid-pull: clean up tmp files, error with `free X GB and
  retry`.
- Expired token on any registry call: `token expired, run cage login
  <host>`. No auto-refresh in MVP.
- Concurrent operations on the same ref: NOT handled in MVP. Single
  user, single shell assumed.

## Out-of-scope items deferred to follow-ups

- `cage image prune` for GC of unreferenced layers and manifests.
- Resumable layer uploads driven by the CLI (using cage-hub's
  multipart endpoints under `POST /blobs/uploads?multipart=true` +
  per-part presigned URLs + `POST .../complete`).
- Automatic token refresh on 401.
- Multi-layer builds (`cage build` snapshotting per RUN/COPY).
- Pull/push by digest.
- Private images and per-image ACLs.
- CI service-account / client-credentials flow.
- Image signing.
- Migration tool for legacy flat custom images.
