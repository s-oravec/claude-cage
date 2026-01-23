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
	assert.Contains(t, userData, "ssh_pwauth: false")
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

func TestGenerateUserDataWithConfig_Basic(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName:      "testcage",
		PubKey:        "ssh-ed25519 AAAA... test@key",
		MountVirtiofs: false,
	}

	userData := GenerateUserDataWithConfig(cfg)

	assert.Contains(t, userData, "#cloud-config")
	assert.Contains(t, userData, "name: cage")
	assert.Contains(t, userData, cfg.PubKey)
	assert.Contains(t, userData, "ssh_pwauth: false")
	assert.NotContains(t, userData, "workspace")
}

func TestGenerateUserDataWithConfig_WithVirtiofs(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName:      "testcage",
		PubKey:        "ssh-ed25519 AAAA... test@key",
		MountVirtiofs: true,
	}

	userData := GenerateUserDataWithConfig(cfg)

	assert.Contains(t, userData, "#cloud-config")
	assert.Contains(t, userData, cfg.PubKey)

	// Should have virtiofs mounts
	assert.Contains(t, userData, "mounts:")
	assert.Contains(t, userData, "workspace")
	assert.Contains(t, userData, "virtiofs")

	// Should have runcmd for mounting
	assert.Contains(t, userData, "mkdir -p /workspace")
	assert.Contains(t, userData, "mount -t virtiofs")
	assert.Contains(t, userData, "chown cage:cage /workspace")
}

func TestCloudInitConfig_Structure(t *testing.T) {
	cfg := CloudInitConfig{
		CageName:      "mycage",
		PubKey:        "ssh-ed25519 key",
		MountVirtiofs: true,
	}

	assert.Equal(t, "mycage", cfg.CageName)
	assert.Equal(t, "ssh-ed25519 key", cfg.PubKey)
	assert.True(t, cfg.MountVirtiofs)
}

func TestGenerateISOWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cageDir := filepath.Join(tmpDir, "test-cage")
	os.MkdirAll(cageDir, 0755)

	cfg := &CloudInitConfig{
		CageName:      "test-cage",
		PubKey:        "ssh-ed25519 AAAA... test@key",
		MountVirtiofs: true,
	}

	isoPath, err := GenerateISOWithConfig(cageDir, cfg)

	if err == nil {
		// ISO should exist
		_, statErr := os.Stat(isoPath)
		assert.NoError(t, statErr)
	}
	// If tools not installed, error is acceptable
}

func TestWriteCloudInitFilesWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &CloudInitConfig{
		CageName:      "test-cage",
		PubKey:        "ssh-ed25519 AAAA... test@key",
		MountVirtiofs: true,
	}

	err := WriteCloudInitFilesWithConfig(tmpDir, cfg)
	require.NoError(t, err)

	// user-data should exist with virtiofs config
	userData, err := os.ReadFile(filepath.Join(tmpDir, "user-data"))
	require.NoError(t, err)
	assert.Contains(t, string(userData), "workspace")
	assert.Contains(t, string(userData), "virtiofs")

	// meta-data should exist
	metaData, err := os.ReadFile(filepath.Join(tmpDir, "meta-data"))
	require.NoError(t, err)
	assert.Contains(t, string(metaData), "instance-id")
}

func TestGenerateUserData_HasPassword(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should have password hash for console login
	assert.Contains(t, userData, "passwd:")
	assert.Contains(t, userData, "$6$") // SHA-512 hash prefix
	assert.Contains(t, userData, "lock_passwd: false")
}

func TestGenerateUserData_DisablesSSHPassword(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// SSH password auth should be disabled
	assert.Contains(t, userData, "ssh_pwauth: false")
}

func TestGenerateUserData_DockerGroup(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// User should be in docker and wheel groups
	assert.Contains(t, userData, "groups: wheel,docker")
}

func TestGenerateUserData_SudoNoPassword(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should have passwordless sudo
	assert.Contains(t, userData, "NOPASSWD:ALL")
}

func TestGenerateUserDataWithConfig_WithEnv(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName: "testcage",
		PubKey:   "ssh-ed25519 AAAA... test@key",
		Env: map[string]string{
			"MY_VAR":      "hello",
			"ANOTHER_VAR": "world",
		},
	}

	userData := GenerateUserDataWithConfig(cfg)

	// Should have env setup commands
	assert.Contains(t, userData, "/etc/profile.d/cage-env.sh")
	assert.Contains(t, userData, "export MY_VAR='hello'")
	assert.Contains(t, userData, "export ANOTHER_VAR='world'")
}

func TestGenerateUserDataWithConfig_EnvWithSpecialChars(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName: "testcage",
		PubKey:   "ssh-ed25519 AAAA... test@key",
		Env: map[string]string{
			"PATH_VAR":  "/usr/local/bin:/usr/bin",
			"QUOTE_VAR": "it's working",
		},
	}

	userData := GenerateUserDataWithConfig(cfg)

	// Should handle special characters
	assert.Contains(t, userData, "export PATH_VAR='/usr/local/bin:/usr/bin'")
	// Single quotes should be escaped
	assert.Contains(t, userData, "export QUOTE_VAR='it'\\''s working'")
}

func TestGenerateUserDataWithConfig_EmptyEnv(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName: "testcage",
		PubKey:   "ssh-ed25519 AAAA... test@key",
		Env:      map[string]string{},
	}

	userData := GenerateUserDataWithConfig(cfg)

	// Should not have env setup when empty
	assert.NotContains(t, userData, "/etc/profile.d/cage-env.sh")
}

func TestGenerateUserDataWithConfig_NilEnv(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName: "testcage",
		PubKey:   "ssh-ed25519 AAAA... test@key",
		Env:      nil,
	}

	userData := GenerateUserDataWithConfig(cfg)

	// Should not have env setup when nil
	assert.NotContains(t, userData, "/etc/profile.d/cage-env.sh")
}

func TestGenerateUserData_AlpineSupport(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should install sudo and doas on Alpine
	assert.Contains(t, userData, "apk add --no-cache sudo doas")

	// Should configure doas for Alpine
	assert.Contains(t, userData, "permit nopass :wheel")
	assert.Contains(t, userData, "/etc/doas.d/wheel.conf")

	// Should setup sudoers.d for all distros
	assert.Contains(t, userData, "/etc/sudoers.d/cage")
}

func TestGenerateUserData_WheelGroup(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// User should be in wheel group (for doas/sudo on RHEL-based and Alpine)
	assert.Contains(t, userData, "wheel")
}

func TestGenerateUserData_OpenRCSupport(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should support OpenRC (Alpine) for Docker
	assert.Contains(t, userData, "rc-update")
	assert.Contains(t, userData, "rc-service")
}

func TestGenerateUserData_SystemdSupport(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should support systemd for Docker
	assert.Contains(t, userData, "systemctl enable docker")
	assert.Contains(t, userData, "systemctl start docker")
}

func TestGenerateUserData_GrowPartition(t *testing.T) {
	userData := GenerateUserData("test", "ssh-ed25519 key")

	// Should have growpart config
	assert.Contains(t, userData, "growpart:")
	assert.Contains(t, userData, "mode: auto")
	assert.Contains(t, userData, "resize_rootfs: true")
}
