# Getting Started

This guide helps developers set up their environment for contributing to Claude Cage.

## Prerequisites

### Required

- **Go 1.18+** - Programming language
- **Linux with KVM** - Hypervisor support
- **libvirt** - VM management
- **QEMU** - Virtualization backend

### Optional

- **virtiofsd** - File sharing (recommended)
- **cloud-localds** - Cloud-init ISO generation

## Quick Setup

### 1. Clone Repository

```bash
git clone https://github.com/s-oravec/claude-cage.git
cd claude-cage
```

### 2. Install Dependencies

Check what's missing:
```bash
make build
./cage doctor
```

Install all dependencies (Ubuntu/Debian):
```bash
sudo apt install -y \
    qemu-kvm \
    libvirt-daemon-system \
    libvirt-clients \
    virtiofsd \
    qemu-utils \
    cloud-image-utils

# Add user to required groups
sudo usermod -aG kvm,libvirt $USER

# Enable libvirtd
sudo systemctl enable --now libvirtd

# Apply group changes (or logout/login)
newgrp libvirt
```

For other distributions, see `cage doctor --fix` for specific commands.

### 3. Build

```bash
make build
```

Binary is created at `./cage`.

### 4. Verify Setup

```bash
./cage doctor
```

All checks should pass (virtiofsd is optional but recommended).

## Project Structure

```
claude-cage/
├── cmd/cage/           # Entry point
│   └── main.go
├── internal/           # Implementation packages
│   ├── cage/           # State management
│   ├── cloudinit/      # Cloud-init generation
│   ├── cmd/            # CLI commands
│   ├── config/         # Configuration
│   ├── doctor/         # System checks
│   ├── images/         # Image management
│   ├── libvirt/        # libvirt integration
│   ├── network/        # Network management
│   ├── progress/       # Progress bar
│   ├── snapshot/       # Snapshots
│   ├── ssh/            # SSH management
│   └── virtiofs/       # File sharing
├── test/e2e/           # E2E tests
├── docs/               # Documentation
├── Makefile            # Build tasks
└── go.mod              # Go modules
```

## Development Workflow

### Building

```bash
# Build binary
make build

# Clean build artifacts
make clean
```

### Running Tests

```bash
# Unit tests
make test

# Unit tests with coverage
make test-coverage

# E2E tests (user mode, no root)
make e2e-user

# Full E2E tests (requires root for bridge mode)
make e2e
```

### Manual Testing

```bash
# Initialize config
./cage config init

# Download a base image
./cage pull --base alpine

# Test with project config workflow
mkdir /tmp/test-cage && cd /tmp/test-cage
./cage init --image alpine
./cage start
./cage ssh
./cage stop
./cage remove
cd ~ && rm -rf /tmp/test-cage

# Or test with explicit cage name
./cage start test-direct  # Will fail without project config
```

## Code Style

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Package names are lowercase, single-word
- Exported functions have doc comments

### Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Cleanup on error (rollback partial operations)

### Testing

- Table-driven tests preferred
- Use `testify/assert` for assertions
- Mock external commands where possible
- Test both success and error paths

## Adding a New Command

1. Create `internal/cmd/newcmd.go`:
```go
package cmd

import "github.com/spf13/cobra"

func NewMyCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand",
        Short: "Brief description",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runMyCommand(cmd, args)
        },
    }
    // Add flags
    return cmd
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}
```

2. Register in `internal/cmd/root.go`:
```go
rootCmd.AddCommand(NewMyCmd())
```

3. Create tests in `internal/cmd/newcmd_test.go`

## Adding a New Package

1. Create directory under `internal/`
2. Add package with clear responsibility
3. Export only necessary functions/types
4. Add tests in `*_test.go`
5. Document in `docs/development/modules.md`

## Common Tasks

### Debugging VM Issues

```bash
# View libvirt logs
journalctl -u libvirtd

# Check domain status
virsh -c qemu:///system list --all

# View domain XML
virsh -c qemu:///system dumpxml cage-<name>

# Console access
virsh -c qemu:///system console cage-<name>
```

### Testing Network Isolation

```bash
# Create a test directory with project config
mkdir /tmp/test-net && cd /tmp/test-net
cat > .cage.yml << 'EOF'
image: alpine
network:
  ssh: auto
EOF

./cage start
./cage verify
./cage remove --force
cd ~ && rm -rf /tmp/test-net
```

### Testing File Sharing

```bash
# Create project with custom shares
mkdir /tmp/test-share && cd /tmp/test-share
mkdir -p src data
cat > .cage.yml << 'EOF'
image: alpine
network:
  ssh: auto
shares:
  - host: ./src
    guest: /home/cage/src
  - host: ./data
    guest: /data
    mode: ro
EOF

./cage start
./cage ssh ls /home/cage/src
./cage remove --force
cd ~ && rm -rf /tmp/test-share
```

## IDE Setup

### VS Code

Recommended extensions:
- Go (golang.go)
- YAML (redhat.vscode-yaml)

`.vscode/settings.json`:
```json
{
    "go.useLanguageServer": true,
    "go.testOnSave": true,
    "go.coverOnSave": true
}
```

### GoLand

- Enable Go modules integration
- Set GOROOT to Go installation
- Configure test runner for coverage

## Troubleshooting

### "Permission denied" on /dev/kvm

```bash
sudo usermod -aG kvm $USER
# Logout and login, or:
newgrp kvm
```

### libvirtd not running

```bash
sudo systemctl enable --now libvirtd
```

### virsh commands fail

Ensure using the system connection (cage requires it):
```bash
virsh -c qemu:///system list
```

### Tests fail with "cage already exists"

Clean up leftover test cages:
```bash
./cage remove --all --force
```

## See Also

- [Testing Guide](testing.md) - Detailed testing documentation
- [Architecture Overview](architecture.md) - System design
- [Modules Overview](modules.md) - Package documentation
