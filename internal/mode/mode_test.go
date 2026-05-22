package mode

import (
	"testing"

	"github.com/s-oravec/cage/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestMode_String(t *testing.T) {
	assert.Equal(t, "user", User.String())
	assert.Equal(t, "root", Root.String())
}

func TestMode_URI(t *testing.T) {
	assert.Equal(t, "qemu:///session", User.URI())
	assert.Equal(t, "qemu:///system", Root.URI())
}

func TestRequiredFromConfig_NilIsUser(t *testing.T) {
	assert.Equal(t, User, RequiredFromConfig(nil))
}

func TestRequiredFromConfig_EmptyIsUser(t *testing.T) {
	cfg := &config.ResolvedConfig{}
	assert.Equal(t, User, RequiredFromConfig(cfg))
}

func TestRequiredFromConfig_SharesForcesRoot(t *testing.T) {
	cfg := &config.ResolvedConfig{
		Shares: []config.ShareConfig{{Host: "/tmp", Guest: "/mnt"}},
	}
	assert.Equal(t, Root, RequiredFromConfig(cfg))
}

func TestRequiredFromConfig_EnvForcesRoot(t *testing.T) {
	cfg := &config.ResolvedConfig{
		Env: map[string]string{"FOO": "bar"},
	}
	assert.Equal(t, Root, RequiredFromConfig(cfg))
}
