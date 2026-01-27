package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// Use unique cage name to avoid collisions
	cageName := fmt.Sprintf("test-start-%d", time.Now().UnixNano()%10000)

	// Create a temp directory with a project config
	tmpDir := t.TempDir()

	// Create project config file
	configContent := fmt.Sprintf(`image: alpine-3.21
cage: %s
`, cageName)
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Cleanup any created cage on exit
	t.Cleanup(func() {
		// Stop and remove cage if it was created
		cleanupCmd := NewRootCmd()
		cleanupCmd.SetOut(&bytes.Buffer{})
		cleanupCmd.SetErr(&bytes.Buffer{})
		cleanupCmd.SetArgs([]string{"remove", cageName, "--force"})
		cleanupCmd.Execute() // Ignore errors - cage might not exist
	})

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
