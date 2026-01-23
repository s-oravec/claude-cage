package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CloudInitConfig holds configuration for cloud-init generation
type CloudInitConfig struct {
	CageName      string
	PubKey        string
	MountVirtiofs bool
	Env           map[string]string
	InstallSSH    bool
}

// GenerateUserData generates cloud-init user-data content
func GenerateUserData(cageName, pubKey string) string {
	return GenerateUserDataWithConfig(&CloudInitConfig{
		CageName:     cageName,
		PubKey:       pubKey,
		MountVirtiofs: false,
	})
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
	virtiofsMounts := ""
	virtiofsRuncmd := ""

	if cfg.MountVirtiofs {
		virtiofsMounts = `
mounts:
  - [ workspace, /workspace, virtiofs, "defaults,nofail", "0", "0" ]
`
		virtiofsRuncmd = `
  - mkdir -p /workspace
  - mount -t virtiofs workspace /workspace || true
  - chown cage:cage /workspace || true`
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

	envRuncmd := generateEnvRuncmd(cfg.Env)

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
      - %s

ssh_pwauth: false

# Grow root partition and filesystem to use full disk
growpart:
  mode: auto
  devices: ['/']
  ignore_growroot_disabled: false

resize_rootfs: true

package_update: false
package_upgrade: false
%s
runcmd:
  # Install sudo on Alpine (apk) or ensure it exists on other distros
  - which apk && apk add --no-cache sudo doas || true
  # Configure doas for Alpine (wheel group)
  - echo "permit nopass :wheel" > /etc/doas.d/wheel.conf 2>/dev/null || true
  # Ensure sudoers is configured (for distros with sudo)
  - echo "cage ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/cage 2>/dev/null || true
  - chmod 440 /etc/sudoers.d/cage 2>/dev/null || true
  # Docker setup (systemd-based distros)
  - systemctl enable docker || true
  - systemctl start docker || true
  # Docker setup (OpenRC-based distros like Alpine)
  - which rc-update && rc-update add docker default || true
  - which rc-service && rc-service docker start || true%s%s%s
`, cfg.PubKey, virtiofsMounts, virtiofsRuncmd, sshRuncmd, envRuncmd)
}

// GenerateMetaData generates cloud-init meta-data content
func GenerateMetaData(cageName string) string {
	return fmt.Sprintf(`instance-id: %s
local-hostname: %s
`, cageName, cageName)
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
