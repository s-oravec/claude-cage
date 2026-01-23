package network

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// VerificationResult holds the result of an isolation test
type VerificationResult struct {
	TestName string
	Passed   bool
	Message  string
}

// VerifyIsolation runs network isolation tests on a cage
func VerifyIsolation(cageName, cageIP string, sshKeyPath string) ([]VerificationResult, error) {
	var results []VerificationResult

	// Test 1: Can reach public internet
	result := runSSHTest(cageName, cageIP, sshKeyPath,
		"Internet access",
		"curl -s --max-time 10 https://google.com > /dev/null && echo OK",
		true)
	results = append(results, result)

	// Test 2: DNS resolution works
	result = runSSHTest(cageName, cageIP, sshKeyPath,
		"DNS resolution",
		"host google.com > /dev/null && echo OK",
		true)
	results = append(results, result)

	// Test 3: Cannot reach 192.168.0.0/16 (should fail)
	result = runSSHTest(cageName, cageIP, sshKeyPath,
		"192.168.0.0/16 blocked",
		"ping -c 1 -W 2 192.168.1.1 > /dev/null 2>&1",
		false)
	results = append(results, result)

	// Test 4: Cannot reach 10.0.0.0/8 (should fail)
	result = runSSHTest(cageName, cageIP, sshKeyPath,
		"10.0.0.0/8 blocked",
		"ping -c 1 -W 2 10.0.0.1 > /dev/null 2>&1",
		false)
	results = append(results, result)

	// Test 5: Cannot reach 172.16.0.0/12 (should fail)
	result = runSSHTest(cageName, cageIP, sshKeyPath,
		"172.16.0.0/12 blocked",
		"ping -c 1 -W 2 172.16.0.1 > /dev/null 2>&1",
		false)
	results = append(results, result)

	// Test 6: Cannot reach link-local (should fail)
	result = runSSHTest(cageName, cageIP, sshKeyPath,
		"169.254.0.0/16 blocked",
		"ping -c 1 -W 2 169.254.169.254 > /dev/null 2>&1",
		false)
	results = append(results, result)

	return results, nil
}

// runSSHTest runs a command via SSH and checks if it passes/fails as expected
func runSSHTest(cageName, cageIP, sshKeyPath, testName, command string, expectSuccess bool) VerificationResult {
	result := VerificationResult{
		TestName: testName,
	}

	args := []string{
		"-i", sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		fmt.Sprintf("cage@%s", cageIP),
		command,
	}

	cmd := exec.Command("ssh", args...)
	err := cmd.Run()

	commandSucceeded := err == nil

	if expectSuccess {
		if commandSucceeded {
			result.Passed = true
			result.Message = "OK"
		} else {
			result.Passed = false
			result.Message = "FAILED (expected to succeed)"
		}
	} else {
		if !commandSucceeded {
			result.Passed = true
			result.Message = "OK (correctly blocked)"
		} else {
			result.Passed = false
			result.Message = "SECURITY ISSUE: should have been blocked!"
		}
	}

	return result
}

// QuickCheck does a quick connectivity check
func QuickCheck(cageIP, sshKeyPath string) error {
	args := []string{
		"-i", sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		fmt.Sprintf("cage@%s", cageIP),
		"echo ok",
	}

	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SSH check failed: %w", err)
	}

	if !strings.Contains(string(output), "ok") {
		return errors.New("unexpected SSH output")
	}

	return nil
}

// WaitForConnectivity waits for SSH to become available
func WaitForConnectivity(cageIP, sshKeyPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := QuickCheck(cageIP, sshKeyPath); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return errors.New("timeout waiting for SSH connectivity")
}
