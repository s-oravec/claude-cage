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
