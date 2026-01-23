package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CloudInitConfig holds configuration for cloud-init generation
type CloudInitConfig struct {
	CageName     string
	PubKey       string
	MountVirtiofs bool
}

// GenerateUserData generates cloud-init user-data content
func GenerateUserData(cageName, pubKey string) string {
	return GenerateUserDataWithConfig(&CloudInitConfig{
		CageName:     cageName,
		PubKey:       pubKey,
		MountVirtiofs: false,
	})
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

	return fmt.Sprintf(`#cloud-config
users:
  - name: cage
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    groups: docker
    lock_passwd: false
    # password: cage
    passwd: $6$jPpNVOdPlZdiMeW.$tGs/Xy0/9wH7CtN9pMaGFnDFmK0THolDE5SALY.rIcwezfG7WU0syq7xov9ZFy.8GI5K03j/LcvK2vr3pf2pp1
    ssh_authorized_keys:
      - %s

ssh_pwauth: true

package_update: false
package_upgrade: false
%s
runcmd:
  - systemctl enable docker || true
  - systemctl start docker || true%s
`, cfg.PubKey, virtiofsMounts, virtiofsRuncmd)
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
