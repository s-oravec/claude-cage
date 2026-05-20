package cmd

import (
	"testing"

	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePlatform_EmptyDefaultsToHost(t *testing.T) {
	arch, err := resolvePlatform("")
	require.NoError(t, err)
	assert.Equal(t, images.HostArchitecture(), arch)
}

func TestResolvePlatform_Arm64(t *testing.T) {
	arch, err := resolvePlatform("arm64")
	require.NoError(t, err)
	assert.Equal(t, "arm64", arch)
}

func TestResolvePlatform_Amd64(t *testing.T) {
	arch, err := resolvePlatform("amd64")
	require.NoError(t, err)
	assert.Equal(t, "amd64", arch)
}

func TestResolvePlatform_RejectsX86_64(t *testing.T) {
	_, err := resolvePlatform("x86_64")
	assert.Error(t, err)
}

func TestResolvePlatform_RejectsBogus(t *testing.T) {
	_, err := resolvePlatform("bogus")
	assert.Error(t, err)
}
