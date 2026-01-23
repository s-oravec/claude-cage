package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeysDir(t *testing.T) {
	dir := KeysDir()
	assert.Contains(t, dir, ".claude-cage")
	assert.Contains(t, dir, "keys")
}

func TestKeyPath(t *testing.T) {
	path := KeyPath("myproject")
	assert.Contains(t, path, "myproject")
	assert.Contains(t, path, "id_ed25519")
}

func TestGenerateKeyPair(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := keysDir
	keysDir = tmpDir
	defer func() { keysDir = oldDir }()

	err := GenerateKeyPair("test-cage")
	require.NoError(t, err)

	// Private key should exist
	privKey := filepath.Join(tmpDir, "test-cage", "id_ed25519")
	info, err := os.Stat(privKey)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Public key should exist
	pubKey := filepath.Join(tmpDir, "test-cage", "id_ed25519.pub")
	_, err = os.Stat(pubKey)
	require.NoError(t, err)
}

func TestGetPublicKey(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := keysDir
	keysDir = tmpDir
	defer func() { keysDir = oldDir }()

	// Generate first
	err := GenerateKeyPair("test-cage")
	require.NoError(t, err)

	// Get public key
	pubKey, err := GetPublicKey("test-cage")
	require.NoError(t, err)

	assert.Contains(t, pubKey, "ssh-ed25519")
	assert.Contains(t, pubKey, "cage@test-cage")
}

func TestGetPublicKey_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := keysDir
	keysDir = tmpDir
	defer func() { keysDir = oldDir }()

	_, err := GetPublicKey("nonexistent")
	assert.Error(t, err)
}

func TestKeyExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := keysDir
	keysDir = tmpDir
	defer func() { keysDir = oldDir }()

	assert.False(t, KeyExists("test-cage"))

	GenerateKeyPair("test-cage")

	assert.True(t, KeyExists("test-cage"))
}

func TestDeleteKeys(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := keysDir
	keysDir = tmpDir
	defer func() { keysDir = oldDir }()

	GenerateKeyPair("test-cage")
	assert.True(t, KeyExists("test-cage"))

	err := DeleteKeys("test-cage")
	require.NoError(t, err)

	assert.False(t, KeyExists("test-cage"))
}
