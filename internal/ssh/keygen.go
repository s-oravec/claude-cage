package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/s-oravec/cage/internal/config"
)

// keysDir can be overridden in tests
var keysDir string

// KeysDir returns the SSH keys directory
func KeysDir() string {
	if keysDir != "" {
		return keysDir
	}
	return filepath.Join(config.Dir(), "keys")
}

// KeyPath returns the private key path for a cage
func KeyPath(cageName string) string {
	return filepath.Join(KeysDir(), cageName, "id_ed25519")
}

// PubKeyPath returns the public key path for a cage
func PubKeyPath(cageName string) string {
	return filepath.Join(KeysDir(), cageName, "id_ed25519.pub")
}

// KeyExists checks if keys exist for a cage
func KeyExists(cageName string) bool {
	_, err := os.Stat(KeyPath(cageName))
	return err == nil
}

// GenerateKeyPair generates an Ed25519 SSH key pair for a cage
func GenerateKeyPair(cageName string) error {
	keyDir := filepath.Join(KeysDir(), cageName)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	keyPath := KeyPath(cageName)

	// Remove existing keys if any
	os.Remove(keyPath)
	os.Remove(keyPath + ".pub")

	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "", // no passphrase
		"-C", fmt.Sprintf("cage@%s", cageName),
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-keygen failed: %s", string(out))
	}

	// Set correct permissions on private key
	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("failed to set key permissions: %w", err)
	}

	return nil
}

// GetPublicKey reads and returns the public key for a cage
func GetPublicKey(cageName string) (string, error) {
	data, err := os.ReadFile(PubKeyPath(cageName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// DeleteKeys removes SSH keys for a cage
func DeleteKeys(cageName string) error {
	return os.RemoveAll(filepath.Join(KeysDir(), cageName))
}

// KnownHostsPath returns the path to the known_hosts file
func KnownHostsPath() string {
	return filepath.Join(config.Dir(), "known_hosts")
}

// RemoveKnownHost removes a host entry from the known_hosts file.
// For user-mode networking with port forwarding, host should be "[127.0.0.1]:PORT".
// For bridge networking, host should be the IP address.
func RemoveKnownHost(host string) error {
	knownHostsPath := KnownHostsPath()

	// Check if file exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return nil // Nothing to remove
	}

	cmd := exec.Command("ssh-keygen", "-f", knownHostsPath, "-R", host)
	// Ignore errors - entry might not exist
	cmd.Run()
	return nil
}
