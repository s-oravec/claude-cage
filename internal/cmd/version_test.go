package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCmd_Output(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd()
	cmd.SetOut(buf)

	err := cmd.Execute()

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "cage version")
	assert.Contains(t, output, "QEMU/KVM")
}

func TestVersionCmd_ContainsVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd()
	cmd.SetOut(buf)

	cmd.Execute()

	output := buf.String()
	// Should contain semver-like version
	assert.Regexp(t, `\d+\.\d+\.\d+`, output)
}
