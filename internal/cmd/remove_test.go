package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveCmd_Exists(t *testing.T) {
	cmd := NewRemoveCmd()

	assert.Equal(t, "remove [name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestRemoveCmd_HasForceFlag(t *testing.T) {
	cmd := NewRemoveCmd()

	flag := cmd.Flag("force")
	assert.NotNil(t, flag)
}

func TestRemoveCmd_HasAllFlag(t *testing.T) {
	cmd := NewRemoveCmd()

	flag := cmd.Flag("all")
	assert.NotNil(t, flag)
}

func TestRemoveCmd_RequiresName(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestRemoveCmd_NonExistentCage(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"remove", "nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
