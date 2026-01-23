package cmd

import (
	"bytes"
	"testing"
)

func TestConsoleCmd(t *testing.T) {
	t.Run("console requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"console"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("console of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"console", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("console has help", func(t *testing.T) {
		cmd := NewConsoleCmd()
		if cmd.Short == "" {
			t.Error("console should have short description")
		}
		if cmd.Long == "" {
			t.Error("console should have long description")
		}
	})
}
