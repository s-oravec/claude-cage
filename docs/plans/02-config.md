# Fáza 02: Config

Konfiguračný systém s YAML súborom.

## Cieľ

- `cage config show` - zobrazí konfiguráciu
- `cage config edit` - otvorí v editore
- `cage config path` - zobrazí cestu
- `cage config init` - vytvorí default config

## Závisí na

- Fáza 01 (CLI foundation)

## Implementácia

### 1. Config štruktúra

```go
// internal/config/config.go
type Config struct {
    Images   ImagesConfig   `yaml:"images"`
    Profiles ProfilesConfig `yaml:"profiles"`
    Network  NetworkConfig  `yaml:"network"`
    Shares   []ShareConfig  `yaml:"shares"`
    Security SecurityConfig `yaml:"security"`
}

type ProfilesConfig map[string]Profile

type Profile struct {
    VCPU         int `yaml:"vcpu"`
    MemoryMB     int `yaml:"memory_mb"`
    IOWeight     int `yaml:"io_weight"`
    MaxProcesses int `yaml:"max_processes"`
}

type NetworkConfig struct {
    BlockedInterfaces []string `yaml:"blocked_interfaces"`
    BlockedSubnets    []string `yaml:"blocked_subnets"`
    DNS               []string `yaml:"dns"`
    PortBind          string   `yaml:"port_bind"`
}

type ShareConfig struct {
    Host  string `yaml:"host"`
    Guest string `yaml:"guest"`
    Mode  string `yaml:"mode"` // rw, ro
}

type SecurityConfig struct {
    MaxCages        int  `yaml:"max_cages"`
    VirtiofsSandbox bool `yaml:"virtiofsd_sandbox"`
}
```

### 2. Default config

```yaml
# ~/.claude-cage/config.yaml
images:
  default: ubuntu-24.04

profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    io_weight: 500
    max_processes: 4096
  heavy:
    vcpu: 8
    memory_mb: 8192
    io_weight: 750
    max_processes: 8192
  light:
    vcpu: 2
    memory_mb: 2048
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
  port_bind: 127.0.0.1

shares:
  - host: ~/projects
    guest: /workspace

security:
  max_cages: 10
  virtiofsd_sandbox: true
```

### 3. Adresárová štruktúra

```
~/.claude-cage/
├── config.yaml
├── images/
├── keys/
├── cages/
└── known_hosts
```

### 4. Príkazy

```go
// cage config show
func showConfig(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    return yaml.NewEncoder(os.Stdout).Encode(cfg)
}

// cage config path
func configPath(cmd *cobra.Command, args []string) error {
    fmt.Println(config.Path())
    return nil
}

// cage config edit
func editConfig(cmd *cobra.Command, args []string) error {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }
    return exec.Command(editor, config.Path()).Run()
}

// cage config init
func initConfig(cmd *cobra.Command, args []string) error {
    if exists && !force {
        return errors.New("config exists, use --force")
    }
    return config.CreateDefault()
}
```

## Acceptance test

```bash
# Init
./cage config init
# ✓ Config created: ~/.claude-cage/config.yaml

# Show
./cage config show
# images:
#   default: ubuntu-24.04
# ...

# Path
./cage config path
# /home/user/.claude-cage/config.yaml

# Edit (opens editor)
./cage config edit
```

## Deliverables

- [x] Config struct with all fields
- [x] YAML loading/saving
- [x] Default config generation
- [x] `cage config show`
- [x] `cage config edit`
- [x] `cage config path`
- [x] `cage config init`
- [ ] Config validation (deferred - not critical for MVP)
