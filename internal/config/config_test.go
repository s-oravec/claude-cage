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

func TestLoadProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a full project config
	configPath := filepath.Join(tmpDir, ProjectConfigFile)
	configContent := `
cage: my-project
image: ubuntu:22.04
profile: heavy
memory: 8G
vcpu: 4
disk: 50
network:
  ssh: "2222"
  ports:
    - "8080:80"
    - "3000:3000"
shares:
  - host: ~/code
    guest: /workspace
    mode: rw
env:
  NODE_ENV: production
  DEBUG: "true"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "my-project", cfg.Cage)
	assert.Equal(t, "ubuntu:22.04", cfg.Image)
	assert.Equal(t, "heavy", cfg.Profile)
	assert.Equal(t, "8G", cfg.Memory)
	assert.Equal(t, 4, cfg.VCPU)
	assert.Equal(t, 50, cfg.DiskGB)
	assert.Equal(t, "2222", cfg.Network.SSH)
	assert.Equal(t, []string{"8080:80", "3000:3000"}, cfg.Network.Ports)
	require.Len(t, cfg.Shares, 1)
	assert.Equal(t, "~/code", cfg.Shares[0].Host)
	assert.Equal(t, "/workspace", cfg.Shares[0].Guest)
	assert.Equal(t, "rw", cfg.Shares[0].Mode)
	assert.Equal(t, "production", cfg.Env["NODE_ENV"])
	assert.Equal(t, "true", cfg.Env["DEBUG"])
}

func TestLoadProjectConfig_CageNameFromDir(t *testing.T) {
	// Create a temp dir with a specific name
	parentDir := t.TempDir()
	projectDir := filepath.Join(parentDir, "awesome-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create minimal config without cage name
	configPath := filepath.Join(projectDir, ProjectConfigFile)
	configContent := `image: alpine:latest`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadProjectConfig(projectDir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Cage name should default to directory name
	assert.Equal(t, "awesome-project", cfg.Cage)
	assert.Equal(t, "alpine:latest", cfg.Image)
}

func TestLoadProjectConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadProjectConfig(tmpDir)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), ProjectConfigFile)
}

func TestLoadProjectConfig_MissingImage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config without image field
	configPath := filepath.Join(tmpDir, ProjectConfigFile)
	configContent := `
cage: my-project
memory: 4G
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadProjectConfig(tmpDir)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "image")
}

func TestProjectConfigExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not exist initially
	assert.False(t, ProjectConfigExists(tmpDir))

	// Create config file
	configPath := filepath.Join(tmpDir, ProjectConfigFile)
	err := os.WriteFile(configPath, []byte("image: alpine"), 0644)
	require.NoError(t, err)

	// Should exist now
	assert.True(t, ProjectConfigExists(tmpDir))
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"4G", 4096, false},
		{"8G", 8192, false},
		{"512M", 512, false},
		{"1024M", 1024, false},
		{"2048", 2048, false},
		{"4g", 4096, false},  // lowercase
		{" 8G ", 8192, false}, // with spaces
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseMemory(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParsePortMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected PortMapping
		hasError bool
	}{
		{"8080:80", PortMapping{Host: 8080, Guest: 80}, false},
		{"3000:3000", PortMapping{Host: 3000, Guest: 3000}, false},
		{"22:22", PortMapping{Host: 22, Guest: 22}, false},
		{"invalid", PortMapping{}, true},
		{"8080", PortMapping{}, true},
		{"abc:80", PortMapping{}, true},
		{"8080:abc", PortMapping{}, true},
		{"8080:80:extra", PortMapping{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParsePortMapping(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolveProjectConfig(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:    "my-project",
		Image:   "ubuntu-24.04",
		Profile: "default",
		Memory:  "8G", // override profile
		Network: ProjectNetwork{
			SSH:   "auto",
			Ports: []string{"8080:80"},
		},
		Shares: []ShareConfig{
			{Host: "./src", Guest: "/home/cage/src", Mode: "rw"},
		},
		Env: map[string]string{"NODE_ENV": "dev"},
	}

	resolved, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/myproject")
	require.NoError(t, err)

	assert.Equal(t, "my-project", resolved.CageName)
	assert.Equal(t, "ubuntu-24.04", resolved.Image)
	assert.Equal(t, 4, resolved.VCPU)           // from default profile
	assert.Equal(t, 8192, resolved.MemoryMB)    // overridden (8G = 8192MB)
	assert.Equal(t, 20, resolved.DiskGB)        // from default profile
	assert.Equal(t, "auto", resolved.SSHPort)
	assert.Len(t, resolved.Ports, 1)
	assert.Equal(t, 8080, resolved.Ports[0].Host)
	assert.Equal(t, 80, resolved.Ports[0].Guest)
	assert.Len(t, resolved.Shares, 1)
	// Share host path should be absolute
	assert.Equal(t, "/home/user/myproject/src", resolved.Shares[0].Host)
	assert.Equal(t, "dev", resolved.Env["NODE_ENV"])
	// ImagePath should be set
	assert.Contains(t, resolved.ImagePath, "ubuntu-24.04.qcow2")
	// Network and Security from global
	assert.Equal(t, globalCfg.Network.PortBind, resolved.Network.PortBind)
	assert.Equal(t, globalCfg.Security.MaxCages, resolved.Security.MaxCages)
}

func TestResolveProjectConfig_ProfileOverrides(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:    "heavy-project",
		Image:   "ubuntu-24.04",
		Profile: "heavy",
		VCPU:    16,    // override heavy profile VCPU
		DiskGB:  100,   // override heavy profile disk
		// Memory not set, should use heavy profile value
		Network: ProjectNetwork{
			SSH: "2222",
		},
	}

	resolved, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/project")
	require.NoError(t, err)

	assert.Equal(t, 16, resolved.VCPU)          // overridden
	assert.Equal(t, 8192, resolved.MemoryMB)    // from heavy profile
	assert.Equal(t, 100, resolved.DiskGB)       // overridden
	assert.Equal(t, "2222", resolved.SSHPort)
}

func TestResolveProjectConfig_UnknownProfile(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:    "my-project",
		Image:   "ubuntu-24.04",
		Profile: "nonexistent",
	}

	_, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestResolveProjectConfig_DefaultProfile(t *testing.T) {
	globalCfg := DefaultConfig()

	// No profile specified, should use "default"
	projectCfg := &ProjectConfig{
		Cage:  "my-project",
		Image: "ubuntu-24.04",
	}

	resolved, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/project")
	require.NoError(t, err)

	assert.Equal(t, 4, resolved.VCPU)        // default profile
	assert.Equal(t, 4096, resolved.MemoryMB) // default profile
	assert.Equal(t, 20, resolved.DiskGB)     // default profile
	assert.Equal(t, "auto", resolved.SSHPort) // default SSH port
}

func TestResolveProjectConfig_AbsoluteSharePath(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:  "my-project",
		Image: "ubuntu-24.04",
		Shares: []ShareConfig{
			{Host: "/absolute/path", Guest: "/guest", Mode: "ro"},
			{Host: "relative/path", Guest: "/guest2", Mode: "rw"},
		},
	}

	resolved, err := ResolveProjectConfig(globalCfg, projectCfg, "/project/dir")
	require.NoError(t, err)

	// Absolute path should stay absolute
	assert.Equal(t, "/absolute/path", resolved.Shares[0].Host)
	// Relative path should be resolved
	assert.Equal(t, "/project/dir/relative/path", resolved.Shares[1].Host)
}

func TestResolveProjectConfig_InvalidMemory(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:   "my-project",
		Image:  "ubuntu-24.04",
		Memory: "invalid",
	}

	_, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid memory")
}

func TestResolveProjectConfig_InvalidPortMapping(t *testing.T) {
	globalCfg := DefaultConfig()

	projectCfg := &ProjectConfig{
		Cage:  "my-project",
		Image: "ubuntu-24.04",
		Network: ProjectNetwork{
			Ports: []string{"invalid"},
		},
	}

	_, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port mapping")
}

