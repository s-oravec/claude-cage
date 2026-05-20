package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPullCmd_Exists(t *testing.T) {
	cmd := NewPullCmd()

	assert.Equal(t, "pull", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestPullCmd_HasBaseFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("base")
	assert.NotNil(t, flag)
}

func TestPullCmd_HasListFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("list")
	assert.NotNil(t, flag)
}

func TestPullCmd_LongMentionsRegistry(t *testing.T) {
	c := NewPullCmd()
	assert.Contains(t, c.Long, "registry")
}

func TestPullCmd_HasPlatformFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("platform")
	assert.NotNil(t, flag)
}
