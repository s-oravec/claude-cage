package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupCmd_Exists(t *testing.T) {
	cmd := NewSetupCmd()

	assert.Equal(t, "setup", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestSetupCmd_HasBaseFlag(t *testing.T) {
	cmd := NewSetupCmd()

	flag := cmd.Flag("base")
	assert.NotNil(t, flag)
}

func TestSetupCmd_HasListFlag(t *testing.T) {
	cmd := NewSetupCmd()

	flag := cmd.Flag("list")
	assert.NotNil(t, flag)
}
