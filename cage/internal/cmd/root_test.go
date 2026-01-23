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
	assert.Contains(t, names, "setup")
}

func TestRootCmd_Name(t *testing.T) {
	root := NewRootCmd()

	assert.Equal(t, "cage", root.Use)
}
