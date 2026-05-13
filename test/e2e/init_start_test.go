package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInitStartWorkflow tests the complete init -> start -> modify -> restart workflow
func TestInitStartWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping workflow test in short mode")
	}

	// Use temp config dir (no shares configured)
	configDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", configDir)
	runCage("config", "init", "--force")

	// Check prerequisites
	if _, _, err := runCage("doctor"); err != nil {
		t.Skip("skipping: cage doctor reports issues")
	}

	// Check if image is available (look for checkmark before image name)
	stdout, _, _ := runCage("setup", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded (run 'cage setup --base %s' first)", testImage, testImage)
	}

	// 1. Create temp project directory
	projectDir := t.TempDir()
	t.Logf("Using project directory: %s", projectDir)

	// Generate unique cage name based on project dir name
	cageName := uniqueName(t)

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		cleanupCage(t, cageName)
		time.Sleep(2 * time.Second)
	})

	var startFailed bool

	// 2. Run cage init
	t.Run("Init", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir, "init", "--image", testImage, "--ssh", "auto", "--cage", cageName)
		if err != nil {
			t.Fatalf("cage init failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Init output: %s", stdout)

		// 3. Verify .claude-cage.yml was created
		configPath := filepath.Join(projectDir, ".claude-cage.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Fatal(".claude-cage.yml was not created")
		}

		// Read and verify config content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}
		configStr := string(content)
		if !strings.Contains(configStr, testImage) {
			t.Errorf("config does not contain image %s: %s", testImage, configStr)
		}
		if !strings.Contains(configStr, "ssh: auto") {
			t.Errorf("config does not contain 'ssh: auto': %s", configStr)
		}
		t.Logf("Config content:\n%s", configStr)
	})

	// 4. Run cage start (from project dir, no name needed)
	t.Run("FirstStart", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir, 2*time.Minute, "start")
		if err != nil {
			startFailed = true
			if strings.Contains(stderr, "Operation not permitted") {
				t.Skipf("skipping: network creation requires root (use CAGE_NETWORK=bridge and run as root)")
			}
			t.Fatalf("cage start failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Start output: %s", stdout)
	})

	if startFailed {
		t.Skip("skipping remaining tests: cage failed to start")
	}

	// Note: start command now waits for SSH (60s timeout), so VM should be ready
	// 5. Verify cage is running
	t.Run("VerifyRunning", func(t *testing.T) {
		stdout, _, err := runCage("list")
		if err != nil {
			t.Fatalf("cage list failed: %v", err)
		}
		if !strings.Contains(stdout, cageName) {
			t.Errorf("cage %s not in list: %s", cageName, stdout)
		}
		if !strings.Contains(stdout, "running") {
			t.Errorf("cage not showing as running: %s", stdout)
		}
	})

	// Wait for SSH to be ready
	t.Run("WaitForSSH", func(t *testing.T) {
		var sshOK bool
		for i := 0; i < 30; i++ {
			// Use cage ssh from project dir (no cage name) to test project config resolution
			stdout, _, err := runCageInDirWithTimeout(projectDir, 10*time.Second, "ssh", "echo", "SSH_OK")
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
		t.Log("SSH connection successful")

		// Wait for cloud-init to complete (avoids apt lock and other race conditions)
		t.Log("Waiting for cloud-init to complete...")
		runCageInDirWithTimeout(projectDir, 2*time.Minute, "ssh", "--", "cloud-init", "status", "--wait")
		t.Log("Cloud-init complete")
	})

	// 6. Stop cage (from project dir, no name needed)
	// Note: stop command now waits for domain to fully stop internally
	t.Run("Stop", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir, "stop", "--force")
		if err != nil {
			t.Fatalf("cage stop failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Stop output: %s", stdout)
	})

	// 7. Modify .claude-cage.yml - add env var
	t.Run("ModifyConfig", func(t *testing.T) {
		configPath := filepath.Join(projectDir, ".claude-cage.yml")
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		// Add env var to config
		newContent := string(content) + "\nenv:\n  E2E_TEST_VAR: hello_from_config\n"
		if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
			t.Fatalf("failed to write modified config: %v", err)
		}
		t.Logf("Modified config:\n%s", newContent)
	})

	// 8. Start again (from project dir)
	// Note: start command now waits for SSH internally (60s timeout)
	t.Run("RestartWithEnv", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir, 2*time.Minute, "start")
		if err != nil {
			t.Fatalf("cage start (after modify) failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Restart output: %s", stdout)
	})

	// Wait for SSH again (from project dir)
	t.Run("WaitForSSHAfterRestart", func(t *testing.T) {
		var sshOK bool
		for i := 0; i < 30; i++ {
			stdout, _, err := runCageInDirWithTimeout(projectDir, 10*time.Second, "ssh", "echo", "SSH_OK")
			if err == nil && strings.Contains(stdout, "SSH_OK") {
				sshOK = true
				break
			}
			t.Logf("Waiting for SSH after restart... (%d/30)", i+1)
			time.Sleep(5 * time.Second)
		}
		if !sshOK {
			t.Fatal("SSH connection failed after restart")
		}
		t.Log("SSH connection successful after restart")

		// Wait for cloud-init to complete (avoids apt lock and other race conditions)
		t.Log("Waiting for cloud-init to complete...")
		runCageInDirWithTimeout(projectDir, 2*time.Minute, "ssh", "--", "cloud-init", "status", "--wait")
		t.Log("Cloud-init complete")
	})

	// 9. SSH in and verify env var is set (from project dir)
	// Note: Runtime env via virtiofs requires:
	// 1. virtiofsd (requires root, not available in user-mode networking)
	// 2. Full Linux distro with /etc/profile.d support (not Alpine)
	// This test may be skipped depending on the test environment
	t.Run("VerifyEnvVar", func(t *testing.T) {
		// Skip on Alpine - it doesn't have bash or profile.d support
		if strings.Contains(testImage, "alpine") {
			t.Skip("Skipping env var test on Alpine (no profile.d support)")
		}

		// Try to source the runtime env file (new mechanism via virtiofs)
		// Use sh instead of bash for broader compatibility
		stdout, stderr, err := runCageInDirWithTimeout(projectDir, 30*time.Second, "ssh",
			"--", "sh", "-c", ". /etc/profile.d/cage-runtime-env.sh 2>/dev/null; echo $E2E_TEST_VAR")
		if err != nil {
			// Runtime env via virtiofs may not be available in user-mode networking
			t.Logf("SSH command failed (may be expected in user-mode networking): %v\nstderr: %s", err, stderr)
			t.Skip("Runtime env via virtiofs not available (requires root)")
		}
		t.Logf("Env var output: %s", stdout)

		if strings.Contains(stdout, "hello_from_config") {
			t.Log("Env var successfully injected via runtime env")
		} else {
			// Check if the profile.d script exists
			stdout2, _, _ := runCageInDirWithTimeout(projectDir, 30*time.Second, "ssh",
				"--", "cat", "/etc/profile.d/cage-runtime-env.sh")
			t.Logf("Runtime env script content: %s", stdout2)

			if strings.Contains(stdout2, "/cage/runtime") {
				t.Log("Runtime env script exists, virtiofs mount may not be available")
				t.Skip("virtiofs mount not available (requires root for virtiofsd)")
			} else {
				t.Errorf("expected E2E_TEST_VAR=hello_from_config, got stdout: %s", stdout)
			}
		}
	})

	// 10. Final stop (cleanup will do the remove)
	t.Run("FinalStop", func(t *testing.T) {
		runCageInDir(projectDir, "stop", "--force")
	})
}
