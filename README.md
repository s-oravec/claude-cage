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

# Download a base image
cage setup --base alpine-3.20

# Start a VM
cage start --name myvm --image alpine-3.20

# Connect via SSH
cage ssh myvm

# Execute commands
cage exec myvm -- uname -a

# Stop the VM
cage stop myvm
```

## Commands

| Command | Description |
|---------|-------------|
| `cage start` | Start a new VM |
| `cage stop` | Stop a running VM |
| `cage list` | List all VMs |
| `cage status` | Show VM status |
| `cage ssh` | Connect to VM via SSH |
| `cage exec` | Execute command in VM |
| `cage logs` | View VM console logs |
| `cage restart` | Restart a VM |
| `cage snapshot` | Manage VM snapshots |
| `cage port` | Manage port forwarding |
| `cage image` | Manage custom images |
| `cage setup` | Download base images |
| `cage config` | Manage configuration |
| `cage doctor` | Check system dependencies |

## Configuration

Configuration file: `~/.claude-cage/config.yaml`

```yaml
profiles:
  default:
    vcpu: 4
    memory_mb: 8192
  light:
    vcpu: 2
    memory_mb: 2048
  heavy:
    vcpu: 8
    memory_mb: 16384

shares:
  - host: ~/projects
    guest: /workspace

network:
  dns:
    - 8.8.8.8
    - 8.8.4.4
```

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
