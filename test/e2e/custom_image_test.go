package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestCustomImageSaveAndReuse tests saving a cage as image and reusing it
// This tests that saved images have SSH keys properly cleaned so new cages can connect
func TestCustomImageSaveAndReuse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping custom image test in short mode")
	}

	// Use temp config dir
	configDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", configDir)
	runCage("config", "init", "--force")

	// Check prerequisites
	if _, _, err := runCage("doctor"); err != nil {
		t.Skip("skipping: cage doctor reports issues")
	}

	// Check if image is available
	stdout, _, _ := runCage("pull", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded (run 'cage pull --base %s' first)", testImage, testImage)
	}

	// Create two project directories
	projectDir1 := t.TempDir()
	projectDir2 := t.TempDir()

	cageName1 := uniqueName(t, "save")
	cageName2 := uniqueName(t, "reuse")
	customImageName := uniqueName(t, "custom")

	t.Logf("Project dir 1: %s (cage: %s)", projectDir1, cageName1)
	t.Logf("Project dir 2: %s (cage: %s)", projectDir2, cageName2)
	t.Logf("Custom image name: %s", customImageName)

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		cleanupCage(t, cageName1)
		cleanupCage(t, cageName2)
		runCage("image", "remove", customImageName, "--force")
		time.Sleep(2 * time.Second)
	})

	var startFailed bool

	// Step 1: cage init in project dir 1
	t.Run("Step1_Init", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir1, "init", "--image", testImage, "--ssh", "auto", "--cage", cageName1)
		if err != nil {
			t.Fatalf("cage init failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Init output: %s", stdout)
	})

	// Step 2: cage start
	t.Run("Step2_Start", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir1, 2*time.Minute, "start")
		if err != nil {
			startFailed = true
			if strings.Contains(stderr, "Operation not permitted") {
				t.Skipf("skipping: network creation requires root")
			}
			t.Fatalf("cage start failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Start output: %s", stdout)
	})

	if startFailed {
		t.Skip("skipping remaining tests: cage failed to start")
	}

	// Wait for VM to boot and SSH to be ready
	t.Log("Waiting for VM to boot...")
	time.Sleep(10 * time.Second)

	t.Run("Step2_WaitForSSH", func(t *testing.T) {
		var sshOK bool
		for i := 0; i < 30; i++ {
			stdout, _, err := runCageInDirWithTimeout(projectDir1, 10*time.Second, "ssh", "echo", "SSH_OK")
			if err == nil && strings.Contains(stdout, "SSH_OK") {
				sshOK = true
				break
			}
			t.Logf("Waiting for SSH... (%d/30)", i+1)
			time.Sleep(5 * time.Second)
		}
		if !sshOK {
			t.Fatal("SSH connection failed after 150s")
		}
		t.Log("SSH connection to original cage successful")
	})

	// Step 3: cage stop
	t.Run("Step3_Stop", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir1, "stop")
		if err != nil {
			t.Fatalf("cage stop failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Stop output: %s", stdout)
	})

	// Wait for stop to complete
	time.Sleep(5 * time.Second)

	// Step 4: cage image save --name <custom-name>
	t.Run("Step4_ImageSave", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir1, 5*time.Minute, "image", "save", "--name", customImageName)
		if err != nil {
			t.Fatalf("cage image save failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Image save output: %s", stdout)

		// Verify image was created. Use `image inspect` rather than `image list`
		// because list truncates names to 20 chars in its table layout.
		if _, stderr, err := runCage("image", "inspect", customImageName); err != nil {
			t.Errorf("custom image %s not found via inspect: %v (stderr: %s)", customImageName, err, stderr)
		}
	})

	// Step 5: cage init --image <custom-name> in project dir 2
	t.Run("Step5_InitWithCustomImage", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir2, "init", "--image", customImageName, "--ssh", "auto", "--cage", cageName2)
		if err != nil {
			t.Fatalf("cage init with custom image failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Init with custom image output: %s", stdout)
	})

	// Step 6: cage start (new cage from custom image)
	t.Run("Step6_StartFromCustomImage", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir2, 2*time.Minute, "start")
		if err != nil {
			t.Fatalf("cage start from custom image failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Start from custom image output: %s", stdout)
	})

	// Wait for VM to boot
	t.Log("Waiting for VM (from custom image) to boot...")
	time.Sleep(10 * time.Second)

	// Step 7: cage ssh - THIS IS THE CRITICAL TEST
	// If SSH keys weren't properly cleaned during image save, this will fail with "Permission denied"
	t.Run("Step7_SSHToCustomImageCage", func(t *testing.T) {
		var sshOK bool
		for i := 0; i < 30; i++ {
			stdout, stderr, err := runCageInDirWithTimeout(projectDir2, 10*time.Second, "ssh", "echo", "SSH_OK_CUSTOM")
			if err == nil && strings.Contains(stdout, "SSH_OK_CUSTOM") {
				sshOK = true
				break
			}
			t.Logf("Waiting for SSH to custom image cage... (%d/30) err=%v stderr=%s", i+1, err, stderr)
			time.Sleep(5 * time.Second)
		}
		if !sshOK {
			t.Fatal("SSH connection to cage from custom image FAILED - this indicates SSH keys were not properly cleaned during image save")
		}
		t.Log("SSH connection to cage from custom image SUCCESSFUL")
	})

	// Verify we're in a working cage
	t.Run("Step8_VerifyCustomImageCage", func(t *testing.T) {
		stdout, _, err := runCageInDirWithTimeout(projectDir2, 30*time.Second, "ssh", "hostname")
		if err != nil {
			t.Fatalf("cage ssh hostname failed: %v", err)
		}
		t.Logf("Hostname from custom image cage: %s", strings.TrimSpace(stdout))
	})

	// Cleanup - stop both cages
	t.Run("Cleanup", func(t *testing.T) {
		runCageInDir(projectDir1, "stop", "--force")
		runCageInDir(projectDir2, "stop", "--force")
	})
}
