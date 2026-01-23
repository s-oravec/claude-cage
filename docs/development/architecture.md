# Architecture Overview

Claude Cage follows a layered architecture with clear separation of concerns. The application is built in Go and uses external tools (QEMU, libvirt, virtiofsd) for virtualization.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                │
│  cmd/cage/main.go → internal/cmd/*.go (cobra commands)          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Orchestration Layer                          │
│  Coordinates between subsystems: cage state, network, storage   │
│  internal/cmd/{create,start,stop,remove}.go                     │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   VM Layer      │ │  Network Layer  │ │  Storage Layer  │
│ internal/libvirt│ │ internal/network│ │ internal/images │
│ internal/virtiofs│                  │ │ internal/cage   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
          │                   │                   │
          ▼                   ▼                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                    External Tools Layer                          │
│  QEMU/KVM · libvirtd · virtiofsd · iptables · qemu-img · passt  │
└─────────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### CLI Layer
- Entry point: `cmd/cage/main.go`
- Command registration: `internal/cmd/root.go`
- User interaction and argument parsing via [cobra](https://github.com/spf13/cobra)

### Orchestration Layer
- Implements high-level operations (create, start, stop, remove)
- Coordinates multiple subsystems for each operation
- Handles error rollback (cleanup on partial failure)
- Located in `internal/cmd/{create,start,stop,remove}.go`

### VM Layer
- **libvirt** (`internal/libvirt/`) - Domain XML generation, VM lifecycle via virsh
- **virtiofs** (`internal/virtiofs/`) - Shared directory management via virtiofsd
- **cloudinit** (`internal/cloudinit/`) - Cloud-init ISO generation for VM bootstrap

### Network Layer
- **network** (`internal/network/`) - Network modes, firewall, port forwarding
- Supports two modes: `auto` (passt/slirp) and `bridge` (libvirt NAT with iptables)

### Storage Layer
- **images** (`internal/images/`) - Base image download and management
- **cage** (`internal/cage/`) - Per-cage state and disk overlay management
- **ssh** (`internal/ssh/`) - SSH key generation and connection

### Configuration Layer
- **config** (`internal/config/`) - Global and project-level configuration
- YAML-based with merge semantics for project overrides

## Key Design Decisions

### 1. libvirt over raw QEMU
Using libvirt provides:
- Consistent API across distributions
- Automatic cleanup on host reboot
- Domain definition persistence
- Network management integration

### 2. Copy-on-Write Disks
Each cage uses a qcow2 overlay on top of base images:
- Fast cage creation (no full disk copy)
- Base images remain immutable
- Easy snapshot and rollback

### 3. Cloud-init for Bootstrap
VMs are configured at first boot via cloud-init:
- User creation with SSH keys
- Environment variable injection
- Distro-agnostic package installation

### 4. Multiple Network Modes
- **auto**: User-mode networking (passt > slirp fallback), no root required
- **bridge**: NAT bridge with iptables isolation, requires root

### 5. State Files over Database
Each cage has a `state.json` in `~/.claude-cage/cages/<name>/`:
- Simple, inspectable, portable
- No database dependency
- Easy backup and restore

## Directory Structure

```
~/.claude-cage/
├── config.yaml          # Global configuration
├── images/              # Downloaded base images (*.qcow2)
├── keys/                # SSH keys per cage
│   └── <cage>/
│       ├── id_ed25519
│       └── id_ed25519.pub
├── cages/               # Per-cage data
│   └── <cage>/
│       ├── state.json   # Runtime state
│       ├── disk.qcow2   # CoW overlay
│       ├── cloud-init.iso
│       └── cloudinit/   # Cloud-init source files
└── known_hosts          # SSH known hosts
```

## External Dependencies

| Tool | Purpose | Required |
|------|---------|----------|
| QEMU/KVM | VM hypervisor | Yes |
| libvirt | VM management | Yes |
| qemu-img | Disk operations | Yes |
| virtiofsd | File sharing | No (optional) |
| cloud-localds | Cloud-init ISO | No (fallback available) |
| passt | Fast user-mode networking | No (slirp fallback) |
| iptables | Firewall rules | Only for bridge mode |

## See Also

- [Data Flow](data-flow.md) - Detailed request lifecycle
- [Modules Overview](modules.md) - Package documentation
- [Security Model](security.md) - Isolation architecture
