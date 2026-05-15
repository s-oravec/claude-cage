package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginCmd_Exists(t *testing.T) {
	c := NewLoginCmd()
	assert.Equal(t, "login", c.Use[:5])
}

func TestLoginCmd_Flags(t *testing.T) {
	c := NewLoginCmd()
	assert.NotNil(t, c.Flag("token-stdin"))
	assert.NotNil(t, c.Flag("list"))
}
