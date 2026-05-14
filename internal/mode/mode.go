// Package mode distinguishes cage's two operating modes:
//
//   - User mode: runs as a regular user, uses libvirt session mode. No
//     virtiofs shares, no injected env, no bridge networking. SLIRP /
//     user-mode network and SSH only. Zero host configuration required.
//
//   - Root mode: runs as root (`sudo cage`), uses libvirt system mode.
//     Supports virtiofs shares, env injection, bridge networking, and
//     host-level network isolation. Uses a system-wide state path under
//     /var/lib/libvirt/images/cage/ to stay compatible with the default
//     libvirt-qemu apparmor profile.
package mode

import (
	"os"

	"github.com/s-oravec/claude-cage/internal/config"
)

// Mode is cage's operating mode.
type Mode int

const (
	User Mode = iota
	Root
)

func (m Mode) String() string {
	switch m {
	case Root:
		return "root"
	default:
		return "user"
	}
}

// URI returns the libvirt connection URI for the mode.
func (m Mode) URI() string {
	if m == Root {
		return "qemu:///system"
	}
	return "qemu:///session"
}

// Current returns the mode of the running process, inferred from the
// effective UID. UID 0 → Root, otherwise User.
func Current() Mode {
	if os.Geteuid() == 0 {
		return Root
	}
	return User
}

// RequiredFromConfig returns the mode required to run a cage with the given
// resolved config. User mode is sufficient when the config has no shares,
// no injected env, and no bridge networking.
func RequiredFromConfig(cfg *config.ResolvedConfig) Mode {
	if cfg == nil {
		return User
	}
	if len(cfg.Shares) > 0 {
		return Root
	}
	if len(cfg.Env) > 0 {
		return Root
	}
	// Bridge networking would also force Root; detect via NetworkConfig
	// fields once bridge mode is selectable from config. For now, project
	// configs always use the auto/SLIRP path.
	return User
}
