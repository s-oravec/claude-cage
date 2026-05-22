package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/s-oravec/cage/internal/config"
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
	// This is a unit test for project-config name resolution. It must NOT
	// reach libvirt. We point the project config at a nonexistent image so
	// that `cage start` exits at the IsDownloaded() check (start.go) before
	// any virsh/QEMU call. Previous version used `image: alpine-3.21`, which
	// in environments where alpine-3.21 happened to be installed caused the
	// test to define real libvirt domains, race with parallel packages, and
	// leak orphan `cage-test-start-XXXX` domains.
	var rb [6]byte
	_, _ = rand.Read(rb[:])
	cageName := "test-start-" + hex.EncodeToString(rb[:])
	imageName := "cage-test-nonexistent-image"

	tmpDir := t.TempDir()
	configContent := fmt.Sprintf("image: %s\ncage: %s\n", imageName, cageName)
	err := os.WriteFile(filepath.Join(tmpDir, config.ProjectConfigFile), []byte(configContent), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Safety net: if start.go ever changes so this test does reach libvirt,
	// clean up cage state and any orphan domain instead of leaving artifacts
	// that block subsequent runs.
	t.Cleanup(func() {
		cleanupCmd := NewRootCmd()
		cleanupCmd.SetOut(&bytes.Buffer{})
		cleanupCmd.SetErr(&bytes.Buffer{})
		cleanupCmd.SetArgs([]string{"remove", cageName, "--force"})
		_ = cleanupCmd.Execute()
		_ = exec.Command("virsh", "-c", "qemu:///session", "undefine",
			"--nvram", "--remove-all-storage",
			"--snapshots-metadata", "--checkpoints-metadata",
			"cage-"+cageName).Run()
	})

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err = cmd.Execute()
	// We expect an error (image is bogus) but the error must come from
	// image resolution, not from the CLI failing to find the cage name -
	// that would mean project-config resolution is broken.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cage name required")
	assert.Contains(t, err.Error(), imageName,
		"start should fail at image-not-found check, after resolving name from project config")
}
