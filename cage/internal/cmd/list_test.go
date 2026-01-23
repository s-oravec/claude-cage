package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCmd_Exists(t *testing.T) {
	cmd := NewListCmd()

	assert.Equal(t, "list", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestListCmd_HasJsonFlag(t *testing.T) {
	cmd := NewListCmd()

	flag := cmd.Flag("json")
	assert.NotNil(t, flag)
}
