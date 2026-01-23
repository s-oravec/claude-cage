package network

import (
	"testing"
)

func TestVerificationResult(t *testing.T) {
	// Test passed result
	r := VerificationResult{
		TestName: "Test 1",
		Passed:   true,
		Message:  "OK",
	}

	if !r.Passed {
		t.Error("result should be passed")
	}
	if r.TestName != "Test 1" {
		t.Errorf("TestName = %q, want %q", r.TestName, "Test 1")
	}

	// Test failed result
	r2 := VerificationResult{
		TestName: "Test 2",
		Passed:   false,
		Message:  "FAILED",
	}

	if r2.Passed {
		t.Error("result should not be passed")
	}
}

// Note: Actual VerifyIsolation tests require a running VM
// These are integration tests and would be run manually

func TestVerificationResult_AllFields(t *testing.T) {
	tests := []struct {
		name     string
		result   VerificationResult
		passed   bool
		testName string
		message  string
	}{
		{
			name: "passed test",
			result: VerificationResult{
				TestName: "Internet access",
				Passed:   true,
				Message:  "OK",
			},
			passed:   true,
			testName: "Internet access",
			message:  "OK",
		},
		{
			name: "failed test",
			result: VerificationResult{
				TestName: "DNS resolution",
				Passed:   false,
				Message:  "FAILED (expected to succeed)",
			},
			passed:   false,
			testName: "DNS resolution",
			message:  "FAILED (expected to succeed)",
		},
		{
			name: "blocked test",
			result: VerificationResult{
				TestName: "192.168.0.0/16 blocked",
				Passed:   true,
				Message:  "OK (correctly blocked)",
			},
			passed:   true,
			testName: "192.168.0.0/16 blocked",
			message:  "OK (correctly blocked)",
		},
		{
			name: "security issue",
			result: VerificationResult{
				TestName: "10.0.0.0/8 blocked",
				Passed:   false,
				Message:  "SECURITY ISSUE: should have been blocked!",
			},
			passed:   false,
			testName: "10.0.0.0/8 blocked",
			message:  "SECURITY ISSUE: should have been blocked!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Passed != tt.passed {
				t.Errorf("Passed = %v, want %v", tt.result.Passed, tt.passed)
			}
			if tt.result.TestName != tt.testName {
				t.Errorf("TestName = %q, want %q", tt.result.TestName, tt.testName)
			}
			if tt.result.Message != tt.message {
				t.Errorf("Message = %q, want %q", tt.result.Message, tt.message)
			}
		})
	}
}
