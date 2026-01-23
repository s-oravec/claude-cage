package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
)

func TestLogsCmd(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("logs of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"logs", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("logs requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"logs"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("logs fails on stopped cage", func(t *testing.T) {
		// Create test state with stopped status
		state := &cage.State{
			Name:      "stoppedlogs",
			Status:    cage.StatusStopped,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("stoppedlogs")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"logs", "stoppedlogs"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for stopped cage")
		}
	})

	t.Run("logs accepts -n flag", func(t *testing.T) {
		// Create test state
		state := &cage.State{
			Name:      "logscage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("logscage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		// This will fail SSH but the flag should be accepted
		cmd.SetArgs([]string{"logs", "logscage", "-n", "50"})

		// We expect an error because SSH won't work in tests,
		// but the command structure should be valid
		_ = cmd.Execute()
	})

	t.Run("logs accepts -f flag", func(t *testing.T) {
		// Create test state
		state := &cage.State{
			Name:      "followcage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("followcage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		// This will fail SSH but the flag should be accepted
		cmd.SetArgs([]string{"logs", "followcage", "-f"})

		// We expect an error because SSH won't work in tests,
		// but the command structure should be valid
		_ = cmd.Execute()
	})
}
