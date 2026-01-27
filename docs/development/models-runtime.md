# Runtime Models

This document describes the runtime state structures used during cage lifecycle.

## Cage State

The primary runtime model is `State`, persisted in `~/.claude-cage/cages/<name>/state.json`.

```go
// internal/cage/state.go

type State struct {
    Name         string    `json:"name"`
    Status       string    `json:"status"`
    Image        string    `json:"image"`
    Profile      string    `json:"profile"`
    NetworkMode  string    `json:"network_mode,omitempty"`
    SSHPort      int       `json:"ssh_port,omitempty"`
    IP           string    `json:"ip,omitempty"`
    Ports        []Port    `json:"ports,omitempty"`
    StartedAt    time.Time `json:"started_at"`
    VirtiofsPID  int       `json:"virtiofs_pid,omitempty"`
    ForwarderPID int       `json:"forwarder_pid,omitempty"`
}
```

### State Fields

| Field | Description |
|-------|-------------|
| `Name` | Unique cage identifier |
| `Status` | Current status: `running` or `stopped` |
| `Image` | Base image used (canonical name) |
| `Profile` | Resource profile name |
| `NetworkMode` | Network mode: `auto` or `bridge` |
| `SSHPort` | Host port for SSH (auto mode) |
| `IP` | VM IP address (bridge mode) |
| `Ports` | Port forwarding rules |
| `StartedAt` | Timestamp when started |
| `VirtiofsPID` | virtiofsd process ID |
| `ForwarderPID` | Port forwarder process ID |

### Status Constants

```go
const (
    StatusRunning = "running"
    StatusStopped = "stopped"
)
```

### Network Mode Constants

```go
const (
    NetworkAuto   = "auto"   // SLIRP user-mode (no root)
    NetworkBridge = "bridge" // libvirt NAT (requires root)
)
```

## Port Model

Represents a port forwarding rule.

```go
type Port struct {
    Host         int    `json:"host"`
    Guest        int    `json:"guest"`
    Protocol     string `json:"protocol"`
    Bind         string `json:"bind,omitempty"`
    ForwarderPID int    `json:"forwarder_pid,omitempty"`
}
```

**Example:**
```json
{
    "host": 8080,
    "guest": 80,
    "protocol": "tcp",
    "bind": "127.0.0.1",
    "forwarder_pid": 12345
}
```

## Restart Configuration

Preserved during stop/start cycles for consistent restarts.

```go
type RestartConfig struct {
    Image   string `json:"image"`
    Profile string `json:"profile"`
    Ports   []Port `json:"ports,omitempty"`
}
```

Stored in `~/.claude-cage/cages/<name>/restart.json`.

## State File Example

Complete `state.json` example:

```json
{
    "name": "myproject",
    "status": "running",
    "image": "ubuntu-24.04",
    "profile": "default",
    "network_mode": "auto",
    "ssh_port": 52341,
    "ports": [
        {
            "host": 8080,
            "guest": 80,
            "protocol": "tcp",
            "bind": "127.0.0.1",
            "forwarder_pid": 23456
        }
    ],
    "started_at": "2024-01-15T10:30:00Z",
    "virtiofs_pid": 12345,
    "forwarder_pid": 0
}
```

## libvirt Domain Configuration

Used to generate XML for VM definition.

```go
// internal/libvirt/domain.go

type DomainConfig struct {
    Name           string
    MemoryMB       int
    VCPU           int
    DiskPath       string
    CloudInitISO   string
    NetworkName    string // Empty for user-mode networking
    VirtiofsSocket string // Optional virtiofs socket path
    SSHPort        int    // Port for SSH forwarding (user-mode only)
}
```

**Generated XML elements:**
- Domain type (kvm)
- CPU (host-passthrough mode)
- Memory allocation
- VirtIO disk (qcow2 overlay)
- Cloud-init CDROM
- Network interface (bridge or user-mode)
- Optional virtiofs filesystem
- Serial console
- RNG device

## Cloud-init Configuration

Configuration for cloud-init ISO generation.

```go
// internal/cloudinit/generate.go

type CloudInitConfig struct {
    CageName      string
    PubKey        string
    MountVirtiofs bool
    Env           map[string]string
    InstallSSH    bool
}
```

**Generated user-data includes:**
- User `cage` with sudo privileges
- SSH authorized key
- Password hash for console access
- Root partition growth
- Docker setup (systemd/openrc)
- Environment variable injection
- Optional virtiofs mount
- Optional SSH server installation

## Network Configuration (Bridge Mode)

```go
// internal/network/libvirt.go

type NetworkConfig struct {
    CageName   string
    BridgeName string
    IPAddress  string
    Netmask    string
    DHCPStart  string
    DHCPEnd    string
}
```

**Defaults:**
- Bridge: `cage-<name>` (max 15 chars)
- IP: `192.168.100.1/24`
- DHCP range: `192.168.100.2` - `192.168.100.254`

## Firewall Configuration

```go
// internal/network/firewall.go

type FirewallConfig struct {
    BridgeName        string
    BlockedInterfaces []string
    BlockedSubnets    []string
    AllowedDNS        []string
}
```

**Generated iptables rules:**
1. Allow established connections
2. Block VPN interfaces (tun+, wg+, tailscale+)
3. Block RFC 1918 subnets
4. Allow DNS only to configured servers
5. Allow HTTP/HTTPS
6. Accept remaining traffic

## virtiofs Daemon Configuration

```go
// internal/virtiofs/daemon.go

type DaemonConfig struct {
    CageName  string
    SharedDir string
    Sandbox   bool // --sandbox=chroot (root only)
    Seccomp   bool // --seccomp=kill (root only)
}

type Daemon struct {
    CageName   string
    SocketPath string
    SharedDir  string
    PID        int
    cmd        *exec.Cmd
}
```

## State Persistence

State operations:

```go
// Save state to disk
func SaveState(state *State) error

// Load state from disk
func LoadState(name string) (*State, error)

// Delete state and directory
func DeleteState(name string) error

// List all cages
func List() ([]*State, error)

// Check if cage exists
func Exists(name string) bool
```

## File Locations

| Content | Path |
|---------|------|
| Cage state | `~/.claude-cage/cages/<name>/state.json` |
| Restart config | `~/.claude-cage/cages/<name>/restart.json` |
| Disk overlay | `~/.claude-cage/cages/<name>/disk.qcow2` |
| Cloud-init ISO | `~/.claude-cage/cages/<name>/cloud-init.iso` |
| SSH keys | `~/.claude-cage/keys/<name>/id_ed25519[.pub]` |
| virtiofs socket | `/tmp/cage-virtiofs/<name>/virtiofs.sock` |

## See Also

- [Configuration Models](models-config.md) - Config structures
- [Data Flow](data-flow.md) - Lifecycle operations
