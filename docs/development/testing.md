# Testing Guide

This document describes how to run and write tests for Claude Cage.

## Test Categories

### Unit Tests

Test individual functions and packages in isolation.

**Location:** `internal/*_test.go`

**Run:**
```bash
make test
# or
go test ./...
```

### E2E Tests

Test full workflows with actual VMs.

**Location:** `test/e2e/e2e_test.go`

**Run:**
```bash
# User-mode networking only (no root)
make e2e-user

# Full tests including bridge mode (requires root)
make e2e
```

## Running Tests

### All Unit Tests

```bash
make test
```

### With Coverage

```bash
make test-coverage
```

Coverage report is generated at `coverage.out`.

View coverage in browser:
```bash
go tool cover -html=coverage.out
```

### Specific Package

```bash
go test ./internal/config/
go test ./internal/cmd/
```

### Specific Test

```bash
go test ./internal/config/ -run TestLoad
go test ./internal/cmd/ -run TestCreateCmd
```

### Verbose Output

```bash
go test -v ./internal/config/
```

## Writing Tests

### Unit Test Structure

```go
// internal/config/config_test.go
package config

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    // ... create test config

    // Execute
    cfg, err := Load()

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "alpine", cfg.Images.Default)
}
```

### Table-Driven Tests

```go
func TestResolveAlias(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"alpine alias", "alpine", "alpine-3.21"},
        {"ubuntu alias", "ubuntu", "ubuntu-24.04"},
        {"full name", "debian-11", "debian-11"},
        {"unknown", "unknown", "unknown"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ResolveAlias(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Mocking File System

Many packages support directory override for testing:

```go
func TestSaveState(t *testing.T) {
    // Override cages directory
    tmpDir := t.TempDir()
    old := cage.SetCagesDir(tmpDir)
    defer cage.SetCagesDir(old)

    // Now tests use tmpDir instead of ~/.claude-cage/cages/
    state := &cage.State{Name: "test", Status: "running"}
    err := cage.SaveState(state)
    assert.NoError(t, err)
}
```

### Testing Commands

Commands can be tested by capturing output:

```go
func TestListCmd(t *testing.T) {
    // Setup temp directory
    tmpDir := t.TempDir()
    cage.SetCagesDir(tmpDir)

    // Create test state
    state := &cage.State{Name: "test", Status: "stopped"}
    cage.SaveState(state)

    // Execute command
    cmd := NewListCmd()
    buf := new(bytes.Buffer)
    cmd.SetOut(buf)
    cmd.SetArgs([]string{})
    err := cmd.Execute()

    // Assert
    assert.NoError(t, err)
    assert.Contains(t, buf.String(), "test")
}
```

### Testing with Fixtures

For complex scenarios, use test fixtures:

```go
func TestLoadConfig(t *testing.T) {
    // Copy fixture to temp dir
    tmpDir := t.TempDir()
    fixture := `
images:
  default: ubuntu
profiles:
  default:
    vcpu: 4
`
    err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(fixture), 0644)
    require.NoError(t, err)

    // Override config dir
    config.SetDir(tmpDir)
    defer config.SetDir("")

    // Test
    cfg, err := config.Load()
    assert.NoError(t, err)
    assert.Equal(t, "ubuntu", cfg.Images.Default)
}
```

## E2E Tests

### Prerequisites

- KVM available
- libvirtd running
- Base image downloaded (`cage pull`)
- User in kvm/libvirt groups

### User-Mode E2E Tests

Test basic workflows without root:

```bash
make e2e-user
```

Tests included:
- Create cage with auto network
- Start/stop cage
- SSH connection
- Cage listing
- Cage removal

### Full E2E Tests

Test all features including bridge mode:

```bash
sudo make e2e
```

Additional tests:
- Bridge network creation
- Firewall rule verification
- Network isolation tests

### E2E Test Structure

```go
// test/e2e/e2e_test.go
func TestCreateStartStop(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }

    name := "e2e-test-" + randomString(6)

    // Create
    err := runCage("create", "-n", name, "--ssh", "auto")
    require.NoError(t, err)
    defer runCage("remove", name, "--force")

    // Start
    err = runCage("start", name)
    require.NoError(t, err)

    // Verify running
    state, err := cage.LoadState(name)
    require.NoError(t, err)
    assert.Equal(t, "running", state.Status)

    // Stop
    err = runCage("stop", name)
    require.NoError(t, err)
}
```

## Test Utilities

### Temporary Directories

Use `t.TempDir()` for automatic cleanup:

```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()  // Automatically cleaned up
    // ...
}
```

### Assertions

Use testify for readable assertions:

```go
import "github.com/stretchr/testify/assert"

assert.NoError(t, err)
assert.Equal(t, expected, actual)
assert.Contains(t, str, substring)
assert.True(t, condition)
assert.Nil(t, value)
```

For fatal assertions (stop test on failure):

```go
import "github.com/stretchr/testify/require"

require.NoError(t, err)  // Stops test if err != nil
```

### Skipping Tests

```go
func TestRequiresRoot(t *testing.T) {
    if os.Getuid() != 0 {
        t.Skip("requires root")
    }
    // ...
}

func TestRequiresKVM(t *testing.T) {
    if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
        t.Skip("requires KVM")
    }
    // ...
}
```

## Test Coverage

### Viewing Coverage

```bash
make test-coverage
go tool cover -html=coverage.out
```

### Coverage Goals

- **Core packages** (config, cage, network): > 80%
- **Commands**: > 70%
- **Utilities**: > 60%

### Untested Areas

Some code is difficult to unit test:
- External command execution (virsh, qemu-img)
- Process management (PIDs, signals)
- Network operations (iptables)

These are covered by E2E tests.

## Continuous Integration

### GitHub Actions Workflow

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.18'
      - run: make test
```

### CI Considerations

- E2E tests require VM support (nested virtualization)
- Some tests may be skipped in CI
- Use build tags for CI-specific tests

## Debugging Tests

### Verbose Output

```bash
go test -v ./internal/config/
```

### Print Statements

```go
func TestDebug(t *testing.T) {
    t.Logf("Debug value: %v", value)
    // ...
}
```

### Run Single Test

```bash
go test -v -run TestSpecificFunction ./internal/config/
```

### Preserve Test Output

```bash
CAGE_TEST_KEEP=1 go test ./test/e2e/
```

## Best Practices

1. **Isolate tests** - Don't depend on global state
2. **Clean up** - Use `defer` for cleanup operations
3. **Use temp directories** - Don't modify real config
4. **Test error paths** - Verify error handling
5. **Keep tests fast** - Mock slow operations
6. **Document test intent** - Clear test names

## See Also

- [Getting Started](getting-started.md) - Development setup
- [Modules Overview](modules.md) - Package documentation
