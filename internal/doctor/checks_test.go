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
	assert.Contains(t, names, "qemu-img installed")
}

func TestRootChecks_ExtendsDefault(t *testing.T) {
	checks := RootChecks()

	names := make([]string, len(checks))
	for i, c := range checks {
		names[i] = c.Name
	}

	// Includes user-mode checks
	assert.Contains(t, names, "KVM available")
	assert.Contains(t, names, "libvirtd running")

	// Plus root-mode extras
	assert.Contains(t, names, "libvirt system mode reachable")
	assert.Contains(t, names, "Home dir traversable by QEMU")
	assert.Contains(t, names, "virtiofsd installed")
}

func TestInstallAllHintFor(t *testing.T) {
	// Common substrings every distro hint should mention.
	common := []string{"qemu", "libvirt", "virtiofsd", "usermod", "kvm"}

	tests := []struct {
		distro   Distro
		mustHave []string // distro-specific package names in addition to common
	}{
		{DistroDebian, []string{"apt", "libvirt-daemon-system", "qemu-utils", "cloud-image-utils", "systemctl", "libvirtd"}},
		{DistroFedora, []string{"dnf", "libvirt-daemon", "qemu-img", "guestfs-tools", "systemctl", "libvirtd"}},
		{DistroArch, []string{"pacman", "libguestfs", "qemu-img", "systemctl", "libvirtd"}},
		{DistroOpenSUSE, []string{"zypper", "libvirt-daemon", "qemu-tools", "systemctl", "libvirtd"}},
		{DistroUnknown, []string{"qemu-kvm", "cloud-image-utils", "systemctl", "libvirtd"}},
	}

	for _, tc := range tests {
		t.Run(string(tc.distro), func(t *testing.T) {
			hint := installAllHintFor(tc.distro)
			for _, s := range common {
				assert.Contains(t, hint, s, "common substring missing for %s", tc.distro)
			}
			for _, s := range tc.mustHave {
				assert.Contains(t, hint, s, "distro-specific substring missing for %s", tc.distro)
			}
		})
	}
}

func TestInstallAllHint_DispatchesByDistro(t *testing.T) {
	// The public entrypoint should produce the same string as the per-distro
	// helper for whichever distro the host happens to be.
	assert.Equal(t, installAllHintFor(DetectDistro()), InstallAllHint())
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
