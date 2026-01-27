package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCageName_WithArgs(t *testing.T) {
	name, projectCfg, err := resolveCageName([]string{"my-cage"})

	assert.NoError(t, err)
	assert.Equal(t, "my-cage", name)
	assert.Nil(t, projectCfg)
}

func TestResolveCageName_WithArgsMultiple(t *testing.T) {
	// Only first arg is used
	name, projectCfg, err := resolveCageName([]string{"cage1", "cage2"})

	assert.NoError(t, err)
	assert.Equal(t, "cage1", name)
	assert.Nil(t, projectCfg)
}

func TestResolveCageName_NoArgsNoConfig(t *testing.T) {
	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd)

	// Change to temp directory without config
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	_, _, err = resolveCageName([]string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), config.ProjectConfigFile)
}

func TestResolveCageName_NoArgsWithConfig(t *testing.T) {
	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd)

	// Create temp directory with config
	tmpDir := t.TempDir()
	configContent := `image: ubuntu-24.04
cage: project-cage
`
	err = os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	os.Chdir(tmpDir)

	name, projectCfg, err := resolveCageName([]string{})

	assert.NoError(t, err)
	assert.Equal(t, "project-cage", name)
	assert.NotNil(t, projectCfg)
	assert.Equal(t, "ubuntu-24.04", projectCfg.Image)
}

func TestResolveCageName_NoArgsWithConfigDefaultName(t *testing.T) {
	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd)

	// Create temp directory with config (no cage name, should default to dir name)
	tmpDir := t.TempDir()
	configContent := `image: alpine-3.21
`
	err = os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	os.Chdir(tmpDir)

	name, projectCfg, err := resolveCageName([]string{})

	assert.NoError(t, err)
	// Name should be the directory basename
	assert.Equal(t, filepath.Base(tmpDir), name)
	assert.NotNil(t, projectCfg)
}
