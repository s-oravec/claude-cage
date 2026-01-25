package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// uniqueName generates a unique cage name for testing
func uniqueName(t *testing.T) string {
	return fmt.Sprintf("e2e-%s-%d", t.Name(), time.Now().UnixNano()%10000)
}
