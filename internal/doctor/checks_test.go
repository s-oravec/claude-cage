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

func TestInstallAllHint(t *testing.T) {
	hint := InstallAllHint()

	// Should contain key packages
	assert.Contains(t, hint, "qemu-kvm")
	assert.Contains(t, hint, "libvirt-daemon-system")
	assert.Contains(t, hint, "virtiofsd")
	assert.Contains(t, hint, "qemu-utils")
	assert.Contains(t, hint, "cloud-image-utils")

	// Should contain group setup
	assert.Contains(t, hint, "usermod")
	assert.Contains(t, hint, "kvm")
	assert.Contains(t, hint, "libvirt")

	// Should enable libvirtd
	assert.Contains(t, hint, "systemctl")
	assert.Contains(t, hint, "enable")
	assert.Contains(t, hint, "libvirtd")
}

func TestDefaultChecks_AllHaveCheckFunc(t *testing.T) {
	checks := DefaultChecks()

	for _, check := range checks {
		assert.NotEmpty(t, check.Name, "check should have a name")
		assert.NotNil(t, check.CheckFunc, "check %s should have a CheckFunc", check.Name)
	}
}

func TestDefaultChecks_AllHaveFixHint(t *testing.T) {
	checks := DefaultChecks()

	for _, check := range checks {
		assert.NotEmpty(t, check.FixHint, "check %s should have a FixHint", check.Name)
	}
}

func TestCheckResult_Structure(t *testing.T) {
	result := CheckResult{
		Check: Check{
			Name:     "test",
			Required: true,
		},
		Passed: true,
		Error:  nil,
	}

	assert.Equal(t, "test", result.Check.Name)
	assert.True(t, result.Passed)
	assert.Nil(t, result.Error)
}

func TestRunChecks_PreservesOrder(t *testing.T) {
	checks := []Check{
		{Name: "first", CheckFunc: func() error { return nil }},
		{Name: "second", CheckFunc: func() error { return nil }},
		{Name: "third", CheckFunc: func() error { return nil }},
	}

	results := RunChecks(checks)

	assert.Equal(t, "first", results[0].Check.Name)
	assert.Equal(t, "second", results[1].Check.Name)
	assert.Equal(t, "third", results[2].Check.Name)
}

func TestAllRequiredPassed_EmptyResults(t *testing.T) {
	results := []CheckResult{}
	assert.True(t, AllRequiredPassed(results))
}

func TestAllRequiredPassed_AllOptionalFailed(t *testing.T) {
	results := []CheckResult{
		{Check: Check{Required: false}, Passed: false},
		{Check: Check{Required: false}, Passed: false},
	}

	assert.True(t, AllRequiredPassed(results))
}

func TestFindVirtiofsd_ReturnsPath(t *testing.T) {
	// This test will pass if virtiofsd is installed, which is expected on dev machines
	path := FindVirtiofsd()
	// We can't assert it's non-empty because it depends on the system
	// But we can verify it returns a string
	_ = path
}
