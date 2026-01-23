package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestStartCmd_RequiresName(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err := cmd.Execute()
	assert.Error(t, err)
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
