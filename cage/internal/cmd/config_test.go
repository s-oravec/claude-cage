package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigCmd_Exists(t *testing.T) {
	cmd := NewConfigCmd()

	assert.Equal(t, "config", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestConfigCmd_HasSubcommands(t *testing.T) {
	cmd := NewConfigCmd()

	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}

	assert.Contains(t, names, "show")
	assert.Contains(t, names, "path")
	assert.Contains(t, names, "init")
	assert.Contains(t, names, "edit")
}

func TestConfigPathCmd_Output(t *testing.T) {
	cmd := NewConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"path"})

	err := cmd.Execute()

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), ".claude-cage")
	assert.Contains(t, buf.String(), "config.yaml")
}
