package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/s-oravec/claude-cage/internal/config"
)

func TestStopCmd_Exists(t *testing.T) {
	cmd := NewStopCmd()

	assert.Equal(t, "stop [name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestStopCmd_HasForceFlag(t *testing.T) {
	cmd := NewStopCmd()

	flag := cmd.Flag("force")
	assert.NotNil(t, flag)
}

func TestStopCmd_HasAllFlag(t *testing.T) {
	cmd := NewStopCmd()

	flag := cmd.Flag("all")
	assert.NotNil(t, flag)
}

func TestStopCmd_AcceptsNoArgsWithConfig(t *testing.T) {
	cmd := NewStopCmd()

	// Should accept 0 args
	err := cmd.Args(cmd, []string{})
	assert.NoError(t, err)

	// Should accept 1 arg
	err = cmd.Args(cmd, []string{"mycage"})
	assert.NoError(t, err)
}

func TestStopCmd_RequiresNameWithoutConfig(t *testing.T) {
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
	cmd.SetArgs([]string{"stop"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), config.ProjectConfigFile)
}

func TestStopCmd_UsesProjectConfig(t *testing.T) {
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
	cmd.SetArgs([]string{"stop"})

	// Will fail because cage doesn't exist, but we verify it resolved the name
	err = cmd.Execute()
	require.Error(t, err, "Expected error when cage doesn't exist")
	assert.Contains(t, err.Error(), "test-cage")
}
