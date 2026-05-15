package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagCmd_Args(t *testing.T) {
	c := NewTagCmd()
	c.SetArgs([]string{"only-one"})
	c.SilenceUsage = true
	c.SilenceErrors = true
	err := c.Execute()
	require.Error(t, err)
}
