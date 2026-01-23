package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	cageBin     string
	testImage   = "alpine-3.20"
	userNetwork = false
)

func init() {
	// Find cage binary
	cageBin = os.Getenv("CAGE_BIN")
	if cageBin == "" {
		// Default to project root
		cageBin = filepath.Join("..", "..", "cage")
	}

	// Allow override via env
	if img := os.Getenv("CAGE_TEST_IMAGE"); img != "" {
		testImage = img
	}

	// Check for user-network mode
	if os.Getenv("CAGE_USER_NETWORK") == "1" {
		userNetwork = true
	}
}

// runCage executes the cage CLI with given arguments
func runCage(args ...string) (string, string, error) {
	cmd := exec.Command(cageBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCageWithTimeout executes cage with a timeout
func runCageWithTimeout(timeout time.Duration, args ...string) (string, string, error) {
	cmd := exec.Command(cageBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return stdout.String(), stderr.String(), fmt.Errorf("timeout after %v", timeout)
	}
}

// uniqueName generates a unique cage name for testing
func uniqueName(t *testing.T) string {
	return fmt.Sprintf("e2e-%s-%d", t.Name(), time.Now().UnixNano()%10000)
}

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
	if !strings.Contains(output, "Checking") && !strings.Contains(output, "✓") {
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

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		runCage("stop", name, "--force")
		// Give libvirt time to cleanup
		time.Sleep(2 * time.Second)
	})

	// Start cage
	var startFailed bool
	t.Run("Start", func(t *testing.T) {
		args := []string{"start", "--name", name, "--image", testImage, "--profile", "light"}
		if userNetwork {
			args = append(args, "--user-network")
		}

		stdout, stderr, err := runCageWithTimeout(2*time.Minute, args...)
		if err != nil {
			startFailed = true
			// Check if it's a permissions issue
			if strings.Contains(stderr, "Operation not permitted") {
				t.Skipf("skipping: network creation requires root (use CAGE_USER_NETWORK=1 or run as root)")
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

	// SSH tests (skip for user-network mode)
	if !userNetwork {
		t.Run("SSH", func(t *testing.T) {
			// Wait for SSH with retries
			var sshOK bool
			for i := 0; i < 30; i++ {
				stdout, _, err := runCageWithTimeout(10*time.Second, "ssh", name, "echo", "SSH_OK")
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
	} else {
		t.Log("Skipping SSH tests (user-network mode)")
	}

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

	// Verify stopped
	t.Run("VerifyStopped", func(t *testing.T) {
		stdout, _, _ := runCage("list")
		if strings.Contains(stdout, name) && strings.Contains(stdout, "running") {
			t.Errorf("cage still showing as running: %s", stdout)
		}
	})
}

// TestCageStartInvalidImage tests starting with invalid image
func TestCageStartInvalidImage(t *testing.T) {
	name := uniqueName(t)
	_, _, err := runCage("start", "--name", name, "--image", "nonexistent-image-12345")
	if err == nil {
		t.Error("expected error for nonexistent image")
		runCage("stop", name, "--force")
	}
}

// TestCageStartDuplicate tests starting a cage that already exists
func TestCageStartDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Use temp config dir (no shares configured)
	tmpDir := t.TempDir()
	t.Setenv("CAGE_CONFIG_DIR", tmpDir)
	runCage("config", "init", "--force")

	// Check if image is available (look for ✓ before image name)
	stdout, _, _ := runCage("setup", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded", testImage)
	}

	name := uniqueName(t)
	t.Cleanup(func() {
		runCage("stop", name, "--force")
		time.Sleep(2 * time.Second)
	})

	args := []string{"start", "--name", name, "--image", testImage, "--profile", "light"}
	if userNetwork {
		args = append(args, "--user-network")
	}

	// First start should succeed
	_, stderr, err := runCageWithTimeout(2*time.Minute, args...)
	if err != nil {
		if strings.Contains(stderr, "Operation not permitted") {
			t.Skipf("skipping: network creation requires root")
		}
		t.Fatalf("first start failed: %v", err)
	}

	// Second start should fail
	_, _, err = runCage(args...)
	if err == nil {
		t.Error("expected error for duplicate cage name")
	}
}
