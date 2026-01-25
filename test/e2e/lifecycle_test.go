package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestCageLifecycle tests the full VM lifecycle
func TestCageLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	// Use temp config dir (no shares configured)
	tmpDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", tmpDir)
	runCage("config", "init", "--force")

	// Check prerequisites
	if _, _, err := runCage("doctor"); err != nil {
		t.Skip("skipping: cage doctor reports issues")
	}

	// Check if image is available (look for ✓ before image name)
	stdout, _, _ := runCage("setup", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded (run 'cage setup --base %s' first)", testImage, testImage)
	}

	name := uniqueName(t)
	t.Logf("Testing with cage name: %s", name)

	// Create project directory for init+start workflow
	projectDir := t.TempDir()

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		runCage("remove", name, "--force")
		// Give libvirt time to cleanup
		time.Sleep(2 * time.Second)
	})

	// Init cage (creates .claude-cage.yml)
	var startFailed bool
	t.Run("Init", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir, "init", "--image", testImage, "--cage", name, "--ssh", "auto")
		if err != nil {
			t.Fatalf("cage init failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Init output: %s", stdout)
	})

	// Start cage
	t.Run("Start", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir, 2*time.Minute, "start")
		if err != nil {
			startFailed = true
			// Check if it's a permissions issue
			if strings.Contains(stderr, "Operation not permitted") {
				t.Skipf("skipping: network creation requires root (use CAGE_NETWORK=bridge and run as root)")
			}
			t.Fatalf("cage start failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Start output: %s", stdout)
	})

	// Skip remaining tests if start failed
	if startFailed {
		t.Skip("skipping remaining tests: cage failed to start")
	}

	// Wait for VM to boot
	t.Log("Waiting for VM to boot...")
	time.Sleep(10 * time.Second)

	// List cages
	t.Run("List", func(t *testing.T) {
		stdout, _, err := runCage("list")
		if err != nil {
			t.Fatalf("cage list failed: %v", err)
		}
		if !strings.Contains(stdout, name) {
			t.Errorf("cage %s not in list: %s", name, stdout)
		}
		if !strings.Contains(stdout, "running") {
			t.Errorf("cage not showing as running: %s", stdout)
		}
	})

	// Status
	t.Run("Status", func(t *testing.T) {
		stdout, _, err := runCage("status", name)
		if err != nil {
			t.Fatalf("cage status failed: %v", err)
		}
		if !strings.Contains(stdout, "running") {
			t.Errorf("expected running status: %s", stdout)
		}
	})

	// Status JSON
	t.Run("StatusJSON", func(t *testing.T) {
		stdout, _, err := runCage("status", name, "--json")
		if err != nil {
			t.Fatalf("cage status --json failed: %v", err)
		}
		if !strings.Contains(stdout, `"status"`) {
			t.Errorf("expected JSON output: %s", stdout)
		}
	})

	// SSH tests (--ssh auto enables SSH in all network modes)
	t.Run("SSH", func(t *testing.T) {
		// Wait for SSH with retries (cloud-init needs time to install openssh on Alpine)
		var sshOK bool
		for i := 0; i < 30; i++ {
			stdout, _, err := runCageWithTimeout(10*time.Second, "ssh", name, "echo SSH_OK")
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
	})

	t.Run("Exec", func(t *testing.T) {
		stdout, _, err := runCageWithTimeout(30*time.Second, "exec", name, "--", "uname", "-a")
		if err != nil {
			t.Fatalf("cage exec failed: %v", err)
		}
		if !strings.Contains(strings.ToLower(stdout), "linux") {
			t.Errorf("expected linux in uname output: %s", stdout)
		}
	})

	t.Run("SSHCommand", func(t *testing.T) {
		stdout, _, err := runCageWithTimeout(30*time.Second, "ssh", name, "hostname")
		if err != nil {
			t.Fatalf("cage ssh hostname failed: %v", err)
		}
		if strings.TrimSpace(stdout) == "" {
			t.Error("expected hostname output")
		}
		t.Logf("Hostname: %s", strings.TrimSpace(stdout))
	})

	// Port list
	t.Run("PortList", func(t *testing.T) {
		stdout, _, err := runCage("port", "list", name)
		if err != nil {
			t.Fatalf("cage port list failed: %v", err)
		}
		// Should show either "No port" or a table header
		if !strings.Contains(stdout, "No port") && !strings.Contains(stdout, "PORT") {
			t.Errorf("unexpected port list output: %s", stdout)
		}
	})

	// Stop cage
	t.Run("Stop", func(t *testing.T) {
		stdout, stderr, err := runCage("stop", name)
		if err != nil {
			t.Fatalf("cage stop failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
	})

	// Wait for stop
	time.Sleep(3 * time.Second)

	// Verify stopped (cage should still exist but be stopped)
	t.Run("VerifyStopped", func(t *testing.T) {
		stdout, _, _ := runCage("list")
		if strings.Contains(stdout, name) && strings.Contains(stdout, "running") {
			t.Errorf("cage still showing as running: %s", stdout)
		}
	})

	// Restart the stopped cage
	t.Run("RestartStopped", func(t *testing.T) {
		stdout, stderr, err := runCageWithTimeout(2*time.Minute, "start", name)
		if err != nil {
			t.Fatalf("cage start (restart) failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		// Verify running again
		stdout, _, _ = runCage("list")
		if !strings.Contains(stdout, name) || !strings.Contains(stdout, "running") {
			t.Errorf("cage not showing as running after restart: %s", stdout)
		}
	})

	// Final stop before cleanup
	runCage("stop", name, "--force")
}
