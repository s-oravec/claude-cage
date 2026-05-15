package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogoutCmd_Flags(t *testing.T) {
	c := NewLogoutCmd()
	assert.NotNil(t, c.Flag("all"))
}

func TestLogoutCmd_RequiresArgOrAll(t *testing.T) {
	c := NewLogoutCmd()
	c.SetArgs([]string{})
	c.SilenceUsage = true
	c.SilenceErrors = true
	err := c.Execute()
	require.Error(t, err)
}
