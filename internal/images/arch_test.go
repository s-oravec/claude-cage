package images

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostArchitecture_ReturnsGOARCH(t *testing.T) {
	assert.Equal(t, runtime.GOARCH, HostArchitecture())
}

func TestSupportedArchitectures_HasAmd64AndArm64(t *testing.T) {
	assert.Contains(t, SupportedArchitectures, "amd64")
	assert.Contains(t, SupportedArchitectures, "arm64")
}
