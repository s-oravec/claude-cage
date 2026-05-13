package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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

// uniqueName generates a unique cage name for testing.
// Pass optional parts to distinguish multiple cages within the same test
// (e.g. uniqueName(t, "save"), uniqueName(t, "reuse")).
func uniqueName(t *testing.T, parts ...string) string {
	name := strings.ReplaceAll(t.Name(), "/", "-")
	var rb [4]byte
	if _, err := rand.Read(rb[:]); err != nil {
		// crypto/rand can't fail on Linux; if it does, fall back to nano time
		return fmt.Sprintf("e2e-%s-%s-%d", name, strings.Join(parts, "-"), time.Now().UnixNano())
	}
	suffix := hex.EncodeToString(rb[:])
	if len(parts) > 0 {
		return "e2e-" + name + "-" + strings.Join(parts, "-") + "-" + suffix
	}
	return "e2e-" + name + "-" + suffix
}

// cleanupCage tears down a test cage robustly: it tries the normal `cage stop`
// + `cage remove --force` path first, then falls back to a direct
// `virsh undefine` so that an orphan libvirt domain (e.g. when `cage init`
// failed mid-flight and never wrote state.json) does not block future test
// runs with "domain ... already exists".
func cleanupCage(t *testing.T, name string) {
	t.Helper()
	runCage("stop", name, "--force")
	runCage("remove", name, "--force")
	cmd := exec.Command("virsh", "-c", "qemu:///session", "undefine",
		"--nvram", "--remove-all-storage",
		"--snapshots-metadata", "--checkpoints-metadata",
		"cage-"+name)
	_ = cmd.Run()
}
