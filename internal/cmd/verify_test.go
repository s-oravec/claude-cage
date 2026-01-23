package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerifyCmd(t *testing.T) {
	cmd := NewVerifyCmd()

	if cmd.Use != "verify [name]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "verify [name]")
	}

	if !strings.Contains(cmd.Short, "isolation") {
		t.Error("Short should mention isolation")
	}
}

func TestVerifyCmd_NoCageName(t *testing.T) {
	cmd := NewVerifyCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Should require cage name
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing cage name")
	}
}

func TestVerifyCmd_NonexistentCage(t *testing.T) {
	cmd := NewVerifyCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"nonexistent-cage-12345"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent cage")
	}
}
