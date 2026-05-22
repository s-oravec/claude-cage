package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/s-oravec/cage/internal/cage"
)

func TestRestartCmd(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("restart of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"restart", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("restart requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"restart"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("restart fails on stopped cage", func(t *testing.T) {
		// Create stopped cage
		state := &cage.State{
			Name:      "stoppedrestart",
			Status:    cage.StatusStopped,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("stoppedrestart")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"restart", "stoppedrestart"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for stopped cage")
		}
	})

	t.Run("restart has force flag", func(t *testing.T) {
		cmd := NewRestartCmd()
		flag := cmd.Flags().Lookup("force")
		if flag == nil {
			t.Error("restart should have --force flag")
		}
	})
}
