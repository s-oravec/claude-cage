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
		"wget -q -O /dev/null --timeout=10 https://google.com && echo OK",
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

// VerifyIsolationWithPort runs network isolation tests via SSH port forwarding
func VerifyIsolationWithPort(cageName, host string, port int, sshKeyPath string) ([]VerificationResult, error) {
	var results []VerificationResult

	// Test 1: Can reach public internet (use netcat to avoid DNS/redirect issues)
	// Test TCP connection to Google's IP
	result := runSSHTestWithPort(host, port, sshKeyPath,
		"Internet access (VM)",
		"timeout 5 nc -zv 142.250.180.100 80 2>&1 && echo OK",
		true)
	results = append(results, result)

	// Test 2: Cannot reach host's SSH (10.0.0.55:22)
	result = runSSHTestWithPort(host, port, sshKeyPath,
		"Host SSH (10.0.0.55:22) blocked",
		"timeout 3 nc -zv 10.0.0.55 22 2>&1 || echo BLOCKED",
		false)
	// Invert logic - we expect the command to output BLOCKED
	if strings.Contains(result.Message, "BLOCKED") {
		result.Passed = true
		result.Message = "OK (correctly blocked)"
	}
	results = append(results, result)

	// Test 3: Cannot reach private IPs
	privateTests := []struct {
		name string
		ip   string
	}{
		{"192.168.0.0/16 blocked", "192.168.1.1"},
		{"10.0.0.0/8 blocked", "10.0.0.1"},
		{"172.16.0.0/12 blocked", "172.16.0.1"},
	}

	for _, test := range privateTests {
		result := runSSHTestWithPort(host, port, sshKeyPath,
			test.name,
			fmt.Sprintf("timeout 2 nc -zv %s 80 2>&1", test.ip),
			false)
		results = append(results, result)
	}

	return results, nil
}

// runSSHTestWithPort runs a command via SSH with port forwarding
func runSSHTestWithPort(host string, port int, sshKeyPath, testName, command string, expectSuccess bool) VerificationResult {
	result := VerificationResult{
		TestName: testName,
	}

	args := []string{
		"-i", sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("cage@%s", host),
		command,
	}

	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()

	commandSucceeded := err == nil

	if expectSuccess {
		if commandSucceeded {
			result.Passed = true
			result.Message = "OK"
		} else {
			result.Passed = false
			result.Message = fmt.Sprintf("FAILED: %s", strings.TrimSpace(string(output)))
		}
	} else {
		if !commandSucceeded || strings.Contains(string(output), "BLOCKED") || strings.Contains(string(output), "timed out") || strings.Contains(string(output), "refused") {
			result.Passed = true
			result.Message = "OK (correctly blocked)"
		} else {
			result.Passed = false
			result.Message = "SECURITY ISSUE: should have been blocked!"
		}
	}

	return result
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
