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
	testImage   = "alpine-3.21"
	networkMode = "" // empty = default (auto), or "bridge"
)

func init() {
	// Find cage binary
	cageBin = os.Getenv("CAGE_BIN")
	if cageBin == "" {
		// Default to project root
		cageBin = filepath.Join("..", "..", "cage")
	}
	// Convert to absolute path for reliability
	if abs, err := filepath.Abs(cageBin); err == nil {
		cageBin = abs
	}

	// Allow override via env
	if img := os.Getenv("CAGE_TEST_IMAGE"); img != "" {
		testImage = img
	}

	// Network mode: default is auto (user-mode), set CAGE_NETWORK=bridge for bridge mode
	networkMode = os.Getenv("CAGE_NETWORK")
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
		runCage("remove", name, "--force")
		// Give libvirt time to cleanup
		time.Sleep(2 * time.Second)
	})

	// Start cage (creates and starts in one step)
	var startFailed bool
	t.Run("Start", func(t *testing.T) {
		args := []string{"start", name, "-i", testImage, "-p", "light", "--ssh", "auto"}
		if networkMode == "bridge" {
			args = append(args, "--network", "bridge")
		}

		stdout, stderr, err := runCageWithTimeout(2*time.Minute, args...)
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

// TestCageStartInvalidImage tests starting with invalid image
func TestCageStartInvalidImage(t *testing.T) {
	name := uniqueName(t)
	_, _, err := runCage("start", name, "-i", "nonexistent-image-12345")
	if err == nil {
		t.Error("expected error for nonexistent image")
		runCage("remove", name, "--force")
	}
}

// runCageInDir executes the cage CLI with given arguments from a specific directory
func runCageInDir(dir string, args ...string) (string, string, error) {
	cmd := exec.Command(cageBin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCageInDirWithTimeout executes cage with a timeout from a specific directory
func runCageInDirWithTimeout(dir string, timeout time.Duration, args ...string) (string, string, error) {
	cmd := exec.Command(cageBin, args...)
	cmd.Dir = dir
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
	cageName := fmt.Sprintf("e2e-init-%d", time.Now().UnixNano()%10000)

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		runCage("stop", cageName, "--force")
		runCage("remove", cageName, "--force")
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

	// Wait for VM to boot
	t.Log("Waiting for VM to boot...")
	time.Sleep(10 * time.Second)

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
	})

	// 6. Stop cage (from project dir, no name needed)
	t.Run("Stop", func(t *testing.T) {
		stdout, stderr, err := runCageInDir(projectDir, "stop", "--force")
		if err != nil {
			t.Fatalf("cage stop failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Stop output: %s", stdout)
	})

	// Wait for stop to complete (virsh destroy is async)
	time.Sleep(5 * time.Second)

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
	t.Run("RestartWithEnv", func(t *testing.T) {
		stdout, stderr, err := runCageInDirWithTimeout(projectDir, 2*time.Minute, "start")
		if err != nil {
			t.Fatalf("cage start (after modify) failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		t.Logf("Restart output: %s", stdout)
	})

	// Wait for VM to boot
	t.Log("Waiting for VM to boot after restart...")
	time.Sleep(10 * time.Second)

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

		// Give time for cloud-init to set up env vars
		time.Sleep(5 * time.Second)

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
	stdout, _, _ := runCage("setup", "--list")
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, testImage) {
		t.Skipf("skipping: image %s not downloaded (run 'cage setup --base %s' first)", testImage, testImage)
	}

	// Create two project directories
	projectDir1 := t.TempDir()
	projectDir2 := t.TempDir()

	cageName1 := fmt.Sprintf("e2e-save-%d", time.Now().UnixNano()%10000)
	cageName2 := fmt.Sprintf("e2e-reuse-%d", time.Now().UnixNano()%10000)
	customImageName := fmt.Sprintf("e2e-custom-%d", time.Now().UnixNano()%10000)

	t.Logf("Project dir 1: %s (cage: %s)", projectDir1, cageName1)
	t.Logf("Project dir 2: %s (cage: %s)", projectDir2, cageName2)
	t.Logf("Custom image name: %s", customImageName)

	// Cleanup on exit
	t.Cleanup(func() {
		t.Log("Cleaning up...")
		runCage("stop", cageName1, "--force")
		runCage("remove", cageName1, "--force")
		runCage("stop", cageName2, "--force")
		runCage("remove", cageName2, "--force")
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

		// Verify image was created
		stdout, _, err = runCage("image", "list")
		if err != nil {
			t.Fatalf("cage image list failed: %v", err)
		}
		if !strings.Contains(stdout, customImageName) {
			t.Errorf("custom image %s not in list: %s", customImageName, stdout)
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

// TestCageStartDuplicate tests starting a cage that already exists and is running
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
		runCage("remove", name, "--force")
		time.Sleep(2 * time.Second)
	})

	args := []string{"start", name, "-i", testImage, "-p", "light"}
	if networkMode == "bridge" {
		args = append(args, "--network", "bridge")
	}

	// First start should succeed
	_, stderr, err := runCageWithTimeout(2*time.Minute, args...)
	if err != nil {
		if strings.Contains(stderr, "Operation not permitted") {
			t.Skipf("skipping: network creation requires root")
		}
		t.Fatalf("first start failed: %v", err)
	}

	// Second start should fail (already running)
	_, _, err = runCage(args...)
	if err == nil {
		t.Error("expected error for already running cage")
	}
}
