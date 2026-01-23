package virtiofs

import (
	"os"
	"path/filepath"
	"strings"
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

func TestStopByPID_InvalidPID(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	// Create socket dir to cleanup
	socketDir := filepath.Join(tmpDir, "testcage")
	os.MkdirAll(socketDir, 0755)

	// Using PID 0 or -1 will fail to send signal, but function should handle it
	err := StopByPID("testcage", 0)
	assert.NoError(t, err) // Should not error

	// Socket dir should be cleaned up
	_, err = os.Stat(socketDir)
	assert.True(t, os.IsNotExist(err))
}

func TestStopByPID_NonExistentPID(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	// Using a very high PID that likely doesn't exist
	err := StopByPID("testcage", 999999999)
	assert.NoError(t, err) // Should not error, just ignores non-existent process
}

func TestExpandPath_RelativePath(t *testing.T) {
	result := ExpandPath("relative/path")
	assert.Equal(t, "relative/path", result)
}

func TestExpandPath_TildeOnly(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/")
	// ExpandPath joins home with empty string after ~/, resulting in just home
	assert.Equal(t, home, result)
}

func TestSocketPath_ContainsSocketFilename(t *testing.T) {
	path := SocketPath("test")
	assert.True(t, filepath.Base(path) == "virtiofs.sock")
}

func TestDaemonConfig_DefaultValues(t *testing.T) {
	cfg := &DaemonConfig{}
	assert.Empty(t, cfg.CageName)
	assert.Empty(t, cfg.SharedDir)
	assert.False(t, cfg.Sandbox)
	assert.False(t, cfg.Seccomp)
}

func TestBuildArgs_ContainsSocketPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	cfg := &DaemonConfig{
		CageName:  "mytest",
		SharedDir: "/tmp/share",
	}

	args := BuildArgs(cfg)

	// Find the socket path argument
	var socketArg string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--socket-path=") {
			socketArg = arg
			break
		}
	}

	assert.NotEmpty(t, socketArg)
	assert.Contains(t, socketArg, "mytest")
	assert.Contains(t, socketArg, "virtiofs.sock")
}

func TestCleanupSocket_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	oldBase := socketBaseDir
	socketBaseDir = tmpDir
	defer func() { socketBaseDir = oldBase }()

	// Should not error when cleaning up non-existent socket
	err := CleanupSocket("nonexistent")
	assert.NoError(t, err)
}

func TestFindVirtiofsd_ReturnsPath(t *testing.T) {
	// This test verifies the function runs without error
	// Actual result depends on system configuration
	path := FindVirtiofsd()
	_ = path // Result depends on whether virtiofsd is installed
}
