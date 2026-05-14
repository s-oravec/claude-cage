package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CloudInitConfig holds configuration for cloud-init generation
type CloudInitConfig struct {
	CageName         string
	PubKey           string
	MountVirtiofs    bool
	Env              map[string]string
	InstallSSH       bool
	UseRuntimeEnv    bool     // Source env from /cage/runtime/env.sh instead of baking in Env
	NetworkIsolation bool     // Add routes to block access to private IP ranges
	AllowedSubnets   []string // Subnets to allow (e.g., SLIRP network 10.0.2.0/24)
}

// GenerateUserData generates cloud-init user-data content
func GenerateUserData(cageName, pubKey string) string {
	return GenerateUserDataWithConfig(&CloudInitConfig{
		CageName:      cageName,
		PubKey:        pubKey,
		MountVirtiofs: false,
	})
}

// generateNetworkIsolationRuncmd generates runcmd lines for network isolation
func generateNetworkIsolationRuncmd(allowedSubnets []string) string {
	// Default blocked subnets (RFC 1918 + link-local)
	blockedSubnets := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
	}

	var lines []string
	lines = append(lines, "  # Network isolation: block access to private IP ranges")
	lines = append(lines, "  # This prevents the cage from accessing the host LAN")

	// Add allowed subnets first (more specific routes take precedence)
	for _, subnet := range allowedSubnets {
		lines = append(lines, fmt.Sprintf("  # Allow %s (required for networking)", subnet))
	}

	// Add unreachable routes for blocked subnets
	// The more specific allowed routes will take precedence
	for _, subnet := range blockedSubnets {
		lines = append(lines, fmt.Sprintf("  - ip route add unreachable %s 2>/dev/null || true", subnet))
	}

	// Make the routes persistent across reboots
	lines = append(lines, "  # Persist network isolation routes")
	lines = append(lines, "  - |")
	lines = append(lines, "    cat > /etc/network/if-up.d/cage-isolation 2>/dev/null << 'EOF' || true")
	lines = append(lines, "    #!/bin/sh")
	for _, subnet := range blockedSubnets {
		lines = append(lines, fmt.Sprintf("    ip route add unreachable %s 2>/dev/null || true", subnet))
	}
	lines = append(lines, "    EOF")
	lines = append(lines, "  - chmod +x /etc/network/if-up.d/cage-isolation 2>/dev/null || true")
	// For systemd-networkd based systems
	lines = append(lines, "  - mkdir -p /etc/networkd-dispatcher/routable.d 2>/dev/null || true")
	lines = append(lines, "  - |")
	lines = append(lines, "    cat > /etc/networkd-dispatcher/routable.d/cage-isolation 2>/dev/null << 'EOF' || true")
	lines = append(lines, "    #!/bin/sh")
	for _, subnet := range blockedSubnets {
		lines = append(lines, fmt.Sprintf("    ip route add unreachable %s 2>/dev/null || true", subnet))
	}
	lines = append(lines, "    EOF")
	lines = append(lines, "  - chmod +x /etc/networkd-dispatcher/routable.d/cage-isolation 2>/dev/null || true")

	return "\n" + strings.Join(lines, "\n")
}

// generateEnvRuncmd generates runcmd lines for environment variables
func generateEnvRuncmd(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	lines = append(lines, "  # Setup environment variables")
	lines = append(lines, "  - mkdir -p /etc/profile.d")

	// Build the content for cage-env.sh
	var envLines []string
	envLines = append(envLines, "# Cage environment variables")
	for _, k := range keys {
		v := env[k]
		// Escape single quotes in value by replacing ' with '\''
		escaped := strings.ReplaceAll(v, "'", "'\\''")
		envLines = append(envLines, fmt.Sprintf("export %s='%s'", k, escaped))
	}

	// Write all env vars to /etc/profile.d/cage-env.sh using cat heredoc
	content := strings.Join(envLines, "\n")
	lines = append(lines, fmt.Sprintf("  - |\n    cat > /etc/profile.d/cage-env.sh << 'CAGEENV'\n    %s\n    CAGEENV", strings.ReplaceAll(content, "\n", "\n    ")))
	lines = append(lines, "  - chmod 644 /etc/profile.d/cage-env.sh")

	return "\n" + strings.Join(lines, "\n")
}

// GenerateUserDataWithConfig generates cloud-init user-data with full config
func GenerateUserDataWithConfig(cfg *CloudInitConfig) string {
	var mounts []string
	virtiofsRuncmd := ""

	if cfg.MountVirtiofs {
		mounts = append(mounts, `  - [ workspace, /workspace, virtiofs, "defaults,nofail", "0", "0" ]`)
		virtiofsRuncmd = `
  - mkdir -p /workspace
  - mount -t virtiofs workspace /workspace || true
  - chown cage:cage /workspace || true`
	}

	if cfg.UseRuntimeEnv {
		mounts = append(mounts, `  - [ cage-runtime, /cage/runtime, virtiofs, "ro,nofail", "0", "0" ]`)
	}

	virtiofsMounts := ""
	if len(mounts) > 0 {
		virtiofsMounts = "\nmounts:\n" + strings.Join(mounts, "\n") + "\n"
	}

	writeFiles := ""
	if cfg.UseRuntimeEnv {
		writeFiles = `
write_files:
  - path: /etc/profile.d/cage-runtime-env.sh
    permissions: '0644'
    content: |
      # Source cage runtime environment
      if [ -f /cage/runtime/env.sh ]; then
          . /cage/runtime/env.sh
      fi
`
	}

	sshRuncmd := ""
	if cfg.InstallSSH {
		sshRuncmd = `
  # Install and enable SSH server
  - which apk && apk add --no-cache openssh && rc-update add sshd default && rc-service sshd start || true
  - which apt-get && apt-get update && apt-get install -y openssh-server && systemctl enable ssh && systemctl start ssh || true
  - which dnf && dnf install -y openssh-server && systemctl enable sshd && systemctl start sshd || true
  - which zypper && zypper install -y openssh && systemctl enable sshd && systemctl start sshd || true`
	}

	// When UseRuntimeEnv is true, don't use the old Env field (mutually exclusive)
	envRuncmd := ""
	if !cfg.UseRuntimeEnv {
		envRuncmd = generateEnvRuncmd(cfg.Env)
	}

	// Network isolation runcmd
	networkIsolationRuncmd := ""
	if cfg.NetworkIsolation {
		networkIsolationRuncmd = generateNetworkIsolationRuncmd(cfg.AllowedSubnets)
	}

	return fmt.Sprintf(`#cloud-config
users:
  - name: cage
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/sh
    groups: wheel,docker
    lock_passwd: false
    # password: cage
    passwd: $6$jPpNVOdPlZdiMeW.$tGs/Xy0/9wH7CtN9pMaGFnDFmK0THolDE5SALY.rIcwezfG7WU0syq7xov9ZFy.8GI5K03j/LcvK2vr3pf2pp1
    ssh_authorized_keys:
      - %[1]s

ssh_pwauth: false

# Grow root partition and filesystem to use full disk
growpart:
  mode: auto
  devices: ['/']
  ignore_growroot_disabled: false

resize_rootfs: true

package_update: false
package_upgrade: false
%[2]s%[3]s
runcmd:
  # Ensure SSH key is set (critical for custom images where user already exists)
  # This ensures the new key is always written, even if cloud-init user module skips existing users
  - mkdir -p /home/cage/.ssh
  - chmod 700 /home/cage/.ssh
  - echo '%[1]s' > /home/cage/.ssh/authorized_keys
  - chmod 600 /home/cage/.ssh/authorized_keys
  - chown -R cage:cage /home/cage/.ssh
  # Install sudo on Alpine (apk) or ensure it exists on other distros
  - which apk && apk add --no-cache sudo doas || true
  # Configure doas for Alpine (wheel group)
  - echo "permit nopass :wheel" > /etc/doas.d/wheel.conf 2>/dev/null || true
  # Ensure sudoers is configured (for distros with sudo)
  - echo "cage ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/cage 2>/dev/null || true
  - chmod 440 /etc/sudoers.d/cage 2>/dev/null || true
  # Locales — generate en_US.UTF-8 so SSH-forwarded LC_*/LANG don't trigger
  # perl/dpkg/locale "Setting locale failed" warnings on every command.
  # C.UTF-8 is the safe fallback (always present on Debian/Ubuntu); en_US.UTF-8
  # covers the most common SSH SendEnv defaults.
  - which apt-get && DEBIAN_FRONTEND=noninteractive apt-get install -y locales >/dev/null 2>&1 || true
  - which locale-gen && locale-gen en_US.UTF-8 >/dev/null 2>&1 || true
  - which update-locale && update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 >/dev/null 2>&1 || true
  - which apk && apk add --no-cache musl-locales musl-locales-lang >/dev/null 2>&1 || true
  # Docker setup (systemd-based distros)
  - systemctl enable docker || true
  - systemctl start docker || true
  # Docker setup (OpenRC-based distros like Alpine)
  - which rc-update && rc-update add docker default || true
  - which rc-service && rc-service docker start || true%[4]s%[5]s%[6]s%[7]s
`, cfg.PubKey, virtiofsMounts, writeFiles, virtiofsRuncmd, sshRuncmd, envRuncmd, networkIsolationRuncmd)
}

// GenerateMetaData generates cloud-init meta-data content
// Uses a unique instance-id with timestamp to ensure cloud-init re-runs on each cage creation
func GenerateMetaData(cageName string) string {
	// Add timestamp to instance-id to force cloud-init to re-run
	// This is critical for custom images where cloud-init data from the original cage exists
	instanceID := fmt.Sprintf("%s-%d", cageName, time.Now().UnixNano())
	return fmt.Sprintf(`instance-id: %s
local-hostname: %s
`, instanceID, cageName)
}

// WriteCloudInitFiles writes user-data and meta-data to a directory
func WriteCloudInitFiles(dir, cageName, pubKey string) error {
	return WriteCloudInitFilesWithConfig(dir, &CloudInitConfig{
		CageName: cageName,
		PubKey:   pubKey,
	})
}

// WriteCloudInitFilesWithConfig writes cloud-init files with full config
func WriteCloudInitFilesWithConfig(dir string, cfg *CloudInitConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	userData := GenerateUserDataWithConfig(cfg)
	if err := os.WriteFile(filepath.Join(dir, "user-data"), []byte(userData), 0644); err != nil {
		return err
	}

	metaData := GenerateMetaData(cfg.CageName)
	if err := os.WriteFile(filepath.Join(dir, "meta-data"), []byte(metaData), 0644); err != nil {
		return err
	}

	return nil
}

// GenerateISO creates a cloud-init ISO from user-data and meta-data
func GenerateISO(cageDir, cageName, pubKey string) (string, error) {
	return GenerateISOWithConfig(cageDir, &CloudInitConfig{
		CageName: cageName,
		PubKey:   pubKey,
	})
}

// GenerateISOWithConfig creates a cloud-init ISO with full config
func GenerateISOWithConfig(cageDir string, cfg *CloudInitConfig) (string, error) {
	// Create cloud-init files
	cloudInitDir := filepath.Join(cageDir, "cloudinit")
	if err := WriteCloudInitFilesWithConfig(cloudInitDir, cfg); err != nil {
		return "", fmt.Errorf("failed to write cloud-init files: %w", err)
	}

	isoPath := filepath.Join(cageDir, "cloud-init.iso")
	userDataPath := filepath.Join(cloudInitDir, "user-data")
	metaDataPath := filepath.Join(cloudInitDir, "meta-data")

	// Try cloud-localds first (preferred)
	if _, err := exec.LookPath("cloud-localds"); err == nil {
		cmd := exec.Command("cloud-localds", isoPath, userDataPath, metaDataPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("cloud-localds failed: %s", string(out))
		}
		return isoPath, nil
	}

	// Fallback to genisoimage
	if _, err := exec.LookPath("genisoimage"); err == nil {
		cmd := exec.Command("genisoimage",
			"-output", isoPath,
			"-volid", "cidata",
			"-joliet",
			"-rock",
			cloudInitDir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("genisoimage failed: %s", string(out))
		}
		return isoPath, nil
	}

	// Fallback to mkisofs
	if _, err := exec.LookPath("mkisofs"); err == nil {
		cmd := exec.Command("mkisofs",
			"-output", isoPath,
			"-volid", "cidata",
			"-joliet",
			"-rock",
			cloudInitDir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("mkisofs failed: %s", string(out))
		}
		return isoPath, nil
	}

	return "", fmt.Errorf("no ISO creation tool found (install cloud-localds, genisoimage, or mkisofs)")
}
