package doctor

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// Check represents a single system check
type Check struct {
	Name      string
	CheckFunc func() error
	Required  bool
}

// CheckResult holds the result of running a check
type CheckResult struct {
	Check  Check
	Passed bool
	Error  error
}

// RunChecks executes all checks and returns results
func RunChecks(checks []Check) []CheckResult {
	results := make([]CheckResult, len(checks))
	for i, check := range checks {
		err := check.CheckFunc()
		results[i] = CheckResult{
			Check:  check,
			Passed: err == nil,
			Error:  err,
		}
	}
	return results
}

// AllRequiredPassed returns true if all required checks passed
func AllRequiredPassed(results []CheckResult) bool {
	for _, r := range results {
		if r.Check.Required && !r.Passed {
			return false
		}
	}
	return true
}

// DefaultChecks returns the standard set of checks
func DefaultChecks() []Check {
	return []Check{
		{Name: "KVM available", CheckFunc: checkKVM, Required: true},
		{Name: "libvirtd running", CheckFunc: checkLibvirtd, Required: true},
		{Name: "User in kvm group", CheckFunc: checkKvmGroup, Required: true},
		{Name: "User in libvirt group", CheckFunc: checkLibvirtGroup, Required: true},
		{Name: "virtiofsd installed", CheckFunc: checkVirtiofsd, Required: true},
		{Name: "qemu-img installed", CheckFunc: checkQemuImg, Required: true},
		{Name: "cloud-localds installed", CheckFunc: checkCloudLocalds, Required: false},
	}
}

func checkKVM() error {
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		return errors.New("/dev/kvm not found")
	}
	return nil
}

func checkLibvirtd() error {
	cmd := exec.Command("systemctl", "is-active", "libvirtd")
	output, err := cmd.Output()
	if err != nil {
		return errors.New("libvirtd not running")
	}
	if strings.TrimSpace(string(output)) != "active" {
		return errors.New("libvirtd not active")
	}
	return nil
}

func checkKvmGroup() error {
	return checkUserInGroup("kvm")
}

func checkLibvirtGroup() error {
	return checkUserInGroup("libvirt")
}

func checkUserInGroup(groupName string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	groups, err := u.GroupIds()
	if err != nil {
		return err
	}
	group, err := user.LookupGroup(groupName)
	if err != nil {
		return err
	}
	for _, gid := range groups {
		if gid == group.Gid {
			return nil
		}
	}
	return errors.New("user not in " + groupName + " group")
}

func checkVirtiofsd() error {
	_, err := exec.LookPath("virtiofsd")
	if err != nil {
		return errors.New("virtiofsd not found in PATH")
	}
	return nil
}

func checkQemuImg() error {
	_, err := exec.LookPath("qemu-img")
	if err != nil {
		return errors.New("qemu-img not found in PATH")
	}
	return nil
}

func checkCloudLocalds() error {
	_, err := exec.LookPath("cloud-localds")
	if err != nil {
		return errors.New("cloud-localds not found in PATH")
	}
	return nil
}
