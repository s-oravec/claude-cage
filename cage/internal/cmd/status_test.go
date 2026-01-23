package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
)

func TestStatusCmd(t *testing.T) {
	// Create temp dir for test cages
	tmpDir := t.TempDir()
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(tmpDir)
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("status of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("status shows cage info", func(t *testing.T) {
		// Create test state
		state := &cage.State{
			Name:      "testcage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			IP:        "192.168.100.2",
			StartedAt: time.Now().Add(-2 * time.Hour),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status", "testcage"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "testcage") {
			t.Error("output should contain cage name")
		}
		if !contains(output, "running") {
			t.Error("output should contain status")
		}
		if !contains(output, "192.168.100.2") {
			t.Error("output should contain IP")
		}

		// Cleanup
		cage.DeleteState("testcage")
	})

	t.Run("status --json outputs valid JSON", func(t *testing.T) {
		// Create test state
		state := &cage.State{
			Name:      "jsoncage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "heavy",
			IP:        "192.168.100.3",
			StartedAt: time.Now().Add(-30 * time.Minute),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status", "jsoncage", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("invalid JSON output: %v", err)
		}

		if result["name"] != "jsoncage" {
			t.Error("JSON should contain cage name")
		}

		// Cleanup
		cage.DeleteState("jsoncage")
	})

	t.Run("status requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})
}

func TestStatusFormat(t *testing.T) {
	// Test uptime formatting
	tests := []struct {
		duration time.Duration
		contains string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour + 15 * time.Minute, "2h15m"},
		{25 * time.Hour, "1d1h"},
	}

	for _, tc := range tests {
		startedAt := time.Now().Add(-tc.duration)
		result := formatUptime(startedAt)
		if !contains(result, tc.contains) {
			t.Errorf("formatUptime(%v) = %s, expected to contain %s", tc.duration, result, tc.contains)
		}
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
