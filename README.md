# Claude Cage

A lightweight QEMU/KVM-based VM sandbox CLI for running Claude Code in isolation.

## Features

- **VM Isolation**: Run Claude Code in a secure, isolated VM environment
- **QEMU/KVM Backend**: Uses libvirt for VM management
- **Copy-on-Write Disks**: Changes don't affect base images
- **File Sharing**: Share directories with the VM using virtio-fs
- **Port Forwarding**: Forward ports between host and VM
- **Snapshots**: Create and restore VM snapshots
- **Multiple Profiles**: Light, default, and heavy resource profiles
- **Multiple Distros**: Alpine, Ubuntu, Debian, Rocky, Alma, Fedora, openSUSE, CentOS

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

### Using Go

```bash
go install github.com/s-oravec/claude-cage/cmd/cage@latest
```

## Quick Start

```bash
# Initialize configuration
cage config init

# Download a base image (alpine is default)
cage setup

# Create a cage
cage create -n myvm

# Start the cage
cage start myvm

# Connect via SSH
cage ssh myvm

# Stop the cage (preserves resources)
cage stop myvm

# Remove the cage completely
cage remove myvm
```

## Commands

### Lifecycle Commands
- [`cage create`](#cage-create) - Create a new cage without starting it
- [`cage start`](#cage-start) - Start an existing cage
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

### Setup Commands
- [`cage setup`](#cage-setup) - Download base images
- [`cage config`](#cage-config) - Manage configuration
- [`cage doctor`](#cage-doctor) - Check system requirements
- [`cage verify`](#cage-verify) - Verify network isolation

---

## Command Reference

### cage create

Create a new cage VM without starting it. Creates disk overlay, network, SSH keys, and VM definition.

```bash
cage create -n <name> [options]
```

**Options:**
| Option | Description |
|--------|-------------|
| `-n, --name` | Name for the cage (required) |
| `-i, --image` | Base image (defaults to config default) |
| `-p, --profile` | Resource profile: `default`, `heavy`, `light` (default: `default`) |
| `--network` | Network mode: `auto`, `bridge` (default: `auto`) |

**Network Modes:**
| Mode | Root? | Speed | SSH | Description |
|------|-------|-------|-----|-------------|
| `auto` | No | Fast* | Console only | Auto-detect: passt > slirp (default) |
| `bridge` | Yes | Fast | Yes | Libvirt bridge with firewall isolation |

\* passt is fast, slirp fallback is slower

**Examples:**
```bash
# Create with default settings (auto network, no root needed)
cage create -n myproject

# Create with specific image and profile
cage create -n heavy-workload -i ubuntu-24.04 -p heavy

# Create with bridge network (requires root, enables SSH)
cage create -n isolated --network bridge
```

---

### cage start

Start a cage that was previously created.

```bash
cage start <name> [options]
```

**Options:**
| Option | Description |
|--------|-------------|
| `--port` | Port forwarding (e.g., `8080:80`), can be specified multiple times |

**Examples:**
```bash
# Start a cage
cage start myproject

# Start with port forwarding
cage start myproject --port 8080:80 --port 3000:3000
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
cage ssh <name> [command]
```

**Examples:**
```bash
# Interactive shell
cage ssh myproject

# Run a command
cage ssh myproject ls -la

# Run multiple commands
cage ssh myproject "cd /workspace && make build"
```

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

List all cages and their status.

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
| `list` | List snapshots |
| `restore` | Restore to a snapshot |
| `delete` | Delete a snapshot |

**Examples:**
```bash
# Create snapshot
cage snapshot create myproject --name before-update

# List snapshots
cage snapshot list myproject

# Restore snapshot
cage snapshot restore myproject --name before-update

# Delete snapshot
cage snapshot delete myproject --name before-update
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
| `list` | List port forwards |
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
| `list` | List available images |
| `save` | Save a cage as a new image |
| `delete` | Delete an image |
| `inspect` | Show image details |

**Examples:**
```bash
# List images
cage image list

# Save cage as image
cage image save myproject --name my-custom-image

# Delete image
cage image delete my-custom-image

# Inspect image
cage image inspect ubuntu-24.04
```

---

### cage setup

Download and prepare base images.

```bash
cage setup [options]
```

**Options:**
| Option | Description |
|--------|-------------|
| `-b, --base` | Base image to download |
| `-l, --list` | List available images |

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
| `rocky` / `rocky-9` | Rocky Linux 9 |
| `rocky-8` | Rocky Linux 8 |
| `alma` / `alma-9` | AlmaLinux 9 |
| `alma-8` | AlmaLinux 8 |
| `fedora` / `fedora-41` | Fedora 41 |
| `fedora-40` | Fedora 40 |
| `opensuse` / `opensuse-15.6` | openSUSE Leap 15.6 |
| `opensuse-15.5` | openSUSE Leap 15.5 |
| `centos` / `centos-stream-9` | CentOS Stream 9 |

**Examples:**
```bash
# Download default image (alpine)
cage setup

# List available images
cage setup --list

# Download specific image
cage setup --base ubuntu-24.04

# Use alias
cage setup --base ubuntu
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

### Global Configuration

Global config file: `~/.claude-cage/config.yaml`

### Project Configuration

You can create a `.claude-cage.yml` file in your project directory to override global settings. This is useful for project-specific environment variables, resource profiles, or port mappings.

**Lookup order:** `./.claude-cage.yml` → `~/.claude-cage/config.yaml`

**Merge behavior:**
- Scalar values (image, profile): project wins
- Maps (env, profiles): merged, project wins on conflicts
- Arrays (shares, dns, blocked_subnets): project replaces entirely

**Example `.claude-cage.yml`:**
```yaml
# Override default profile
profiles:
  default:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50

# Project-specific environment
env:
  NODE_ENV: development
  DATABASE_URL: postgres://localhost/myapp
  API_KEY: secret

# Custom shares for this project
shares:
  - host: .
    guest: /app
    mode: rw
```

### Full Configuration Reference

```yaml
images:
  default: alpine

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
  PATH_EXTRA: "/opt/custom/bin"
```

The `env` section defines environment variables that are injected into `/etc/profile.d/cage-env.sh` and available to all login shells in the cage.

## Development

```bash
# Run tests with coverage
make test

# Run e2e tests (requires KVM)
make e2e-user  # user-mode networking (no root)
make e2e       # full tests (needs root for bridge)

# Build
make build

# Clean
make clean
```

## License

MIT
