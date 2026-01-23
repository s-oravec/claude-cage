package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSHCmd_Exists(t *testing.T) {
	cmd := NewSSHCmd()

	assert.Equal(t, "ssh [name] [command]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestSSHCmd_AcceptsArgs(t *testing.T) {
	cmd := NewSSHCmd()

	// Should accept 1 arg (cage name)
	err := cmd.Args(cmd, []string{"mycage"})
	assert.NoError(t, err)

	// Should accept 2 args (cage name + command)
	err = cmd.Args(cmd, []string{"mycage", "whoami"})
	assert.NoError(t, err)
}
