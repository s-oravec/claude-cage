package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateUserData generates cloud-init user-data content
func GenerateUserData(cageName, pubKey string) string {
	return fmt.Sprintf(`#cloud-config
users:
  - name: cage
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    groups: docker
    ssh_authorized_keys:
      - %s

ssh_pwauth: false

package_update: false
package_upgrade: false

runcmd:
  - systemctl enable docker || true
  - systemctl start docker || true
`, pubKey)
}

// GenerateMetaData generates cloud-init meta-data content
func GenerateMetaData(cageName string) string {
	return fmt.Sprintf(`instance-id: %s
local-hostname: %s
`, cageName, cageName)
}

// WriteCloudInitFiles writes user-data and meta-data to a directory
func WriteCloudInitFiles(dir, cageName, pubKey string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	userData := GenerateUserData(cageName, pubKey)
	if err := os.WriteFile(filepath.Join(dir, "user-data"), []byte(userData), 0644); err != nil {
		return err
	}

	metaData := GenerateMetaData(cageName)
	if err := os.WriteFile(filepath.Join(dir, "meta-data"), []byte(metaData), 0644); err != nil {
		return err
	}

	return nil
}

// GenerateISO creates a cloud-init ISO from user-data and meta-data
func GenerateISO(cageDir, cageName, pubKey string) (string, error) {
	// Create cloud-init files
	cloudInitDir := filepath.Join(cageDir, "cloudinit")
	if err := WriteCloudInitFiles(cloudInitDir, cageName, pubKey); err != nil {
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
