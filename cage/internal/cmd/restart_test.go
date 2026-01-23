package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
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

func TestRestartConfig(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("save and load restart config", func(t *testing.T) {
		cfg := &cage.RestartConfig{
			Image:   "ubuntu-24.04",
			Profile: "heavy",
			Ports: []cage.Port{
				{Host: 8080, Guest: 80, Protocol: "tcp", Bind: "127.0.0.1"},
			},
		}

		// Need to create the cage directory first
		if err := cage.EnsureDir("testcage"); err != nil {
			t.Fatal(err)
		}

		if err := cage.SaveRestartConfig("testcage", cfg); err != nil {
			t.Fatalf("failed to save restart config: %v", err)
		}

		loaded, err := cage.LoadRestartConfig("testcage")
		if err != nil {
			t.Fatalf("failed to load restart config: %v", err)
		}

		if loaded.Image != cfg.Image {
			t.Errorf("expected image '%s', got '%s'", cfg.Image, loaded.Image)
		}
		if loaded.Profile != cfg.Profile {
			t.Errorf("expected profile '%s', got '%s'", cfg.Profile, loaded.Profile)
		}
		if len(loaded.Ports) != 1 {
			t.Errorf("expected 1 port, got %d", len(loaded.Ports))
		}

		// Cleanup
		cage.DeleteRestartConfig("testcage")
		cage.DeleteState("testcage")
	})

	t.Run("delete restart config", func(t *testing.T) {
		cfg := &cage.RestartConfig{
			Image:   "ubuntu-24.04",
			Profile: "default",
		}

		if err := cage.EnsureDir("deletecage"); err != nil {
			t.Fatal(err)
		}

		if err := cage.SaveRestartConfig("deletecage", cfg); err != nil {
			t.Fatalf("failed to save restart config: %v", err)
		}

		if err := cage.DeleteRestartConfig("deletecage"); err != nil {
			t.Fatalf("failed to delete restart config: %v", err)
		}

		_, err := cage.LoadRestartConfig("deletecage")
		if err == nil {
			t.Error("expected error loading deleted config")
		}

		// Cleanup
		cage.DeleteState("deletecage")
	})
}
