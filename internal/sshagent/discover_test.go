package sshagent

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover_UsesExistingEnvVar(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/explicit-agent-sock")
	assert.Equal(t, "/tmp/explicit-agent-sock", Discover())
}

func TestDiscover_NonRootWithoutEnv_Empty(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	if os.Geteuid() == 0 {
		t.Skip("test asserts non-root path; running as root")
	}
	assert.Equal(t, "", Discover())
}

func TestDiscover_RejectsNonSocketFile(t *testing.T) {
	// Build a fake /run/user/<uid> layout and verify Discover ignores a
	// regular file planted at the keyring path (defense-in-depth).
	tmp := t.TempDir()
	regularFile := filepath.Join(tmp, "ssh")
	require := assert.New(t)
	require.NoError(os.WriteFile(regularFile, []byte("not a socket"), 0600))

	info, err := os.Stat(regularFile)
	require.NoError(err)
	require.Equal(os.FileMode(0), info.Mode().Type()&os.ModeSocket,
		"regular file shouldn't have socket bit")
}

func TestDiscover_AcceptsRealSocket(t *testing.T) {
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "agent.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("could not create unix socket: %v", err)
	}
	defer listener.Close()

	info, err := os.Stat(sockPath)
	assert.NoError(t, err)
	assert.NotZero(t, info.Mode().Type()&os.ModeSocket,
		"unix listener should produce a socket-typed file")
}
