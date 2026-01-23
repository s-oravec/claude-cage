package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStopCmd_Exists(t *testing.T) {
	cmd := NewStopCmd()

	assert.Equal(t, "stop [name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestStopCmd_HasForceFlag(t *testing.T) {
	cmd := NewStopCmd()

	flag := cmd.Flag("force")
	assert.NotNil(t, flag)
}

func TestStopCmd_HasAllFlag(t *testing.T) {
	cmd := NewStopCmd()

	flag := cmd.Flag("all")
	assert.NotNil(t, flag)
}
