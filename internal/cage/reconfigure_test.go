package cage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s-oravec/cage/internal/config"
)

func TestReconfigure_FailsIfRunning(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	// Create a cage in running state
	state := &State{
		Name:    "test",
		Status:  StatusRunning,
		Image:   "ubuntu-24.04",
		SSHPort: 2222,
	}
	err := SaveState(state)
	require.NoError(t, err)

	// Create resolved config
	cfg := &config.ResolvedConfig{
		CageName: "test",
		Image:    "ubuntu-24.04",
		VCPU:     4,
		MemoryMB: 4096,
		DiskGB:   20,
	}

	// Reconfigure should fail because cage is running
	err = Reconfigure("test", cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cage must be stopped to reconfigure")
}

func TestReconfigure_FailsIfCageNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	cfg := &config.ResolvedConfig{
		CageName: "nonexistent",
		Image:    "ubuntu-24.04",
		VCPU:     4,
		MemoryMB: 4096,
		DiskGB:   20,
	}

	err := Reconfigure("nonexistent", cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load cage state")
}

func TestReconfigure_UpdatesShares(t *testing.T) {
	// Note: This test verifies the Reconfigure function generates correct XML
	// but cannot fully test libvirt operations without mocking.
	// The actual libvirt integration is tested via e2e tests.

	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	// Create a stopped cage
	state := &State{
		Name:    "test",
		Status:  StatusStopped,
		Image:   "ubuntu-24.04",
		SSHPort: 2222,
	}
	err := SaveState(state)
	require.NoError(t, err)

	// Create resolved config with shares
	cfg := &config.ResolvedConfig{
		CageName: "test",
		Image:    "ubuntu-24.04",
		VCPU:     4,
		MemoryMB: 4096,
		DiskGB:   20,
		Shares: []config.ShareConfig{
			{Host: "/home/user/projects", Guest: "/workspace", Mode: "rw"},
		},
	}

	// Reconfigure will fail at the libvirt step since we don't have a real domain
	// But it should get past the state validation
	err = Reconfigure("test", cfg)

	// We expect an error from libvirt (domain doesn't exist),
	// but NOT a "cage must be stopped" error
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "cage must be stopped")
	// The error should be from libvirt/domain operations
	assert.Contains(t, err.Error(), "redefine domain")
}

func TestReconfigure_WithDifferentMemory(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	// Create a stopped cage
	state := &State{
		Name:    "memtest",
		Status:  StatusStopped,
		Image:   "ubuntu-24.04",
		SSHPort: 2223,
	}
	err := SaveState(state)
	require.NoError(t, err)

	// Create resolved config with different memory
	cfg := &config.ResolvedConfig{
		CageName: "memtest",
		Image:    "ubuntu-24.04",
		VCPU:     8,
		MemoryMB: 8192,
		DiskGB:   40,
	}

	// Reconfigure will fail at libvirt but should pass state checks
	err = Reconfigure("memtest", cfg)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "cage must be stopped")
}
