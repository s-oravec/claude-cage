package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartCmd_Exists(t *testing.T) {
	cmd := NewStartCmd()

	assert.Equal(t, "start", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestStartCmd_HasNameFlag(t *testing.T) {
	cmd := NewStartCmd()

	flag := cmd.Flag("name")
	assert.NotNil(t, flag)
}

func TestStartCmd_HasProfileFlag(t *testing.T) {
	cmd := NewStartCmd()

	flag := cmd.Flag("profile")
	assert.NotNil(t, flag)
}

func TestStartCmd_HasImageFlag(t *testing.T) {
	cmd := NewStartCmd()

	flag := cmd.Flag("image")
	assert.NotNil(t, flag)
}

func TestStartCmd_HasPortFlag(t *testing.T) {
	cmd := NewStartCmd()

	flag := cmd.Flag("port")
	assert.NotNil(t, flag)
}
