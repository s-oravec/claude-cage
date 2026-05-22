# Configuration Models

This document describes the configuration data structures and YAML schema.

## Configuration Files

| File | Purpose | Priority |
|------|---------|----------|
| `~/.cage/config.yaml` | Global configuration | Base |
| `./.cage.yml` | Project-specific overrides | Higher |

## Config Structure

```go
// internal/config/config.go

type Config struct {
    Images   ImagesConfig       `yaml:"images"`
    Profiles map[string]Profile `yaml:"profiles"`
    Network  NetworkConfig      `yaml:"network"`
    Shares   []ShareConfig      `yaml:"shares"`
    Security SecurityConfig     `yaml:"security"`
    Env      map[string]string  `yaml:"env,omitempty"`
}
```

### ImagesConfig

Default image selection.

```go
type ImagesConfig struct {
    Default string `yaml:"default"`
}
```

**YAML:**
```yaml
images:
  default: alpine  # Can be alias (alpine) or full name (alpine-3.21)
```

### Profile

Resource allocation for cages.

```go
type Profile struct {
    VCPU         int `yaml:"vcpu"`
    MemoryMB     int `yaml:"memory_mb"`
    DiskGB       int `yaml:"disk_gb"`
    IOWeight     int `yaml:"io_weight"`
    MaxProcesses int `yaml:"max_processes"`
}
```

**Default Profiles:**
```yaml
profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    disk_gb: 20
    io_weight: 500
    max_processes: 4096
  heavy:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50
    io_weight: 750
    max_processes: 8192
  light:
    vcpu: 2
    memory_mb: 2048
    disk_gb: 10
    io_weight: 250
    max_processes: 2048
```

### NetworkConfig

Network isolation settings.

```go
type NetworkConfig struct {
    BlockedInterfaces []string `yaml:"blocked_interfaces"`
    BlockedSubnets    []string `yaml:"blocked_subnets"`
    DNS               []string `yaml:"dns"`
    PortBind          string   `yaml:"port_bind"`
}
```

**Default Values:**
```yaml
network:
  blocked_interfaces:
    - tun+        # OpenVPN, generic TUN
    - tailscale+  # Tailscale
    - wg+         # WireGuard
  blocked_subnets:
    - 10.0.0.0/8       # RFC 1918 Class A
    - 172.16.0.0/12    # RFC 1918 Class B
    - 192.168.0.0/16   # RFC 1918 Class C
    - 169.254.0.0/16   # Link-local
  dns:
    - 1.1.1.1    # Cloudflare
    - 8.8.8.8    # Google
  port_bind: "127.0.0.1"
```

### ShareConfig

Host-to-guest directory mapping.

```go
type ShareConfig struct {
    Host  string `yaml:"host"`
    Guest string `yaml:"guest"`
    Mode  string `yaml:"mode"`  // "rw" or "ro"
}
```

**Example:**
```yaml
shares:
  - host: ~/projects
    guest: /workspace
    mode: rw
  - host: ~/shared-data
    guest: /data
    mode: ro
```

### SecurityConfig

Security-related settings.

```go
type SecurityConfig struct {
    MaxCages        int  `yaml:"max_cages"`
    VirtiofsSandbox bool `yaml:"virtiofsd_sandbox"`
}
```

**Example:**
```yaml
security:
  max_cages: 10
  virtiofsd_sandbox: true
```

### Environment Variables

Environment variables injected into the cage.

```yaml
env:
  NODE_ENV: development
  API_KEY: secret-key
  PATH_EXTRA: /opt/custom/bin
```

These are written to `/etc/profile.d/cage-env.sh` in the VM via cloud-init.

## Full Configuration Example

```yaml
# ~/.cage/config.yaml

images:
  default: ubuntu

profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    disk_gb: 20
    io_weight: 500
    max_processes: 4096
  heavy:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50
    io_weight: 750
    max_processes: 8192
  light:
    vcpu: 2
    memory_mb: 2048
    disk_gb: 10
    io_weight: 250
    max_processes: 2048

network:
  blocked_interfaces:
    - tun+
    - tailscale+
    - wg+
  blocked_subnets:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 169.254.0.0/16
  dns:
    - 1.1.1.1
    - 8.8.8.8
  port_bind: "127.0.0.1"

shares:
  - host: ~/projects
    guest: /workspace
    mode: rw

security:
  max_cages: 10
  virtiofsd_sandbox: true

env:
  EDITOR: vim
```

## Project Configuration Example

```yaml
# ./.cage.yml (in project directory)

# Override default profile for this project
profiles:
  default:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50

# Project-specific environment
env:
  NODE_ENV: development
  DATABASE_URL: postgres://localhost/myapp
  API_KEY: project-specific-key

# Custom shares for this project
shares:
  - host: .
    guest: /app
    mode: rw
```

## Configuration Merge Rules

When project config exists, it's merged with global config:

| Field Type | Merge Behavior |
|------------|----------------|
| Scalar (string, int) | Project wins if set |
| Map (profiles, env) | Deep merge, project wins on conflict |
| Array (shares, dns) | Project replaces entirely |

**Example Merge:**

Global:
```yaml
profiles:
  default: {vcpu: 4}
  heavy: {vcpu: 8}
env:
  EDITOR: vim
```

Project:
```yaml
profiles:
  default: {vcpu: 16}
env:
  NODE_ENV: dev
```

Result:
```yaml
profiles:
  default: {vcpu: 16}  # Project wins
  heavy: {vcpu: 8}     # Preserved from global
env:
  EDITOR: vim          # Preserved from global
  NODE_ENV: dev        # Added from project
```

## Validation

Configuration is validated at load time:

- Profile names must match when referenced
- Image aliases are resolved to canonical names
- Disk sizes must be positive
- Memory must be at least 512MB

## See Also

- [Runtime Models](models-runtime.md) - State structures
- [Architecture Overview](architecture.md) - System design
