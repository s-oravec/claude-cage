# Modules Overview

This document describes the package structure and responsibilities of each module.

## Package Structure

```
cage/
├── cmd/cage/           # Application entry point
│   └── main.go
├── internal/           # Private packages
│   ├── cage/           # Cage state management
│   ├── cloudinit/      # Cloud-init generation
│   ├── cmd/            # CLI commands
│   ├── config/         # Configuration management
│   ├── doctor/         # System requirements checking
│   ├── images/         # Base image management
│   ├── libvirt/        # libvirt integration
│   ├── network/        # Network management
│   ├── progress/       # Progress bar utilities
│   ├── snapshot/       # Snapshot management
│   ├── ssh/            # SSH key management
│   └── virtiofs/       # File sharing daemon
└── test/e2e/           # End-to-end tests
```

## Module Details

### cmd/cage

Entry point that initializes and executes the root command.

```go
// main.go
func main() {
    rootCmd := cmd.NewRootCmd()
    rootCmd.Execute()
}
```

### internal/cage

Manages per-cage state and directory structure.

**Key Types:**
- `State` - Runtime state (name, status, network info, PIDs)
- `Port` - Port forwarding configuration
- `RestartConfig` - Configuration preserved across restarts

**Key Functions:**
- `SaveState()` / `LoadState()` - Persist/load state to JSON
- `Exists()` - Check if cage exists
- `List()` - List all cages
- `Dir()` - Get cage directory path

**File:** `internal/cage/state.go`

### internal/cloudinit

Generates cloud-init configuration for VM bootstrap.

**Key Types:**
- `CloudInitConfig` - Configuration for cloud-init generation

**Key Functions:**
- `GenerateUserDataWithConfig()` - Generate user-data YAML
- `GenerateMetaData()` - Generate meta-data
- `GenerateISOWithConfig()` - Create bootable cloud-init ISO

**Features:**
- User creation with sudo access
- SSH key injection
- Environment variable setup
- Distro-agnostic package installation (apk/apt/dnf/zypper)
- Optional virtiofs mount configuration
- Optional SSH server installation

**File:** `internal/cloudinit/generate.go`

### internal/cmd

CLI command implementations using [cobra](https://github.com/spf13/cobra).

See [CLI Commands](modules-cmd.md) for detailed documentation.

### internal/config

Configuration loading, merging, and persistence.

**Key Types:**
- `Config` - Root configuration structure
- `Profile` - Resource profile (CPU, memory, disk)
- `NetworkConfig` - Network settings
- `ShareConfig` - Host-guest share mapping
- `SecurityConfig` - Security settings

**Key Functions:**
- `Load()` - Load and merge global + project config
- `LoadGlobal()` - Load only global config
- `Save()` - Persist configuration
- `Merge()` - Merge two configurations

**File:** `internal/config/config.go`

### internal/doctor

System requirements verification.

**Key Types:**
- `Check` - Individual requirement check
- `CheckResult` - Result of running a check
- `Distro` - Detected Linux distribution

**Key Functions:**
- `DefaultChecks()` - Get standard checks list
- `RunChecks()` - Execute all checks
- `DetectDistro()` - Detect Linux distribution
- `InstallAllHint()` - Get installation command for all dependencies

**Checks Performed:**
- KVM availability (`/dev/kvm`)
- libvirtd service running
- User in kvm/libvirt groups
- Tool availability (qemu-img, virtiofsd, etc.)

**File:** `internal/doctor/checks.go`

### internal/images

Base image download and management.

**Key Types:**
- `ImageSource` - Image metadata (URL, format)
- `ProgressWriter` - Download progress tracking

**Key Functions:**
- `Download()` - Download image from URL
- `Setup()` - Download and prepare image
- `ImagePath()` - Get path to image file
- `IsDownloaded()` - Check if image exists
- `ListDownloaded()` - List available images
- `ResolveAlias()` - Resolve image alias to canonical name

**Supported Images:**
- Alpine Linux (3.20, 3.21)
- Ubuntu (20.04, 22.04, 24.04)
- Debian (11, 12)
- Rocky Linux (8, 9)
- AlmaLinux (8, 9)
- Fedora (40, 41)
- openSUSE Leap (15.5, 15.6)
- CentOS Stream 9

**Files:** `internal/images/manager.go`, `internal/images/sources.go`, `internal/images/types.go`

### internal/libvirt

libvirt domain management via virsh.

**Key Types:**
- `DomainConfig` - VM configuration
- `Client` - virsh command wrapper

**Key Functions:**
- `GenerateDomainXML()` - Generate libvirt domain XML
- `DefineDomain()` - Define VM in libvirt
- `StartDomain()` / `StopDomain()` - VM lifecycle
- `DomainExists()` - Check if domain exists

**File:** `internal/libvirt/domain.go`, `internal/libvirt/client.go`

### internal/network

Network configuration and isolation.

**Key Types:**
- `NetworkConfig` - libvirt network configuration
- `FirewallConfig` - iptables configuration
- `VerificationResult` - Network isolation test result

**Key Functions:**
- `CreateNetwork()` / `DestroyNetwork()` - libvirt network management
- `SetupFirewall()` / `CleanupFirewall()` - iptables rules
- `GenerateFirewallRules()` - Generate iptables rule set
- `VerifyIsolation()` - Run network isolation tests
- `FindFreePort()` - Find available TCP port

**Files:** `internal/network/libvirt.go`, `internal/network/firewall.go`, `internal/network/verify.go`, `internal/network/port.go`

### internal/progress

Terminal progress bar for downloads.

**Key Functions:**
- `NewBar()` - Create progress bar
- `Update()` - Update progress
- `Complete()` - Mark complete

**File:** `internal/progress/bar.go`

### internal/snapshot

VM snapshot management.

**Key Functions:**
- `Create()` - Create snapshot
- `Restore()` - Restore to snapshot
- `Delete()` - Delete snapshot
- `List()` - List snapshots

**File:** `internal/snapshot/snapshot.go`

### internal/ssh

SSH key generation and connection.

**Key Functions:**
- `GenerateKeyPair()` - Generate Ed25519 key pair
- `GetPublicKey()` - Read public key
- `DeleteKeys()` - Remove key pair
- `KeyPath()` / `PubKeyPath()` - Get key file paths

**File:** `internal/ssh/keygen.go`, `internal/ssh/connect.go`

### internal/virtiofs

virtiofsd daemon management for file sharing.

**Key Types:**
- `DaemonConfig` - virtiofsd configuration
- `Daemon` - Running daemon instance

**Key Functions:**
- `Start()` - Start virtiofsd daemon
- `Stop()` - Stop daemon and cleanup
- `FindVirtiofsd()` - Locate virtiofsd binary
- `SocketPath()` - Get socket file path

**File:** `internal/virtiofs/daemon.go`

## Dependency Graph

```
cmd/cage
    └── internal/cmd
            ├── internal/cage
            ├── internal/cloudinit
            ├── internal/config
            ├── internal/doctor
            ├── internal/images
            ├── internal/libvirt
            ├── internal/network
            ├── internal/progress
            ├── internal/snapshot
            ├── internal/ssh
            └── internal/virtiofs
```

## External Dependencies

| Package | Purpose |
|---------|---------|
| github.com/spf13/cobra | CLI framework |
| gopkg.in/yaml.v3 | YAML parsing |
| github.com/stretchr/testify | Test assertions |

## See Also

- [CLI Commands](modules-cmd.md) - Command implementations
- [Architecture Overview](architecture.md) - System design
