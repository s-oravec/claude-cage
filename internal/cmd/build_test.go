package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCmd_HasPlatformFlag(t *testing.T) {
	cmd := NewBuildCmd()

	flag := cmd.Flag("platform")
	assert.NotNil(t, flag)
}

func TestPushCmd_HasNoPlatformFlag(t *testing.T) {
	cmd := NewPushCmd()

	// push has no arch choice; its arch is intrinsic to the local manifest.
	flag := cmd.Flag("platform")
	assert.Nil(t, flag)
}
