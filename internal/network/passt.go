package network

import (
	"os/exec"
)

// HasPasst checks if passt is available on the system
func HasPasst() bool {
	_, err := exec.LookPath("passt")
	return err == nil
}
