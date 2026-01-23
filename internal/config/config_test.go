package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Structure(t *testing.T) {
	cfg := Config{
		Images: ImagesConfig{Default: "alpine"},
		Profiles: map[string]Profile{
			"default": {VCPU: 4, MemoryMB: 4096},
		},
		Network: NetworkConfig{
			BlockedInterfaces: []string{"tun+"},
			BlockedSubnets:    []string{"10.0.0.0/8"},
			DNS:               []string{"1.1.1.1"},
			PortBind:          "127.0.0.1",
		},
		Shares: []ShareConfig{
			{Host: "~/projects", Guest: "/workspace", Mode: "rw"},
		},
		Security: SecurityConfig{
			MaxCages:        10,
			VirtiofsSandbox: true,
		},
	}

	assert.Equal(t, "alpine", cfg.Images.Default)
	assert.Equal(t, 4, cfg.Profiles["default"].VCPU)
	assert.Equal(t, "tun+", cfg.Network.BlockedInterfaces[0])
	assert.Equal(t, "/workspace", cfg.Shares[0].Guest)
	assert.True(t, cfg.Security.VirtiofsSandbox)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Images
	assert.Equal(t, "alpine", cfg.Images.Default)

	// Profiles
	assert.Contains(t, cfg.Profiles, "default")
	assert.Contains(t, cfg.Profiles, "heavy")
	assert.Contains(t, cfg.Profiles, "light")
	assert.Equal(t, 4, cfg.Profiles["default"].VCPU)
	assert.Equal(t, 8, cfg.Profiles["heavy"].VCPU)
	assert.Equal(t, 2, cfg.Profiles["light"].VCPU)

	// Network
	assert.Contains(t, cfg.Network.BlockedInterfaces, "tun+")
	assert.Contains(t, cfg.Network.BlockedInterfaces, "tailscale+")
	assert.Contains(t, cfg.Network.BlockedInterfaces, "wg+")
	assert.Contains(t, cfg.Network.BlockedSubnets, "10.0.0.0/8")
	assert.Contains(t, cfg.Network.BlockedSubnets, "172.16.0.0/12")
	assert.Contains(t, cfg.Network.BlockedSubnets, "192.168.0.0/16")
	assert.Equal(t, "127.0.0.1", cfg.Network.PortBind)

	// Security
	assert.Equal(t, 10, cfg.Security.MaxCages)
	assert.True(t, cfg.Security.VirtiofsSandbox)
}

func TestConfigDir(t *testing.T) {
	dir := Dir()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".claude-cage"), dir)
}

func TestConfigPath(t *testing.T) {
	path := Path()
	assert.Contains(t, path, ".claude-cage")
	assert.Contains(t, path, "config.yaml")
}

func TestSaveAndLoad(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	cfg := DefaultConfig()
	cfg.Images.Default = "test-image"

	// Save
	err := Save(cfg)
	require.NoError(t, err)

	// Load
	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "test-image", loaded.Images.Default)
	assert.Equal(t, cfg.Profiles["default"].VCPU, loaded.Profiles["default"].VCPU)
}

func TestLoad_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	// Should return error when file doesn't exist
	_, err := Load()
	assert.Error(t, err)
}

func TestCreateDefault(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	err := CreateDefault()
	require.NoError(t, err)

	// File should exist
	_, err = os.Stat(filepath.Join(tmpDir, "config.yaml"))
	assert.NoError(t, err)

	// Should be loadable
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "alpine", cfg.Images.Default)
}

func TestCreateDefault_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	// Create first
	err := CreateDefault()
	require.NoError(t, err)

	// Second create should fail
	err = CreateDefault()
	assert.Error(t, err)
}

func TestCreateDefault_Force(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	// Create first
	err := CreateDefault()
	require.NoError(t, err)

	// Force should work
	err = CreateDefaultForce()
	assert.NoError(t, err)
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := configDir
	configDir = tmpDir
	defer func() { configDir = oldDir }()

	assert.False(t, Exists())

	CreateDefault()

	assert.True(t, Exists())
}

func TestGetProfile(t *testing.T) {
	cfg := DefaultConfig()

	profile, err := cfg.GetProfile("default")
	require.NoError(t, err)
	assert.Equal(t, 4, profile.VCPU)

	profile, err = cfg.GetProfile("heavy")
	require.NoError(t, err)
	assert.Equal(t, 8, profile.VCPU)

	_, err = cfg.GetProfile("nonexistent")
	assert.Error(t, err)
}
