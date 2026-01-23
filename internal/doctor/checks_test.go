package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheck_Structure(t *testing.T) {
	// Check should have Name, CheckFunc, and Required fields
	check := Check{
		Name:     "test check",
		Required: true,
		CheckFunc: func() error {
			return nil
		},
	}

	assert.Equal(t, "test check", check.Name)
	assert.True(t, check.Required)
	assert.NotNil(t, check.CheckFunc)
}

func TestRunChecks_AllPass(t *testing.T) {
	checks := []Check{
		{Name: "check1", Required: true, CheckFunc: func() error { return nil }},
		{Name: "check2", Required: false, CheckFunc: func() error { return nil }},
	}

	results := RunChecks(checks)

	assert.Len(t, results, 2)
	assert.True(t, results[0].Passed)
	assert.True(t, results[1].Passed)
	assert.Nil(t, results[0].Error)
}

func TestRunChecks_RequiredFails(t *testing.T) {
	checks := []Check{
		{Name: "required", Required: true, CheckFunc: func() error { return assert.AnError }},
	}

	results := RunChecks(checks)

	assert.Len(t, results, 1)
	assert.False(t, results[0].Passed)
	assert.NotNil(t, results[0].Error)
}

func TestAllRequiredPassed_True(t *testing.T) {
	results := []CheckResult{
		{Check: Check{Required: true}, Passed: true},
		{Check: Check{Required: false}, Passed: false}, // optional can fail
	}

	assert.True(t, AllRequiredPassed(results))
}

func TestAllRequiredPassed_False(t *testing.T) {
	results := []CheckResult{
		{Check: Check{Required: true}, Passed: false}, // required failed
	}

	assert.False(t, AllRequiredPassed(results))
}

func TestDefaultChecks_ContainsExpected(t *testing.T) {
	checks := DefaultChecks()

	// Should have at least these checks
	names := make([]string, len(checks))
	for i, c := range checks {
		names[i] = c.Name
	}

	assert.Contains(t, names, "KVM available")
	assert.Contains(t, names, "libvirtd running")
	assert.Contains(t, names, "User in kvm group")
	assert.Contains(t, names, "User in libvirt group")
	assert.Contains(t, names, "virtiofsd installed")
	assert.Contains(t, names, "qemu-img installed")
}
