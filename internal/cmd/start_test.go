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

func TestStartCmd_Exists(t *testing.T) {
	cmd := NewStartCmd()

	assert.Equal(t, "start [name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestStartCmd_HasPortFlag(t *testing.T) {
	cmd := NewStartCmd()

	flag := cmd.Flag("port")
	assert.NotNil(t, flag)
}

func TestStartCmd_RequiresNameOrProjectConfig(t *testing.T) {
	// When run without name and without project config, should error
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), config.ProjectConfigFile)
}

func TestStartCmd_NonExistentCage(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start", "nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStartCmd_UsesProjectConfig(t *testing.T) {
	// Create a temp directory with a project config
	tmpDir := t.TempDir()

	// Create project config file
	configContent := `image: alpine-3.21
cage: test-cage
`
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Run start without name - should pick up cage name from config
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err = cmd.Execute()
	// Will fail because global config doesn't exist, but it should not be
	// a "name required" error - it should be a config loading error or similar
	if err != nil {
		// Should not complain about missing name
		assert.NotContains(t, err.Error(), "cage name required")
	}
	// If no error, the command started successfully which means config was read
}
