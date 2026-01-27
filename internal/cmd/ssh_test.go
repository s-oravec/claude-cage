package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHCmd_Exists(t *testing.T) {
	cmd := NewSSHCmd()

	assert.Equal(t, "ssh [name] [command...]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestSSHCmd_AcceptsArgs(t *testing.T) {
	cmd := NewSSHCmd()

	// Should accept 0 args
	err := cmd.Args(cmd, []string{})
	assert.NoError(t, err)

	// Should accept 1 arg (cage name or command)
	err = cmd.Args(cmd, []string{"mycage"})
	assert.NoError(t, err)

	// Should accept 2 args (cage name + command)
	err = cmd.Args(cmd, []string{"mycage", "whoami"})
	assert.NoError(t, err)

	// Should accept multiple args
	err = cmd.Args(cmd, []string{"mycage", "ls", "-la", "/home"})
	assert.NoError(t, err)
}

func TestResolveSSHArgs_WithExistingCage(t *testing.T) {
	// This test requires an existing cage, so we test the non-existing case
	name, command, err := resolveSSHArgs([]string{"nonexistent-cage"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-cage")
	assert.Empty(t, name)
	assert.Empty(t, command)
}

func TestResolveSSHArgs_NoArgsWithConfig(t *testing.T) {
	// Create temp directory with config
	tmpDir := t.TempDir()
	configContent := `image: ubuntu-24.04
cage: project-cage
`
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Chdir(oldWd)
	})

	name, command, err := resolveSSHArgs([]string{})

	assert.NoError(t, err)
	assert.Equal(t, "project-cage", name)
	assert.Empty(t, command)
}

func TestResolveSSHArgs_CommandOnlyWithConfig(t *testing.T) {
	// Create temp directory with config
	tmpDir := t.TempDir()
	configContent := `image: ubuntu-24.04
cage: project-cage
`
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	// Save current directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Chdir(oldWd)
	})

	// When first arg is not an existing cage, treat all args as command
	name, command, err := resolveSSHArgs([]string{"ls", "-la"})

	assert.NoError(t, err)
	assert.Equal(t, "project-cage", name)
	assert.Equal(t, "ls -la", command)
}

func TestSSHCmd_UsesProjectConfig(t *testing.T) {
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

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ssh"})

	// Will fail because cage doesn't exist, but we verify it resolved the name
	err = cmd.Execute()
	require.Error(t, err, "Expected error when cage doesn't exist")
	assert.Contains(t, err.Error(), "test-cage")
}
