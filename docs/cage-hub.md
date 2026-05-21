# Cage-hub registry

cage-hub is a registry for cage images, similar in spirit to Docker Hub for cages. Image refs are fully qualified as `host/owner/name:tag`. The cage CLI can target any number of cage-hub hosts; the host in the ref selects which one.

## Authenticate

```
cage login cage-hub.io
```

Opens a device flow: cage prints a URL and a user code, you open the URL in a browser, enter the code, and approve. The resulting token is stored in `~/.claude-cage/auth.yaml` (mode 0600).

For CI / non-interactive use, generate a personal access token from the cage-hub web UI under `/settings/tokens` and pipe it in:

```
echo "$CAGE_HUB_TOKEN" | cage login cage-hub.io --token-stdin
```

List logged-in hosts:

```
cage login --list
```

Remove credentials:

```
cage logout cage-hub.io
cage logout --all
```

Note: `cage logout` is local-only. To revoke a PAT, use the cage-hub web UI.

## Build and push

```
cd ~/projects/myapp
cage build -t cage-hub.io/stiivo/devbox:v1 .
cage push cage-hub.io/stiivo/devbox:v1
```

Add `--latest` to also move the `latest` tag:

```
cage push cage-hub.io/stiivo/devbox:v1 --latest
```

Layers already present on the server are skipped (HEAD probe). Layers above ~256 MB use multipart upload for resumability; smaller layers use a single PUT.

To build and publish for a specific architecture, see [Architecture support](#architecture-support).

## Pull and run

On another machine, after `cage login`:

```
cage pull cage-hub.io/stiivo/devbox:v1
```

This fetches the manifest, verifies its sha256, downloads missing layer blobs (resumable via HTTP Range), and validates the base distro image. If the base is missing locally, cage pulls it from the distro mirror automatically.

To run:

```
cd ~/projects/anotherapp
cat > .cage.yml <<'EOF'
image: cage-hub.io/stiivo/devbox:v1
EOF
cage start
```

`cage start` materializes the layer chain (`disk-base.qcow2` rebased to the local distro base) and creates the per-cage overlay on top.

See [Architecture support](#architecture-support) for pulling a non-host architecture with `--platform`.

## Architecture support

cage images are architecture-specific. The base distro aliases (`alpine`, `ubuntu`, `debian` and their versioned variants) are each available for both `amd64` and `arm64`.

### Host-arch auto-detection and `--platform`

cage auto-detects the host architecture (from Go's `GOARCH`) and uses it by default. To target a different architecture, pass `--platform amd64` or `--platform arm64`. The flag is accepted on `cage build` and `cage pull` only - not on `cage push`. An empty/omitted value means the host architecture; invalid values are rejected.

### Building for a target architecture

```
cage build -t cage-hub.io/stiivo/devbox:v1 --platform arm64 .
```

Building for an architecture other than the host's runs the build under QEMU emulation and is noticeably slower than a native-arch build. Native-arch builds are fast.

### Cross-arch push and server-side auto-composition

`cage push` has no `--platform` flag: the architecture comes from the built image. The CLI always pushes a single-arch manifest and never composes multi-arch indexes itself. When you push a second architecture to the same tag, the cage-hub server auto-composes a multi-arch index.

Push prints the resulting tag target. For a single architecture it reports a `manifest`:

```
Pushed: sha256:abc123def456 (amd64)
Tag stiivo/devbox:v1 -> manifest sha256:abc123def456
```

After pushing a second architecture to the same tag, the server composes an index and push reports it:

```
Pushed: sha256:789abc012def (arm64)
Tag stiivo/devbox:v1 -> index sha256:fed987cba654 (auto-composed by server)
```

A typical cross-arch publish flow:

```
cage build -t cage-hub.io/stiivo/devbox:v1 --platform amd64 .
cage push cage-hub.io/stiivo/devbox:v1
cage build -t cage-hub.io/stiivo/devbox:v1 --platform arm64 .
cage push cage-hub.io/stiivo/devbox:v1   # server auto-composes the index
```

### Pull architecture dispatch

`cage pull` resolves a tag and dispatches by architecture:

- If the tag resolves to a multi-arch index, pull selects the entry matching the target architecture (host arch, or `--platform`). If no entry matches, pull errors and lists the architectures the index provides.
- If the tag resolves to a single manifest of a different architecture, pull errors with a hint to retry with `--platform <that arch>`.

### Base-image cache

The base-image cache is flat: each base lives at `~/.claude-cage/images/<name>.qcow2`, with the architecture recorded in the image metadata. Only one architecture of a given base is on disk at a time. Pulling or building a different architecture of the same base re-downloads it (replacing the cached copy). Caches created before multi-arch support are treated as the host architecture.

### Inspecting a tag's architectures

`cage tag inspect <ref>` reports whether a tag points at a single-arch `manifest` or a multi-arch `index`, its digest, and the architecture(s) it covers. It works for both registry refs and local refs.

```
cage tag inspect cage-hub.io/stiivo/devbox:v1
Kind:          index
Digest:        sha256:fed987cba654
Architectures: amd64, arm64
```

## Retag locally

```
cage tag cage-hub.io/stiivo/devbox:v1 cage-hub.io/stiivo/devbox:stable
cage push cage-hub.io/stiivo/devbox:stable
```

## Insecure / dev registries

For local development against `localhost:5000` or `cage-hub.local`, add the host to the insecure allowlist in `~/.claude-cage/config.yaml`:

```yaml
registries:
  insecure:
    - localhost:5000
    - cage-hub.local
```

These hosts are reached over plain HTTP with no TLS verification.

## Reference

- CLI spec: `docs/superpowers/specs/2026-05-14-cage-hub-registry-cli-design.md`
- Server: see `../cage-hub` repo
- Implementation plan: `docs/superpowers/plans/2026-05-15-cage-hub-registry-cli.md`
