package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort()
	assert.NoError(t, err)
	assert.Greater(t, port, 0)
	assert.Less(t, port, 65536)
}
