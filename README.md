<h1><img src="assets/claude-cage.png" height="50" alt="Claude Cage" style="vertical-align: middle;"> Claude Cage</h1>

A lightweight QEMU/KVM-based VM sandbox CLI for running Claude Code in isolation.

## Features

- **VM Isolation**: Run Claude Code in a secure, isolated VM environment
- **QEMU/KVM Backend**: Uses libvirt for VM management
- **Copy-on-Write Disks**: Changes don't affect base images
- **File Sharing**: Share directories with the VM using virtio-fs (root mode)
- **Port Forwarding**: Forward ports between host and VM
- **Snapshots**: Create and restore VM snapshots
- **Multiple Profiles**: Light, default, and heavy resource profiles
- **Multiple Distros**: Alpine, Ubuntu, Debian (each available for amd64 and arm64)

## Operating Modes

Claude Cage runs in one of two modes, distinguished by whether the cage
process has root privileges. The mode determines which features are
available:

| Feature | User mode (`cage`) | Root mode (`sudo cage`) |
|---|---|---|
| SSH into the VM | ✅ | ✅ |
| SLIRP / user-mode networking | ✅ | ✅ |
| VM-side network blocking (cloud-init) | ✅ | ✅ |
| Snapshots | ✅ | ✅ |
| Host-level network isolation (netns + passt) | ❌ | ✅ |
| Bridge networking (libvirt-managed) | ❌ | ✅ |
| Shared folders (virtiofs) | ❌ | ✅ |
| Injected env via virtiofs | ❌ | ✅ |
| libvirt backend | session | system |
| Metadata path (state.json, SSH keys) | `~/.claude-cage/` | `~/.claude-cage/` (invoking user) |
| VM artifacts path (disk, ISO, images) | `~/.claude-cage/` | `/var/lib/libvirt/images/cage/` |

**User mode** is the default and what most users want: a sandboxed VM
with SSH and SLIRP networking, no host configuration required. Run `cage
init` to create a user-mode cagefile.

**Root mode** unlocks shared folders, environment injection, and stronger
network isolation. Run `cage init --root` to create a root-mode cagefile,
then `sudo cage start`. Root mode requires libvirt-qemu apparmor
compatibility (handled automatically when state lives under
`/var/lib/libvirt/images/`).

Cage **enforces the mode at start time**: if your `.cage.yml`
includes `shares:`, `env:`, or `network: bridge`, running plain `cage
start` errors out with a hint to use `sudo cage start`.

See [docs/modes.md](docs/modes.md) for the full design rationale.
For cage's first-boot bootstrap (user creation, SSH key injection,
locales, virtiofs mounts, env injection) see
[docs/cloud-init.md](docs/cloud-init.md).

## Installation

### Prerequisites

- Linux with KVM support
- QEMU and libvirt
- virtiofsd (for file sharing)

Check dependencies:
```bash
cage doctor
```

### From Source

```bash
git clone https://github.com/s-oravec/claude-cage.git
cd claude-cage
make build
make install  # installs to ~/.local/bin/
```

For root mode (`sudo cage`), install system-wide instead so sudo's
default `secure_path` can find the binary:

```bash
make install-system  # installs to /usr/local/bin/ (asks for sudo)
```

### Using Go

```bash
go install github.com/s-oravec/claude-cage/cmd/cage@latest
```

## Quick Start

### Project-Based Workflow (Recommended)

```bash
# Check system requirements
cage doctor

# Initialize global configuration
cage config init

# Download a base image
cage pull --base ubuntu-24.04

# In your project directory, create a cage configuration
cd ~/projects/myapp
cage init --image ubuntu-24.04

# Start the cage (creates automatically on first run)
cage start

# Connect via SSH
cage ssh

# Stop the cage (preserves resources)
cage stop

# Remove the cage completely
cage remove
```

As an alternative to manually copying qcow2 files between machines, you can publish a built image to a cage-hub registry and pull it on another host (see [Registry Commands](#registry-commands) and [docs/cage-hub.md](docs/cage-hub.md)):

```bash
cage login cage-hub.io
cage build -t cage-hub.io/me/devbox:v1 .
cage push cage-hub.io/me/devbox:v1
# on another machine:
cage pull cage-hub.io/me/devbox:v1
```

### Direct Usage (Without Project Config)

```bash
# Start an existing cage by name
cage start myvm

# Connect to a specific cage
cage ssh myvm

# Stop a specific cage
cage stop myvm
```

## Commands

### Lifecycle Commands
- [`cage init`](#cage-init) - Initialize cage configuration in current directory
- [`cage start`](#cage-start) - Start a cage (creates if needed)
- [`cage stop`](#cage-stop) - Stop a running cage (preserves resources)
- [`cage remove`](#cage-remove) - Remove a cage and all its resources
- [`cage restart`](#cage-restart) - Restart a running cage

### Connection Commands
- [`cage ssh`](#cage-ssh) - Connect to a cage via SSH
- [`cage exec`](#cage-exec) - Execute a command in a cage
- [`cage console`](#cage-console) - Connect to serial console

### Information Commands
- [`cage list`](#cage-list) - List all cages
- [`cage status`](#cage-status) - Show detailed cage status
- [`cage logs`](#cage-logs) - View cage console logs

### Management Commands
- [`cage snapshot`](#cage-snapshot) - Manage cage snapshots
- [`cage port`](#cage-port) - Manage port forwarding
- [`cage image`](#cage-image) - Manage custom images

### Build Commands
- [`cage build`](#cage-build) - Build an image from a Cagefile

### Setup Commands
- [`cage pull`](#cage-pull) - Download base images
- [`cage config`](#cage-config) - Manage configuration
- [`cage doctor`](#cage-doctor) - Check system requirements
- [`cage verify`](#cage-verify) - Verify network isolation

### Registry Commands
- [`cage login`](#registry-commands) - Authenticate with a cage-hub registry
- [`cage logout`](#registry-commands) - Remove stored registry credentials
- [`cage push`](#registry-commands) - Push a local image to a registry
- [`cage pull`](#registry-commands) - Pull an image from a registry (also accepts registry refs)
- [`cage tag`](#registry-commands) - Create or move a local tag

See [docs/cage-hub.md](docs/cage-hub.md) for the full how-to.

---

## Command Reference

### cage init

Initialize a `.cage.yml` configuration file in the current directory. This file defines how `cage start` will create and run the cage.

```bash
cage init [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--image` | Base image name (default: from `~/.claude-cage/config.yaml`) |
| `--cage` | Cage name (default: directory name) |
| `--memory` | Memory allocation (e.g., `4G`, `8G`) |
| `--vcpu` | Number of virtual CPUs |
| `--disk` | Disk size in GB |
| `--ssh` | SSH port: `auto` or specific port (default: `auto`) |
| `-f, --force` | Overwrite existing configuration |
| `--dir` | Target directory (default: current directory) |

**Examples:**
```bash
# Initialize using default image from config
cage init

# Initialize with specific image
cage init --image ubuntu-24.04

# Initialize with custom resources
cage init --image debian-12 --memory 8G --vcpu 4

# Initialize with specific cage name
cage init --image alpine --cage my-sandbox
```

---

### cage start

Start a cage. If run in a directory with `.cage.yml` and the cage doesn't exist, it will be created automatically.

```bash
cage start [name] [options]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Cage name (optional if `.cage.yml` exists in current directory) |

**Options:**

| Option | Description |
|--------|-------------|
| `--port` | Port forwarding (e.g., `8080:80`), can be specified multiple times |

**Behavior:**
- If `name` is provided: starts the specified cage
- If `name` is omitted: reads cage name from `.cage.yml` in current directory
- If cage doesn't exist and `.cage.yml` is present: creates the cage first
- If cage exists: validates configuration and reconfigures if stopped

**Examples:**
```bash
# Start cage from project config (in project directory)
cage start

# Start a specific cage by name
cage start myproject

# Start with port forwarding
cage start --port 8080:80 --port 3000:3000
```

---

### cage stop

Stop a running cage. Resources (disk, network, keys) are preserved for restart.

```bash
cage stop <name> [options]
cage stop --all
```

**Options:**

| Option | Description |
|--------|-------------|
| `-f, --force` | Force immediate shutdown (default: graceful) |
| `-a, --all` | Stop all running cages |

**Examples:**
```bash
# Graceful shutdown
cage stop myproject

# Force immediate shutdown
cage stop myproject --force

# Stop all cages
cage stop --all
```

---

### cage remove

Remove a cage and all its associated resources permanently.

```bash
cage remove <name> [options]
cage rm <name> [options]
cage remove --all
```

**Options:**

| Option | Description |
|--------|-------------|
| `-f, --force` | Force removal even if running |
| `-a, --all` | Remove all cages |

**Examples:**
```bash
# Remove a stopped cage
cage remove myproject

# Remove a running cage
cage remove myproject --force

# Remove all cages
cage remove --all --force
```

---

### cage restart

Restart a running cage by stopping and starting it.

```bash
cage restart <name> [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-f, --force` | Force immediate shutdown before restart |

**Examples:**
```bash
# Graceful restart
cage restart myproject

# Force restart
cage restart myproject --force
```

---

### cage ssh

Connect to a running cage via SSH.

```bash
cage ssh [-A] <name> [command]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-A, --forward-agent` | Forward the local ssh-agent into the cage (for `git clone`/`ssh` to private hosts from inside, without copying keys in) |

**Examples:**
```bash
# Interactive shell
cage ssh myproject

# Run a command
cage ssh myproject ls -la

# Run multiple commands
cage ssh myproject "cd /workspace && make build"

# Clone a private Gitea repo from inside the cage using your host's ssh-agent
cage ssh -A myproject
# (inside cage): git clone gitea.internal.example.com:org/repo.git
```

**SSH agent forwarding under `sudo`:** `sudo` resets the environment by
default, so `SSH_AUTH_SOCK` gets stripped before cage runs. To make
`sudo cage … -A` "just work" without per-host sudoers tweaks, cage
auto-discovers the invoking user's ssh-agent socket via `$SUDO_USER`
and the standard runtime-dir layout (`/run/user/<uid>/keyring/ssh`,
`ssh-agent.socket`, `openssh_agent`). See
[docs/agent-forwarding.md](docs/agent-forwarding.md) for the security
model.

---

### cage exec

Execute a command in a running cage without TTY allocation. Useful for scripting.

```bash
cage exec <name> -- <command> [args...]
```

**Examples:**
```bash
# Get system info
cage exec myproject -- uname -a

# Check disk usage
cage exec myproject -- df -h

# Run a script
cage exec myproject -- /workspace/build.sh
```

---

### cage console

Connect to the cage VM's serial console. This is the primary way to access cages using `--network auto` (default). Also useful for debugging boot issues.

```bash
cage console <name>
```

**Login credentials:** `cage` / `cage`

**Exit:** Press `Ctrl+]`

**Examples:**
```bash
cage console myproject
```

---

### cage list

List all cages and their status. Alias: `ls`

```bash
cage list [options]
cage ls [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output as JSON |

**Examples:**
```bash
# List all cages
cage list

# JSON output
cage list --json
```

---

### cage status

Display detailed status information for a cage.

```bash
cage status <name> [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output as JSON |
| `-w, --watch` | Continuously update status |

**Examples:**
```bash
# Show status
cage status myproject

# Watch status
cage status myproject --watch

# JSON output
cage status myproject --json
```

---

### cage logs

Display system logs from a running cage.

```bash
cage logs <name> [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-n, --lines` | Number of lines to show (default: 100) |
| `-f, --follow` | Follow log output (stream) |

**Examples:**
```bash
# Show last 100 lines
cage logs myproject

# Show last 50 lines
cage logs myproject -n 50

# Follow logs
cage logs myproject -f
```

---

### cage snapshot

Manage snapshots for cage VMs.

```bash
cage snapshot <subcommand>
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `create` | Create a snapshot |
| `list` | List snapshots (alias: `ls`) |
| `restore` | Restore to a snapshot |
| `remove` | Remove a snapshot (aliases: `rm`, `delete`) |

**Examples:**
```bash
# Create snapshot
cage snapshot create myproject --name before-update

# List snapshots
cage snapshot list myproject

# Restore snapshot
cage snapshot restore myproject --name before-update

# Remove snapshot
cage snapshot remove myproject --name before-update
```

---

### cage port

Manage port forwarding rules for cages.

```bash
cage port <subcommand>
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `add` | Add a port forward |
| `list` | List port forwards (alias: `ls`) |
| `remove` | Remove a port forward |

**Examples:**
```bash
# Add port forward
cage port add myproject 8080:80

# List port forwards
cage port list myproject

# Remove port forward
cage port remove myproject 8080
```

---

### cage image

Manage base and custom images.

```bash
cage image <subcommand>
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `list` | List available images (alias: `ls`) |
| `save` | Save a stopped cage as a new image |
| `remove` | Remove an image (aliases: `rm`, `delete`) |
| `inspect` | Show image details |

**Notes:**
- `save` requires the cage to be stopped to avoid corrupted disk state
- When run from a project directory with `.cage.yml`, cage-name is optional
- Saved images are prepared for reuse (SSH keys cleared, cloud-init reset)
- For full image preparation, install `virt-customize` (from `libguestfs-tools`)
- `list` includes an `ARCH` column showing each image's architecture (`amd64` or `arm64`)

**Examples:**
```bash
# List images
cage image list

# Save cage as image (explicit cage name)
cage image save myproject --name my-custom-image

# Save cage as image (from project directory)
cage image save --name my-custom-image

# Remove image
cage image remove my-custom-image

# Inspect image
cage image inspect ubuntu-24.04
```

---

### cage build

Build a custom image from a Cagefile using Dockerfile-compatible syntax.

```bash
cage build -t <name> <context>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `context` | Build context directory (for COPY operations) |

**Options:**

| Option | Description |
|--------|-------------|
| `-t, --tag` | Name for the built image (required) |
| `-f, --file` | Path to Cagefile (default: `<context>/Cagefile`) |
| `--build-arg` | Build argument KEY=VALUE (can be repeated) |
| `--keep-on-error` | Keep temporary cage **defined** on build failure (stopped) |
| `--interactive` | On failure, leave the temp cage **running** with SSH instructions for live debugging |
| `-A, --forward-agent` | Forward host ssh-agent into RUN steps (for `git clone` over SSH from inside the build) |
| `--platform` | Target architecture (`amd64` or `arm64`); defaults to the host architecture |

Cross-arch builds (building for an architecture other than the host's) run under
QEMU emulation and are noticeably slower than native-arch builds. See
[docs/cage-hub.md](docs/cage-hub.md) for details on architecture support.

**Cagefile Instructions:**

| Instruction | Description |
|-------------|-------------|
| `FROM <image>` | Base image (required, must be first) |
| `ARG <name>=<value>` | Build-time argument |
| `ENV <key>=<value>` | Environment variable |
| `WORKDIR <path>` | Set working directory |
| `USER <username>` | Identity for subsequent `RUN`/`COPY` (default: `root`) |
| `COPY <src> <dest>` | Copy files from build context |
| `RUN <command>` | Execute shell command (supports `\` line continuation) |

For full syntax documentation, see [Cagefile Reference](docs/cagefile.md).

**Example Cagefile:**
```dockerfile
FROM ubuntu-24.04
ARG VERSION=1.0
ENV NODE_ENV=development
WORKDIR /app

# Multiline RUN with backslash continuation
RUN apt-get update && \
    apt-get install -y \
    nodejs \
    npm

COPY ./package.json /app/
RUN npm install
COPY ./src /app/src
```

**Examples:**
```bash
# Build from current directory
cage build -t my-dev-env .

# Build with custom Cagefile location
cage build -t my-image -f ./docker/Cagefile ./project

# Build with build arguments
cage build -t my-image --build-arg VERSION=2.0 --build-arg DEBUG=true .

# Keep temp cage on failure for debugging (stopped, files preserved)
cage build -t my-image --keep-on-error .

# Like --keep-on-error but cage stays RUNNING; SSH in to inspect live state
cage build -t my-image --interactive .

# Clone a private Gitea repo during the build using your host's ssh-agent
cage build -t my-image -A .
# (Cagefile: RUN git clone gitea.internal.example.com:org/repo.git)
```

Under `sudo cage build -A`, cage auto-discovers the invoking user's
ssh-agent socket from `$SUDO_USER` (no sudoers tweak needed). See
[docs/agent-forwarding.md](docs/agent-forwarding.md).

---

### cage pull

Download and prepare base images.

```bash
cage pull [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-b, --base` | Base image to download |
| `-l, --list` | List available images |
| `--platform` | Target architecture (`amd64` or `arm64`); defaults to the host architecture |

**Available Images:**

| Image | Description |
|-------|-------------|
| `alpine` / `alpine-3.21` | Alpine Linux 3.21 (minimal, ~250MB) |
| `alpine-3.20` | Alpine Linux 3.20 |
| `ubuntu` / `ubuntu-24.04` | Ubuntu 24.04 LTS |
| `ubuntu-22.04` | Ubuntu 22.04 LTS |
| `ubuntu-20.04` | Ubuntu 20.04 LTS |
| `debian` / `debian-12` | Debian 12 (Bookworm) |
| `debian-11` | Debian 11 (Bullseye) |

Each image is available for both `amd64` and `arm64`. By default pull selects the
host architecture (auto-detected); override it with `--platform amd64` or
`--platform arm64`.

**Examples:**
```bash
# Download default image (alpine)
cage pull

# List available images
cage pull --list

# Download specific image
cage pull --base ubuntu-24.04

# Use alias
cage pull --base ubuntu

# Pull from a registry (fully-qualified ref)
cage pull cage-hub.io/stiivo/devbox:v1
```

---

## Registry Commands

cage can publish and fetch built images from a cage-hub registry, similar in spirit to Docker Hub. Image refs are fully qualified as `host/owner/name:tag` (e.g. `cage-hub.io/stiivo/devbox:v1`). See [docs/cage-hub.md](docs/cage-hub.md) for the full how-to.

### cage login

Authenticate with a registry host using device flow (interactive) or a personal access token (CI). Credentials are stored in `~/.claude-cage/auth.yaml` (mode 0600).

```bash
cage login <host>           # device flow: prints URL and user code
cage login <host> --token-stdin   # read PAT from stdin
cage login --list           # list logged-in hosts
```

Example:
```bash
cage login cage-hub.io
echo "$CAGE_HUB_TOKEN" | cage login cage-hub.io --token-stdin
```

### cage logout

Remove stored credentials for a host. This is local-only; to revoke a PAT, use the registry's web UI.

```bash
cage logout <host>
cage logout --all
```

### cage push

Push a local image to a registry. Layers already present on the server are skipped via HEAD probe; large layers use multipart upload for resumability.

```bash
cage push <host/owner/name:tag> [--latest]
```

Example:
```bash
cage push cage-hub.io/stiivo/devbox:v1
cage push cage-hub.io/stiivo/devbox:v1 --latest   # also move :latest
```

`cage push` has no `--platform` flag: the architecture comes from the built
image. The CLI always pushes a single-arch manifest and never composes indexes
itself. When you push a second architecture to the same tag, the cage-hub server
auto-composes a multi-arch index. Push prints the resulting tag target (a
`manifest` for a single arch, or an `index` once the server composes one):

```
Pushed: sha256:abc123def456 (amd64)
Tag stiivo/devbox:v1 -> manifest sha256:abc123def456
```

and, after pushing a second arch to the same tag:

```
Pushed: sha256:789abc012def (arm64)
Tag stiivo/devbox:v1 -> index sha256:fed987cba654 (auto-composed by server)
```

See [docs/cage-hub.md](docs/cage-hub.md) for the full architecture-support how-to.

### cage pull (registry ref)

`cage pull` also accepts a fully-qualified registry ref. It downloads the manifest, verifies its sha256, fetches any missing layer blobs (resumable via HTTP Range), and pulls the base distro image automatically if missing locally.

```bash
cage pull cage-hub.io/stiivo/devbox:v1
cage pull cage-hub.io/stiivo/devbox:v1 --platform arm64
```

The `--platform` flag (`amd64` or `arm64`, defaulting to the host architecture)
controls architecture dispatch:

- If the tag resolves to a multi-arch index, pull selects the entry matching the
  target architecture. If no entry matches, pull errors and lists the
  architectures the index does provide.
- If the tag resolves to a single manifest of a different architecture, pull
  errors with a hint to retry with `--platform <that arch>`.

### cage tag

Create or move a local tag, pointing it at the same manifest as an existing local ref.

```bash
cage tag <src-ref> <dst-ref>
```

Example:
```bash
cage tag cage-hub.io/stiivo/devbox:v1 cage-hub.io/stiivo/devbox:stable
cage push cage-hub.io/stiivo/devbox:stable
```

#### cage tag inspect

Inspect a tag and report whether it points at a single-arch `manifest` or a
multi-arch `index`, along with its digest and the architecture(s) it covers.
Works for both registry refs and local refs.

```bash
cage tag inspect <ref>
```

Example:
```bash
cage tag inspect cage-hub.io/stiivo/devbox:v1
Kind:          index
Digest:        sha256:fed987cba654
Architectures: amd64, arm64
```

---

### cage config

Manage cage configuration.

```bash
cage config <subcommand>
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `init` | Create default configuration |
| `show` | Display current configuration |
| `edit` | Open config in editor |
| `path` | Show config file path |

**Examples:**
```bash
# Initialize config
cage config init

# Force reinitialize
cage config init --force

# Show config
cage config show

# Edit config
cage config edit
```

---

### cage doctor

Check if all system requirements are met.

```bash
cage doctor [options]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--fix` | Show installation commands for missing dependencies |

**Examples:**
```bash
# Check requirements
cage doctor

# Show fix commands
cage doctor --fix
```

---

### cage verify

Run network isolation tests on a running cage.

```bash
cage verify <name>
```

Tests include:
- Internet access (should work)
- DNS resolution (should work)
- RFC 1918 subnets (should be blocked)
- Link-local addresses (should be blocked)

**Examples:**
```bash
cage verify myproject
```

---

## Configuration

### Project Configuration

Create a `.cage.yml` file in your project directory with `cage init` or manually. This file defines the cage for your project.

**File:** `.cage.yml` (in project root)

**Example:**
```yaml
# Cage configuration for this project
cage: my-project        # optional, defaults to directory name
image: ubuntu-24.04     # required, base image

# Resources (optional, overrides profile)
profile: default        # optional, uses global profile as base
memory: 4G              # optional, overrides profile
vcpu: 2                 # optional, overrides profile
disk: 20                # optional, disk size in GB

# Network
network:
  ssh: auto             # port number or "auto"
  ports:                # optional, additional port forwards
    - "8080:80"
    - "3000:3000"
  isolation: true       # optional, block LAN/private ranges (default true)
  allowed_subnets:      # optional, extra subnets reachable while isolated
    - 192.168.1.0/24

# File sharing
shares:
  - host: ./src         # relative to project directory
    guest: /home/cage/src
  - host: ./data
    guest: /data
    mode: ro            # optional, "ro" for read-only

# Environment variables (injected at start time)
env:
  NODE_ENV: development
  DEBUG: "true"
```

**Field Reference:**

| Field | Required | Description |
|-------|----------|-------------|
| `cage` | No | Cage name (default: directory name) |
| `image` | Yes | Base image (e.g., `ubuntu-24.04`, `alpine`) |
| `profile` | No | Global profile to use as base (`default`, `heavy`, `light`) |
| `memory` | No | Memory allocation (e.g., `4G`, `8G`) |
| `vcpu` | No | Number of virtual CPUs |
| `disk` | No | Disk size in GB |
| `network.ssh` | No | SSH port: `auto` or specific port number |
| `network.ports` | No | Port forwards in `host:guest` format |
| `network.isolation` | No | Block LAN/private ranges on the auto/SLIRP path (default `true`). Set `false` to let the cage reach the host LAN/private ranges (less secure). |
| `network.allowed_subnets` | No | CIDRs that stay reachable while isolation is on (routed via the SLIRP gateway `10.0.2.2`). Invalid CIDRs are rejected at `cage start`. Auto/SLIRP path only. |
| `shares` | No | Directory shares (host/guest/mode) |
| `env` | No | Environment variables |

### Global Configuration

Global config file: `~/.claude-cage/config.yaml`

This file defines default settings, resource profiles, and security configuration used across all cages.

### Global Configuration Reference

```yaml
# ~/.claude-cage/config.yaml

images:
  default: alpine           # Default image for new cages

profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    disk_gb: 20
  light:
    vcpu: 2
    memory_mb: 2048
    disk_gb: 10
  heavy:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50

shares:
  - host: ~/projects
    guest: /workspace
    mode: rw

network:
  dns:
    - 1.1.1.1
    - 8.8.8.8
  blocked_subnets:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
  port_bind: "127.0.0.1"

security:
  max_cages: 10
  virtiofsd_sandbox: true

env:
  MY_API_KEY: "secret-key"
  NODE_ENV: "development"
```

### Configuration Resolution

When `cage start` runs, configuration is resolved in this order:

1. **Global config** (`~/.claude-cage/config.yaml`) - provides defaults and profiles
2. **Project config** (`.cage.yml`) - specifies image, overrides resources
3. **Command line** - port forwarding flags

Project config `profile` field references a global profile, then `memory`/`vcpu`/`disk` override specific values.

### Environment Variables

Environment variables from `env` are injected at cage start time via virtiofs, allowing changes without recreating the cage. They are available in `/etc/profile.d/cage-runtime-env.sh`.

## Development

### Go Style & Best Practices

This project follows the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) — the most widely adopted community standard.

**Additional references:**
- [Effective Go](https://go.dev/doc/effective_go) — foundational Go idioms
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) — common review feedback

**Tooling (enforced in CI):**
```bash
# Format code (required - no style debates)
gofmt -w .

# Static analysis
go vet ./...

# Comprehensive linting (includes 50+ linters)
golangci-lint run ./...
```

**Key Principles:**
- Always check error returns (`errcheck`)
- Use `context.Context` for cancellation
- Prefer explicit over implicit
- Keep functions small and focused
- Document exported symbols

### Building & Testing

```bash
# Run tests with coverage
make test

# Run e2e tests (requires KVM)
make e2e-user  # user-mode networking (no root)
make e2e       # full tests (needs root for bridge)

# Lint (install: https://golangci-lint.run/usage/install/)
make lint

# Build
make build

# Clean
make clean
```

For detailed architecture, modules, and security documentation, see [Developer Documentation](docs/development/README.md).

## License

MIT

---

Made with [Claude](https://claude.ai)
