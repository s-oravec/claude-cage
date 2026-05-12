package e2e

import (
	"strings"
	"testing"
)

// TestCageVersion tests the version command
func TestCageVersion(t *testing.T) {
	stdout, _, err := runCage("version")
	if err != nil {
		t.Fatalf("cage version failed: %v", err)
	}
	if !strings.Contains(stdout, "cage version") {
		t.Errorf("expected 'cage version' in output, got: %s", stdout)
	}
}

// TestCageDoctor tests the doctor command
func TestCageDoctor(t *testing.T) {
	stdout, stderr, err := runCage("doctor")
	output := stdout + stderr
	if err != nil {
		// Doctor may return error if dependencies missing, but should still run
		t.Logf("doctor returned error (may be expected): %v", err)
	}
	// Accept any recognizable doctor marker. ✗ counts as evidence the
	// checks ran just as much as ✓ does — many hosts (CI macOS, fresh
	// Linux without KVM) report all-failures, and that's still valid output.
	if !strings.Contains(output, "Checking") &&
		!strings.Contains(output, "✓") &&
		!strings.Contains(output, "✗") {
		t.Errorf("expected doctor output, got: %s", output)
	}
}

// TestCageConfigInit tests config initialization
func TestCageConfigInit(t *testing.T) {
	// Use temp config dir
	tmpDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", tmpDir)

	stdout, stderr, err := runCage("config", "init", "--force")
	if err != nil {
		t.Fatalf("config init failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
}

// TestCageSetupList tests listing available images
func TestCageSetupList(t *testing.T) {
	stdout, _, err := runCage("setup", "--list")
	if err != nil {
		t.Fatalf("setup --list failed: %v", err)
	}
	if !strings.Contains(stdout, "alpine") && !strings.Contains(stdout, "ubuntu") {
		t.Errorf("expected image list, got: %s", stdout)
	}
}
