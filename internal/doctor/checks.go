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
	FixHint   string // Installation/fix hint for the user
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
		{
			Name:      "KVM available",
			CheckFunc: checkKVM,
			Required:  true,
			FixHint:   "Enable virtualization in BIOS/UEFI, or install: sudo apt install qemu-kvm",
		},
		{
			Name:      "libvirtd running",
			CheckFunc: checkLibvirtd,
			Required:  true,
			FixHint:   "sudo apt install libvirt-daemon-system && sudo systemctl enable --now libvirtd",
		},
		{
			Name:      "User in kvm group",
			CheckFunc: checkKvmGroup,
			Required:  true,
			FixHint:   "sudo usermod -aG kvm $USER && newgrp kvm",
		},
		{
			Name:      "User in libvirt group",
			CheckFunc: checkLibvirtGroup,
			Required:  true,
			FixHint:   "sudo usermod -aG libvirt $USER && newgrp libvirt",
		},
		{
			Name:      "virtiofsd installed",
			CheckFunc: checkVirtiofsd,
			Required:  true,
			FixHint:   "sudo apt install virtiofsd",
		},
		{
			Name:      "qemu-img installed",
			CheckFunc: checkQemuImg,
			Required:  true,
			FixHint:   "sudo apt install qemu-utils",
		},
		{
			Name:      "cloud-localds installed",
			CheckFunc: checkCloudLocalds,
			Required:  false,
			FixHint:   "sudo apt install cloud-image-utils",
		},
	}
}

// InstallAllHint returns a single command to install all required packages
func InstallAllHint() string {
	return "sudo apt install -y qemu-kvm libvirt-daemon-system libvirt-clients virtiofsd qemu-utils cloud-image-utils && sudo usermod -aG kvm,libvirt $USER && sudo systemctl enable --now libvirtd"
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

// Common locations for virtiofsd
var virtiofsdPaths = []string{
	"virtiofsd",              // In PATH
	"/usr/lib/qemu/virtiofsd", // Ubuntu/Debian
	"/usr/libexec/virtiofsd",  // Fedora/RHEL
}

// FindVirtiofsd returns the path to virtiofsd or empty string if not found
func FindVirtiofsd() string {
	for _, path := range virtiofsdPaths {
		if path == "virtiofsd" {
			if p, err := exec.LookPath(path); err == nil {
				return p
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func checkVirtiofsd() error {
	if FindVirtiofsd() == "" {
		return errors.New("virtiofsd not found (checked PATH, /usr/lib/qemu/, /usr/libexec/)")
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
