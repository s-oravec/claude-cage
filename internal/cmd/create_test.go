package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCmd_Exists(t *testing.T) {
	cmd := NewCreateCmd()

	assert.Equal(t, "create", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestCreateCmd_HasNameFlag(t *testing.T) {
	cmd := NewCreateCmd()

	flag := cmd.Flag("name")
	assert.NotNil(t, flag)
}

func TestCreateCmd_HasProfileFlag(t *testing.T) {
	cmd := NewCreateCmd()

	flag := cmd.Flag("profile")
	assert.NotNil(t, flag)
}

func TestCreateCmd_HasImageFlag(t *testing.T) {
	cmd := NewCreateCmd()

	flag := cmd.Flag("image")
	assert.NotNil(t, flag)
}

func TestCreateCmd_HasNetworkFlag(t *testing.T) {
	cmd := NewCreateCmd()

	flag := cmd.Flag("network")
	assert.NotNil(t, flag)
	assert.Equal(t, "auto", flag.DefValue)
}

func TestCreateCmd_RequiresName(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"create"})

	err := cmd.Execute()
	assert.Error(t, err)
}
