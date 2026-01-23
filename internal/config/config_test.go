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

func TestMerge_ScalarValues(t *testing.T) {
	base := DefaultConfig()
	other := &Config{
		Images: ImagesConfig{Default: "ubuntu"},
	}

	base.Merge(other)

	assert.Equal(t, "ubuntu", base.Images.Default)
}

func TestMerge_EnvMerge(t *testing.T) {
	base := DefaultConfig()
	base.Env = map[string]string{
		"EXISTING": "value",
		"OVERRIDE": "old",
	}

	other := &Config{
		Env: map[string]string{
			"OVERRIDE": "new",
			"NEW_VAR":  "added",
		},
	}

	base.Merge(other)

	assert.Equal(t, "value", base.Env["EXISTING"])
	assert.Equal(t, "new", base.Env["OVERRIDE"])
	assert.Equal(t, "added", base.Env["NEW_VAR"])
}

func TestMerge_ArraysReplace(t *testing.T) {
	base := DefaultConfig()
	other := &Config{
		Shares: []ShareConfig{
			{Host: "/project", Guest: "/code", Mode: "rw"},
		},
		Network: NetworkConfig{
			DNS: []string{"9.9.9.9"},
		},
	}

	base.Merge(other)

	// Shares replaced
	assert.Len(t, base.Shares, 1)
	assert.Equal(t, "/project", base.Shares[0].Host)

	// DNS replaced
	assert.Len(t, base.Network.DNS, 1)
	assert.Equal(t, "9.9.9.9", base.Network.DNS[0])
}

func TestMerge_ProfilesMerge(t *testing.T) {
	base := DefaultConfig()
	other := &Config{
		Profiles: map[string]Profile{
			"default": {VCPU: 16, MemoryMB: 16384, DiskGB: 100},
			"custom":  {VCPU: 1, MemoryMB: 512, DiskGB: 5},
		},
	}

	base.Merge(other)

	// default overridden
	assert.Equal(t, 16, base.Profiles["default"].VCPU)
	// custom added
	assert.Equal(t, 1, base.Profiles["custom"].VCPU)
	// heavy still exists
	assert.Equal(t, 8, base.Profiles["heavy"].VCPU)
}

func TestMerge_PartialConfig(t *testing.T) {
	base := DefaultConfig()
	// Partial config - only env
	other := &Config{
		Env: map[string]string{
			"MY_VAR": "value",
		},
	}

	base.Merge(other)

	// Env added
	assert.Equal(t, "value", base.Env["MY_VAR"])
	// Everything else unchanged
	assert.Equal(t, "alpine", base.Images.Default)
	assert.Equal(t, 4, base.Profiles["default"].VCPU)
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	// Save current dir
	oldDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	path := FindProjectConfig()
	assert.Empty(t, path)
}

func TestFindProjectConfig_Found(t *testing.T) {
	// Save current dir
	oldDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Create project config
	configPath := filepath.Join(tmpDir, ProjectConfigName)
	os.WriteFile(configPath, []byte("images:\n  default: ubuntu\n"), 0644)

	path := FindProjectConfig()
	assert.Equal(t, configPath, path)
}

func TestLoadProjectConfig(t *testing.T) {
	// Save current dir
	oldDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Create project config
	configPath := filepath.Join(tmpDir, ProjectConfigName)
	os.WriteFile(configPath, []byte(`
images:
  default: ubuntu
env:
  NODE_ENV: production
`), 0644)

	cfg, err := LoadProjectConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "ubuntu", cfg.Images.Default)
	assert.Equal(t, "production", cfg.Env["NODE_ENV"])
}

func TestLoadProjectConfig_NotFound(t *testing.T) {
	// Save current dir
	oldDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	cfg, err := LoadProjectConfig()
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_WithProjectConfig(t *testing.T) {
	// Setup global config dir
	globalDir := t.TempDir()
	oldConfigDir := configDir
	configDir = globalDir
	defer func() { configDir = oldConfigDir }()

	// Save global config
	globalCfg := DefaultConfig()
	globalCfg.Env = map[string]string{"GLOBAL": "yes"}
	Save(globalCfg)

	// Setup project dir
	oldDir, _ := os.Getwd()
	projectDir := t.TempDir()
	os.Chdir(projectDir)
	defer os.Chdir(oldDir)

	// Create project config
	os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
images:
  default: debian
env:
  PROJECT: yes
`), 0644)

	// Load should merge
	cfg, err := Load()
	require.NoError(t, err)

	// Project overrides
	assert.Equal(t, "debian", cfg.Images.Default)
	assert.Equal(t, "yes", cfg.Env["PROJECT"])
	// Global preserved
	assert.Equal(t, "yes", cfg.Env["GLOBAL"])
}
