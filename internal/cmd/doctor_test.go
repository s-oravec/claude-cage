package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctorCmd_Exists(t *testing.T) {
	cmd := NewDoctorCmd()

	assert.Equal(t, "doctor", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestDoctorCmd_RunsChecks(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewDoctorCmd()
	cmd.SetOut(buf)

	// Doctor command should run without panicking
	// Actual pass/fail depends on system state
	err := cmd.Execute()

	// Should not error even if checks fail
	assert.NoError(t, err)

	output := buf.String()
	// Should contain check names
	assert.Contains(t, output, "KVM")
}
