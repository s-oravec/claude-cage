package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-oravec/cage/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveCmd_Exists(t *testing.T) {
	cmd := NewRemoveCmd()

	assert.Equal(t, "remove [name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestRemoveCmd_HasForceFlag(t *testing.T) {
	cmd := NewRemoveCmd()

	flag := cmd.Flag("force")
	assert.NotNil(t, flag)
}

func TestRemoveCmd_HasAllFlag(t *testing.T) {
	cmd := NewRemoveCmd()

	flag := cmd.Flag("all")
	assert.NotNil(t, flag)
}

func TestRemoveCmd_HasRmAlias(t *testing.T) {
	cmd := NewRemoveCmd()

	assert.Contains(t, cmd.Aliases, "rm")
}

func TestRemoveCmd_AcceptsNoArgsWithConfig(t *testing.T) {
	cmd := NewRemoveCmd()

	// Should accept 0 args
	err := cmd.Args(cmd, []string{})
	assert.NoError(t, err)

	// Should accept 1 arg
	err = cmd.Args(cmd, []string{"mycage"})
	assert.NoError(t, err)
}

func TestRemoveCmd_RequiresNameWithoutConfig(t *testing.T) {
	// Create temp directory without config
	tmpDir := t.TempDir()

	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	// Change to temp directory
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Chdir(oldWd)
	})

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), config.ProjectConfigFile)
}

func TestRemoveCmd_NonExistentCage(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove", "nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemoveCmd_UsesProjectConfig(t *testing.T) {
	// Create temp directory with config FIRST before changing dirs
	tmpDir := t.TempDir()
	configContent := `image: ubuntu-24.04
cage: test-cage
`
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	// Change to temp directory
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "Failed to change to temp directory")

	// Use t.Cleanup for more reliable cleanup
	t.Cleanup(func() {
		os.Chdir(oldWd)
	})

	// Verify we're in the right place. On macOS /var is a symlink to
	// /private/var, so the path returned by Getwd() may not be byte-equal
	// to t.TempDir(); resolve both sides before comparing.
	cwd, _ := os.Getwd()
	wantDir, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	gotDir, err := filepath.EvalSymlinks(cwd)
	require.NoError(t, err)
	require.Equal(t, wantDir, gotDir, "Should be in tmpDir")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove"})

	// Will fail because cage doesn't exist, but we verify it resolved the name
	err = cmd.Execute()
	require.Error(t, err, "Expected error when cage doesn't exist")
	assert.Contains(t, err.Error(), "test-cage")
}

func TestRemoveCmd_RmAliasWorks(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"rm", "nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
