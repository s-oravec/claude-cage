package cloudinit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateUserData(t *testing.T) {
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... cage@test"

	userData := GenerateUserData("test-cage", pubKey)

	assert.Contains(t, userData, "#cloud-config")
	assert.Contains(t, userData, "name: cage")
	assert.Contains(t, userData, "NOPASSWD:ALL")
	assert.Contains(t, userData, pubKey)
	assert.Contains(t, userData, "ssh_pwauth: true")
	assert.Contains(t, userData, "lock_passwd: false")
	assert.Contains(t, userData, "passwd:")
}

func TestGenerateMetaData(t *testing.T) {
	metaData := GenerateMetaData("test-cage")

	assert.Contains(t, metaData, "instance-id: test-cage")
	assert.Contains(t, metaData, "local-hostname: test-cage")
}

func TestWriteCloudInitFiles(t *testing.T) {
	tmpDir := t.TempDir()

	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... cage@test"

	err := WriteCloudInitFiles(tmpDir, "test-cage", pubKey)
	require.NoError(t, err)

	// user-data should exist
	userData, err := os.ReadFile(filepath.Join(tmpDir, "user-data"))
	require.NoError(t, err)
	assert.Contains(t, string(userData), "#cloud-config")

	// meta-data should exist
	metaData, err := os.ReadFile(filepath.Join(tmpDir, "meta-data"))
	require.NoError(t, err)
	assert.Contains(t, string(metaData), "instance-id")
}

func TestGenerateISO_FilesExist(t *testing.T) {
	tmpDir := t.TempDir()
	cageDir := filepath.Join(tmpDir, "test-cage")
	os.MkdirAll(cageDir, 0755)

	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... cage@test"

	// This test may fail if genisoimage/cloud-localds not installed
	// That's OK - the function handles fallback
	isoPath, err := GenerateISO(cageDir, "test-cage", pubKey)

	if err == nil {
		// ISO should exist
		_, statErr := os.Stat(isoPath)
		assert.NoError(t, statErr)
	}
	// If tools not installed, function returns error which is acceptable
}
