package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	root := NewRootCmd()

	// Root should have version and doctor subcommands
	var names []string
	for _, sub := range root.Commands() {
		names = append(names, sub.Name())
	}

	assert.Contains(t, names, "version")
	assert.Contains(t, names, "doctor")
	assert.Contains(t, names, "config")
	assert.Contains(t, names, "pull")
	assert.Contains(t, names, "init")
	assert.Contains(t, names, "start")
	assert.Contains(t, names, "stop")
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "ssh")
	assert.Contains(t, names, "verify")
	assert.Contains(t, names, "status")
	assert.Contains(t, names, "exec")
	assert.Contains(t, names, "logs")
	assert.Contains(t, names, "port")
	assert.Contains(t, names, "restart")
	assert.Contains(t, names, "snapshot")
	assert.Contains(t, names, "image")
}

func TestRootCmd_Name(t *testing.T) {
	root := NewRootCmd()

	assert.Equal(t, "cage", root.Use)
}
