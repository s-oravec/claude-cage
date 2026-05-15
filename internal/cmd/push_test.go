package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushCmd_Args(t *testing.T) {
	c := NewPushCmd()
	assert.NotNil(t, c.Flag("latest"))
}
