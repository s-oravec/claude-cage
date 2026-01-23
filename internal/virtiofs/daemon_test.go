package virtiofs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath_Tilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	result := ExpandPath("~/projects")
	assert.Equal(t, filepath.Join(home, "projects"), result)
}

func TestExpandPath_NoTilde(t *testing.T) {
	result := ExpandPath("/absolute/path")
	assert.Equal(t, "/absolute/path", result)
}

func TestSocketDir(t *testing.T) {
	dir := SocketDir("myproject")
	assert.Contains(t, dir, "myproject")
}

func TestSocketPath(t *testing.T) {
	path := SocketPath("myproject")
	assert.Contains(t, path, "myproject")
	assert.Contains(t, path, "virtiofs.sock")
}

func TestValidateShareDir_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	err := ValidateShareDir(tmpDir)
	assert.NoError(t, err)
}

func TestValidateShareDir_NotExists(t *testing.T) {
	err := ValidateShareDir("/nonexistent/path")
	assert.Error(t, err)
}

func TestValidateShareDir_NotDirectory(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(tmpFile, []byte("test"), 0644)

	err := ValidateShareDir(tmpFile)
	assert.Error(t, err)
}

func TestDaemonConfig(t *testing.T) {
	cfg := &DaemonConfig{
		CageName:  "test",
		SharedDir: "/home/user/projects",
		Sandbox:   true,
		Seccomp:   true,
	}

	assert.Equal(t, "test", cfg.CageName)
	assert.True(t, cfg.Sandbox)
	assert.True(t, cfg.Seccomp)
}

func TestBuildArgs(t *testing.T) {
	cfg := &DaemonConfig{
		CageName:  "test",
		SharedDir: "/home/user/projects",
		Sandbox:   true,
		Seccomp:   true,
	}

	args := BuildArgs(cfg)

	assert.Contains(t, args, "--shared-dir=/home/user/projects")
	assert.Contains(t, args, "--cache=auto")

	// When running as non-root (tests), always uses --sandbox=none
	// When running as root, would use --sandbox=chroot and --seccomp=kill
	if os.Getuid() == 0 {
		assert.Contains(t, args, "--sandbox=chroot")
		assert.Contains(t, args, "--seccomp=kill")
	} else {
		assert.Contains(t, args, "--sandbox=none")
	}
}

func TestBuildArgs_NoHardening(t *testing.T) {
	cfg := &DaemonConfig{
		CageName:  "test",
		SharedDir: "/home/user/projects",
		Sandbox:   false,
		Seccomp:   false,
	}

	args := BuildArgs(cfg)

	assert.Contains(t, args, "--shared-dir=/home/user/projects")
	// Non-root always gets --sandbox=none regardless of config
	if os.Getuid() != 0 {
		assert.Contains(t, args, "--sandbox=none")
	}
}

func TestEnsureSocketDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	err := EnsureSocketDir("testcage")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(tmpDir, "testcage"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCleanupSocket(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	// Create socket dir
	socketDir := filepath.Join(tmpDir, "testcage")
	os.MkdirAll(socketDir, 0755)
	os.WriteFile(filepath.Join(socketDir, "virtiofs.sock"), []byte{}, 0644)

	err := CleanupSocket("testcage")
	require.NoError(t, err)

	_, err = os.Stat(socketDir)
	assert.True(t, os.IsNotExist(err))
}
