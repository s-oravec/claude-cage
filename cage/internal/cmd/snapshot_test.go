package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
)

func TestSnapshotCmd(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("snapshot has subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "--help"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "create") {
			t.Error("snapshot should have create subcommand")
		}
		if !contains(output, "list") {
			t.Error("snapshot should have list subcommand")
		}
		if !contains(output, "restore") {
			t.Error("snapshot should have restore subcommand")
		}
		if !contains(output, "delete") {
			t.Error("snapshot should have delete subcommand")
		}
	})

	t.Run("snapshot create requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "create"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("snapshot create requires snapshot name", func(t *testing.T) {
		// Create test cage
		state := &cage.State{
			Name:      "snapcage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("snapcage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "create", "snapcage"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when snapshot name not provided")
		}
	})

	t.Run("snapshot create of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "create", "nonexistent", "--name", "snap1"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("snapshot list of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "list", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("snapshot restore requires snapshot name", func(t *testing.T) {
		// Create test cage
		state := &cage.State{
			Name:      "restorecage",
			Status:    cage.StatusStopped,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("restorecage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "restore", "restorecage"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when snapshot name not provided")
		}
	})

	t.Run("snapshot delete requires snapshot name", func(t *testing.T) {
		// Create test cage
		state := &cage.State{
			Name:      "deletecage",
			Status:    cage.StatusStopped,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("deletecage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"snapshot", "delete", "deletecage"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when snapshot name not provided")
		}
	})
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"this is a very long string", 10, "this is..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		got := truncateString(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
