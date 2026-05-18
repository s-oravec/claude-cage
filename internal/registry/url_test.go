package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveURL_PassesAbsolute(t *testing.T) {
	c, _ := NewClient("api.example", Options{Insecure: true})
	assert.Equal(t, "https://storage.example/x", c.resolveURL("https://storage.example/x"))
	assert.Equal(t, "http://storage.example/x", c.resolveURL("http://storage.example/x"))
}

func TestResolveURL_PrependsBaseForRelative(t *testing.T) {
	c, _ := NewClient("api.example", Options{Insecure: true})
	assert.Equal(t, "http://api.example/api/v1/x", c.resolveURL("/api/v1/x"))
}
