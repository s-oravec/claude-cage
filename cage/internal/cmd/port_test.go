package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
)

func TestPortCmd(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("port has subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "--help"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "list") {
			t.Error("port should have list subcommand")
		}
		if !contains(output, "add") {
			t.Error("port should have add subcommand")
		}
		if !contains(output, "remove") {
			t.Error("port should have remove subcommand")
		}
	})

	t.Run("port list of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "list", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("port list shows empty message", func(t *testing.T) {
		// Create test state without ports
		state := &cage.State{
			Name:      "noports",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("noports")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "list", "noports"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "No port forwards") {
			t.Error("should indicate no port forwards")
		}
	})

	t.Run("port list shows configured ports", func(t *testing.T) {
		// Create test state with ports
		state := &cage.State{
			Name:      "withports",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
			Ports: []cage.Port{
				{Host: 8080, Guest: 80, Protocol: "tcp", Bind: "127.0.0.1"},
				{Host: 5432, Guest: 5432, Protocol: "tcp", Bind: "127.0.0.1"},
			},
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("withports")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "list", "withports"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "8080") {
			t.Error("should show port 8080")
		}
		if !contains(output, "5432") {
			t.Error("should show port 5432")
		}
	})

	t.Run("port add requires running cage", func(t *testing.T) {
		// Create stopped cage
		state := &cage.State{
			Name:      "stoppedcage",
			Status:    cage.StatusStopped,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("stoppedcage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "add", "stoppedcage", "8080:80"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for stopped cage")
		}
	})

	t.Run("port add detects duplicates", func(t *testing.T) {
		// Create running cage with existing port
		state := &cage.State{
			Name:      "dupcage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
			Ports: []cage.Port{
				{Host: 8080, Guest: 80, Protocol: "tcp", Bind: "127.0.0.1"},
			},
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("dupcage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "add", "dupcage", "8080:8080"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for duplicate port")
		}
	})

	t.Run("port remove of non-existent port", func(t *testing.T) {
		// Create running cage
		state := &cage.State{
			Name:      "removecage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("removecage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "remove", "removecage", "9999"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent port")
		}
	})

	t.Run("port remove works", func(t *testing.T) {
		// Create running cage with port
		state := &cage.State{
			Name:      "removeok",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
			Ports: []cage.Port{
				{Host: 8080, Guest: 80, Protocol: "tcp", Bind: "127.0.0.1"},
			},
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("removeok")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "remove", "removeok", "8080"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify port was removed
		updated, _ := cage.LoadState("removeok")
		if len(updated.Ports) != 0 {
			t.Error("port should have been removed")
		}
	})

	t.Run("port add requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "add"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("port remove requires invalid port number", func(t *testing.T) {
		// Create running cage
		state := &cage.State{
			Name:      "invalidport",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("invalidport")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"port", "remove", "invalidport", "notanumber"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for invalid port number")
		}
	})
}
