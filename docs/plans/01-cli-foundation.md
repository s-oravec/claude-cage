# Fáza 01: CLI Foundation

Základná štruktúra projektu a CLI príkazy `version` a `doctor`.

## Cieľ

- Funkčný Go projekt s cobra CLI
- `cage version` - zobrazí verziu
- `cage doctor` - skontroluje prerequisites

## Implementácia

### 1. Projekt setup

```bash
mkdir -p cage/cmd/cage
mkdir -p cage/internal/{config,libvirt,network,virtiofs,images,ssh}
cd cage
go mod init github.com/user/cage
go get github.com/spf13/cobra
go get github.com/spf13/viper
```

### 2. Štruktúra

```
cage/
├── cmd/cage/
│   └── main.go
├── internal/
│   ├── cmd/
│   │   ├── root.go
│   │   ├── version.go
│   │   └── doctor.go
│   └── doctor/
│       └── checks.go
├── go.mod
└── go.sum
```

### 3. Príkazy

**cage version:**
```go
// Výstup:
// cage version 0.1.0
// QEMU/KVM backend
// Config: ~/.claude-cage/config.yaml
```

**cage doctor:**
```go
// Kontroly:
// ✓ KVM available (/dev/kvm)
// ✓ libvirtd running
// ✓ User in kvm group
// ✓ User in libvirt group
// ✓ virtiofsd installed
// ✓ qemu-img installed
// ✗ cloud-localds not found (optional, for cloud-init)
```

### 4. Doctor checks

```go
type Check struct {
    Name     string
    Check    func() error
    Required bool
}

var checks = []Check{
    {"KVM available", checkKVM, true},
    {"libvirtd running", checkLibvirtd, true},
    {"User in kvm group", checkKvmGroup, true},
    {"User in libvirt group", checkLibvirtGroup, true},
    {"virtiofsd installed", checkVirtiofsd, true},
    {"qemu-img installed", checkQemuImg, true},
    {"cloud-localds installed", checkCloudLocalds, false},
}
```

## Acceptance test

```bash
# Build
go build -o cage ./cmd/cage

# Test
./cage version
# cage version 0.1.0
# QEMU/KVM backend

./cage doctor
# ✓ KVM available
# ✓ libvirtd running
# ...
```

## Deliverables

- [x] Go project initialized
- [x] cobra CLI setup
- [x] `cage version` command
- [x] `cage doctor` command with all checks
- [x] Makefile (build, test, install)
