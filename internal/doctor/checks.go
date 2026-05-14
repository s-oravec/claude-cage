package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// Distro represents the detected Linux distribution
type Distro string

const (
	DistroDebian   Distro = "debian" // Debian, Ubuntu, etc.
	DistroFedora   Distro = "fedora" // Fedora, RHEL, Rocky, Alma
	DistroArch     Distro = "arch"   // Arch Linux
	DistroOpenSUSE Distro = "opensuse"
	DistroUnknown  Distro = "unknown"
)

// DetectDistro detects the current Linux distribution
func DetectDistro() Distro {
	// Check /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return DistroUnknown
	}
	content := strings.ToLower(string(data))

	if strings.Contains(content, "ubuntu") || strings.Contains(content, "debian") ||
		strings.Contains(content, "mint") || strings.Contains(content, "pop!_os") {
		return DistroDebian
	}
	if strings.Contains(content, "fedora") || strings.Contains(content, "rhel") ||
		strings.Contains(content, "rocky") || strings.Contains(content, "alma") ||
		strings.Contains(content, "centos") {
		return DistroFedora
	}
	if strings.Contains(content, "arch") || strings.Contains(content, "manjaro") {
		return DistroArch
	}
	if strings.Contains(content, "opensuse") || strings.Contains(content, "suse") {
		return DistroOpenSUSE
	}
	return DistroUnknown
}

// Check represents a single system check
type Check struct {
	Name      string
	CheckFunc func() error
	Required  bool
	FixHint   string       // Installation/fix hint for the user (always populated)
	FixFunc   func() error // Optional: auto-applies the fix without sudo
}

// CheckResult holds the result of running a check
type CheckResult struct {
	Check  Check
	Passed bool
	Error  error
}

// FixResult holds the result of attempting an auto-fix
type FixResult struct {
	Check   Check
	Applied bool  // true if FixFunc ran successfully
	Error   error // FixFunc error, or nil if no FixFunc was available
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

// RunFixes invokes FixFunc on each failed check that has one. Returns a
// FixResult per fix attempted. Checks without a FixFunc are skipped silently.
func RunFixes(results []CheckResult) []FixResult {
	var fixes []FixResult
	for _, r := range results {
		if r.Passed || r.Check.FixFunc == nil {
			continue
		}
		err := r.Check.FixFunc()
		fixes = append(fixes, FixResult{
			Check:   r.Check,
			Applied: err == nil,
			Error:   err,
		})
	}
	return fixes
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

// DefaultChecks returns the prerequisite checks for user mode (regular user,
// libvirt session). This is the default surface that `cage doctor` validates.
func DefaultChecks() []Check {
	distro := DetectDistro()

	return []Check{
		{
			Name:      "KVM available",
			CheckFunc: checkKVM,
			Required:  true,
			FixHint:   fixHintKVM(distro),
		},
		{
			Name:      "libvirtd running",
			CheckFunc: checkLibvirtd,
			Required:  true,
			FixHint:   fixHintLibvirtd(distro),
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
			Name:      "qemu-img installed",
			CheckFunc: checkQemuImg,
			Required:  true,
			FixHint:   fixHintQemuImg(distro),
		},
		{
			Name:      "cloud-localds installed",
			CheckFunc: checkCloudLocalds,
			Required:  false, // Optional - genisoimage/mkisofs can be used instead
			FixHint:   fixHintCloudLocalds(distro),
		},
		{
			Name:      "virt-customize installed",
			CheckFunc: checkVirtCustomize,
			Required:  false, // Optional - needed for proper image save cleanup
			FixHint:   fixHintVirtCustomize(distro),
		},
	}
}

// RootChecks returns DefaultChecks plus the prerequisites for root mode
// (sudo): libvirt system mode connectivity, home-dir traversability by
// libvirt-qemu, and virtiofsd for shares.
func RootChecks() []Check {
	checks := DefaultChecks()
	distro := DetectDistro()

	qemuUser := QemuUser()
	displayQemuUser := qemuUser
	if displayQemuUser == "" {
		displayQemuUser = "libvirt-qemu"
	}
	home, _ := os.UserHomeDir()

	return append(checks,
		Check{
			Name:      "libvirt system mode reachable",
			CheckFunc: checkLibvirtSystemMode,
			Required:  true,
			FixHint:   "Run 'newgrp libvirt' (or log out/in) so libvirt group membership takes effect; ensure /var/run/libvirt/libvirt-sock exists",
		},
		Check{
			Name:      "Home dir traversable by QEMU",
			CheckFunc: checkHomeAccessibleToQemu,
			Required:  true,
			FixHint:   fmt.Sprintf("setfacl -m u:%s:x %s", displayQemuUser, home),
			FixFunc:   fixHomeACL,
		},
		Check{
			Name:      "virtiofsd installed",
			CheckFunc: checkVirtiofsd,
			Required:  true, // Required for shares in root mode
			FixHint:   fixHintVirtiofsd(distro),
		},
		Check{
			Name:      "QEMU supports -netdev stream (7.2+)",
			CheckFunc: checkQemuStreamNetdev,
			Required:  false, // Optional - cage falls back to SLIRP without host-side netns isolation
			FixHint:   "Update QEMU to 7.2+ for full host-side network isolation via passt+netns (Ubuntu 24.04+ / Debian 13+ / current Fedora); older QEMU still works with VM-side cloud-init blocking",
		},
	)
}

func fixHintKVM(d Distro) string {
	switch d {
	case DistroDebian:
		return "Enable virtualization in BIOS/UEFI, or install: sudo apt install qemu-kvm"
	case DistroFedora:
		return "Enable virtualization in BIOS/UEFI, or install: sudo dnf install qemu-kvm"
	case DistroArch:
		return "Enable virtualization in BIOS/UEFI, or install: sudo pacman -S qemu-base"
	case DistroOpenSUSE:
		return "Enable virtualization in BIOS/UEFI, or install: sudo zypper install qemu-kvm"
	default:
		return "Enable virtualization in BIOS/UEFI and install QEMU/KVM"
	}
}

func fixHintLibvirtd(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install libvirt-daemon-system && sudo systemctl enable --now libvirtd"
	case DistroFedora:
		return "sudo dnf install libvirt-daemon && sudo systemctl enable --now libvirtd"
	case DistroArch:
		return "sudo pacman -S libvirt && sudo systemctl enable --now libvirtd"
	case DistroOpenSUSE:
		return "sudo zypper install libvirt-daemon && sudo systemctl enable --now libvirtd"
	default:
		return "Install libvirt and enable libvirtd service"
	}
}

func fixHintVirtiofsd(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install virtiofsd"
	case DistroFedora:
		return "sudo dnf install virtiofsd"
	case DistroArch:
		return "sudo pacman -S virtiofsd"
	case DistroOpenSUSE:
		return "sudo zypper install virtiofsd"
	default:
		return "Install virtiofsd (part of QEMU)"
	}
}

func fixHintQemuImg(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install qemu-utils"
	case DistroFedora:
		return "sudo dnf install qemu-img"
	case DistroArch:
		return "sudo pacman -S qemu-img"
	case DistroOpenSUSE:
		return "sudo zypper install qemu-tools"
	default:
		return "Install qemu-img (part of QEMU)"
	}
}

func fixHintCloudLocalds(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install cloud-image-utils"
	case DistroFedora:
		return "sudo dnf install cloud-utils"
	case DistroArch:
		return "yay -S cloud-image-utils (from AUR)"
	case DistroOpenSUSE:
		return "sudo zypper install cloud-utils"
	default:
		return "Install cloud-localds (part of cloud-image-utils)"
	}
}

func fixHintVirtCustomize(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install libguestfs-tools"
	case DistroFedora:
		return "sudo dnf install guestfs-tools"
	case DistroArch:
		return "sudo pacman -S libguestfs"
	case DistroOpenSUSE:
		return "sudo zypper install guestfs-tools"
	default:
		return "Install libguestfs-tools for proper image save cleanup"
	}
}

// InstallAllHint returns a single command to install all required packages
// for the host's detected distribution.
func InstallAllHint() string {
	return installAllHintFor(DetectDistro())
}

// installAllHintFor returns the install-everything hint for a specific distro.
// Split out from InstallAllHint so tests can exercise every branch without
// depending on the host OS.
func installAllHintFor(d Distro) string {
	switch d {
	case DistroDebian:
		return "sudo apt install -y qemu-kvm libvirt-daemon-system libvirt-clients virtiofsd qemu-utils cloud-image-utils libguestfs-tools && sudo usermod -aG kvm,libvirt $USER && sudo systemctl enable --now libvirtd"
	case DistroFedora:
		return "sudo dnf install -y qemu-kvm libvirt-daemon libvirt-client virtiofsd qemu-img cloud-utils guestfs-tools && sudo usermod -aG kvm,libvirt $USER && sudo systemctl enable --now libvirtd"
	case DistroArch:
		return "sudo pacman -S qemu-base libvirt virtiofsd qemu-img libguestfs && sudo usermod -aG kvm,libvirt $USER && sudo systemctl enable --now libvirtd"
	case DistroOpenSUSE:
		return "sudo zypper install qemu-kvm libvirt-daemon virtiofsd qemu-tools cloud-utils guestfs-tools && sudo usermod -aG kvm,libvirt $USER && sudo systemctl enable --now libvirtd"
	default:
		return "Install: qemu-kvm, libvirt, virtiofsd, qemu-img, cloud-image-utils, libguestfs-tools\nAdd user to groups: sudo usermod -aG kvm,libvirt $USER\nEnable libvirtd: sudo systemctl enable --now libvirtd"
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

func checkLibvirtSystemMode() error {
	cmd := exec.Command("virsh", "-c", "qemu:///system", "version")
	if err := cmd.Run(); err != nil {
		return errors.New("cannot connect to qemu:///system")
	}
	return nil
}

// QemuUser returns the system user QEMU runs as under libvirt system mode.
// Returns "libvirt-qemu" (Debian/Ubuntu) or "qemu" (Fedora/Arch/openSUSE)
// whichever is present in /etc/passwd, or empty string if neither exists.
func QemuUser() string {
	for _, name := range []string{"libvirt-qemu", "qemu"} {
		if _, err := user.Lookup(name); err == nil {
			return name
		}
	}
	return ""
}

func checkHomeAccessibleToQemu() error {
	qemuUser := QemuUser()
	if qemuUser == "" {
		return errors.New("could not detect qemu user (libvirt-qemu or qemu)")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	info, err := os.Stat(home)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", home, err)
	}
	// Others has execute (anyone can traverse)
	if info.Mode().Perm()&0o001 != 0 {
		return nil
	}

	if _, err := exec.LookPath("getfacl"); err != nil {
		return fmt.Errorf("%s not traversable by %s (install 'acl' to auto-fix, or chmod o+x %s)", home, qemuUser, home)
	}

	out, err := exec.Command("getfacl", "--absolute-names", "-p", home).Output()
	if err != nil {
		return fmt.Errorf("getfacl failed on %s: %w", home, err)
	}

	prefix := "user:" + qemuUser + ":"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, prefix) && strings.Contains(line[len(prefix):], "x") {
			return nil
		}
	}
	return fmt.Errorf("%s not traversable by %s (qemu cannot reach files under it)", home, qemuUser)
}

func fixHomeACL() error {
	qemuUser := QemuUser()
	if qemuUser == "" {
		return errors.New("could not detect qemu user (libvirt-qemu or qemu)")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("setfacl"); err != nil {
		return errors.New("setfacl not found; install the 'acl' package first")
	}
	out, err := exec.Command("setfacl", "-m", fmt.Sprintf("u:%s:x", qemuUser), home).CombinedOutput()
	if err != nil {
		return fmt.Errorf("setfacl failed: %s", strings.TrimSpace(string(out)))
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
	"virtiofsd",               // In PATH
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

func checkVirtCustomize() error {
	_, err := exec.LookPath("virt-customize")
	if err != nil {
		return errors.New("virt-customize not found (saved images may need manual cleanup)")
	}
	return nil
}

func checkQemuStreamNetdev() error {
	out, err := exec.Command("qemu-system-x86_64", "-netdev", "help").CombinedOutput()
	if err != nil {
		return errors.New("could not query QEMU netdev support")
	}
	if !strings.Contains(string(out), "stream") {
		return errors.New("QEMU lacks -netdev stream (need 7.2+); host-side netns isolation will fall back to SLIRP")
	}
	return nil
}
