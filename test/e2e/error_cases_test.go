package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestCageStartInvalidImage tests starting with invalid image
func TestCageStartInvalidImage(t *testing.T) {
	projectDir := t.TempDir()
	name := uniqueName(t)

	// Init with invalid image
	_, _, err := runCageInDir(projectDir, "init", "--image", "nonexistent-image-12345", "--cage", name)
	if err != nil {
		// Init itself might fail for invalid image (if validation is done there)
		return
	}

	// If init succeeds, start should fail
	_, _, err = runCageInDirWithTimeout(projectDir, 30*time.Second, "start")
	if err == nil {
		t.Error("expected error for nonexistent image")
		runCage("remove", name, "--force")
	}
}

// TestCageStartDuplicate tests starting a cage that already exists and is running
func TestCageStartDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Use temp config dir (no shares configured)
	configDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", configDir)
	runCage("config", "init", "--force")

	// Check if image is available (look for ✓ before image name)
	stdout, _, _ := runCage("pull", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded", testImage)
	}

	// Create project directory for init+start workflow
	projectDir := t.TempDir()
	name := uniqueName(t)

	t.Cleanup(func() {
		cleanupCage(t, name)
		time.Sleep(2 * time.Second)
	})

	// Init cage
	_, _, err := runCageInDir(projectDir, "init", "--image", testImage, "--cage", name)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// First start should succeed
	_, stderr, err := runCageInDirWithTimeout(projectDir, 2*time.Minute, "start")
	if err != nil {
		if strings.Contains(stderr, "Operation not permitted") {
			t.Skipf("skipping: network creation requires root")
		}
		t.Fatalf("first start failed: %v", err)
	}

	// Second start should fail (already running)
	_, _, err = runCageInDir(projectDir, "start")
	if err == nil {
		t.Error("expected error for already running cage")
	}
}
