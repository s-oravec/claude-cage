package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s-oravec/cage/internal/config"
)

func TestInitCommand_CreatesConfigFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--image", "ubuntu-24.04", "--dir", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err)

	// Check file was created
	configPath := filepath.Join(tmpDir, config.ProjectConfigFile)
	assert.FileExists(t, configPath)

	// Read and verify content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Should have header comment
	assert.Contains(t, string(data), "# Cage configuration")

	// Parse and verify YAML
	var cfg config.ProjectConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "ubuntu-24.04", cfg.Image)
	assert.Equal(t, "auto", cfg.Network.SSH)
	// Lite cagefile by default: no shares
	assert.Empty(t, cfg.Shares)

	// Check success message
	assert.Contains(t, buf.String(), "Created")
}

func TestInitCommand_RootAddsShares(t *testing.T) {
	tmpDir := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--image", "ubuntu-24.04", "--root", "--dir", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, config.ProjectConfigFile))
	require.NoError(t, err)

	var cfg config.ProjectConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	require.Len(t, cfg.Shares, 1)
	assert.Equal(t, ".", cfg.Shares[0].Host)
	assert.Equal(t, "/workspace", cfg.Shares[0].Guest)
}

func TestInitCommand_FailsIfConfigExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing config file
	configPath := filepath.Join(tmpDir, config.ProjectConfigFile)
	err := os.WriteFile(configPath, []byte("existing: config"), 0644)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--image", "ubuntu-24.04", "--dir", tmpDir})

	err = cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInitCommand_ForceOverwrites(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing config file
	configPath := filepath.Join(tmpDir, config.ProjectConfigFile)
	err := os.WriteFile(configPath, []byte("existing: config"), 0644)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--image", "ubuntu-24.04", "--force", "--dir", tmpDir})

	err = cmd.Execute()

	require.NoError(t, err)

	// Verify new content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.ProjectConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "ubuntu-24.04", cfg.Image)
}

func TestInitCommand_RequiresImageWhenNoDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// Override config dir and create config with empty default image
	configDir := filepath.Join(tmpDir, "empty-config")
	os.MkdirAll(configDir, 0755)
	oldConfigDir := config.Dir()
	config.SetDir(configDir)
	defer config.SetDir(oldConfigDir)

	// Create config file with no default image
	cfg := config.DefaultConfig()
	cfg.Images.Default = "" // No default image
	config.Save(cfg)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--dir", tmpDir})

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image")
}

func TestInitCommand_SetsOptionalFields(t *testing.T) {
	tmpDir := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--image", "debian-12",
		"--cage", "myproject",
		"--memory", "8G",
		"--vcpu", "4",
		"--disk", "50",
		"--ssh", "2222",
		"--dir", tmpDir,
	})

	err := cmd.Execute()

	require.NoError(t, err)

	// Read and verify content
	configPath := filepath.Join(tmpDir, config.ProjectConfigFile)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.ProjectConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "debian-12", cfg.Image)
	assert.Equal(t, "myproject", cfg.Cage)
	assert.Equal(t, "8G", cfg.Memory)
	assert.Equal(t, 4, cfg.VCPU)
	assert.Equal(t, 50, cfg.DiskGB)
	assert.Equal(t, "2222", cfg.Network.SSH)
}

func TestInitCommand_DefaultCageNameFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := NewInitCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--image", "ubuntu-24.04", "--dir", tmpDir})

	err := cmd.Execute()

	require.NoError(t, err)

	// Read and verify - cage should be empty (will default to dir name when loaded)
	configPath := filepath.Join(tmpDir, config.ProjectConfigFile)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.ProjectConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Cage name should be omitted (empty) - LoadProjectConfig will default it
	assert.Empty(t, cfg.Cage)
}
