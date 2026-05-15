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
