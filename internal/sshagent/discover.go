// Package sshagent finds the invoking user's ssh-agent socket when cage is
// running under sudo. With env_reset in sudoers, SSH_AUTH_SOCK is stripped
// before cage runs — so `cage build -A` would have nothing to forward.
// This package re-discovers the socket via SUDO_USER and the standard
// runtime-dir layout, so `sudo cage build -A` works without any
// per-host sudoers tweak.
package sshagent

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// candidateSockets lists the standard places ssh-agent sockets live on a
// modern Linux desktop, in priority order:
//
//   - /run/user/<uid>/keyring/ssh        gnome-keyring (default on
//     Ubuntu/Debian/Fedora GNOME)
//   - /run/user/<uid>/ssh-agent.socket   systemd user unit for ssh-agent
//   - /run/user/<uid>/openssh_agent      some openssh installations
//
// Returns the first path that exists AND is actually a socket (not a
// regular file an attacker could have planted).
func candidateSockets(uid int) []string {
	runUser := filepath.Join("/run/user", strconv.Itoa(uid))
	return []string{
		filepath.Join(runUser, "keyring", "ssh"),
		filepath.Join(runUser, "ssh-agent.socket"),
		filepath.Join(runUser, "openssh_agent"),
	}
}

// Discover returns a path to a usable ssh-agent socket, or "" if none.
//
// Priority:
//  1. If SSH_AUTH_SOCK is already set in the environment (user passed
//     --preserve-env or has env_keep configured), use that.
//  2. If running as root with SUDO_USER set, probe standard runtime-dir
//     paths for that user and return the first socket found.
//  3. Otherwise "" — SSH will fail with its normal "Could not open a
//     connection to your authentication agent" error.
func Discover() string {
	if s := os.Getenv("SSH_AUTH_SOCK"); s != "" {
		return s
	}
	if os.Geteuid() != 0 {
		// Non-root and no env var — nothing to discover.
		return ""
	}
	name := os.Getenv("SUDO_USER")
	if name == "" {
		return ""
	}
	usr, err := user.Lookup(name)
	if err != nil {
		return ""
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return ""
	}
	for _, path := range candidateSockets(uid) {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().Type()&os.ModeSocket == 0 {
			// Defensive: only accept paths that are actually sockets, so
			// a regular file planted at /run/user/<uid>/keyring/ssh can't
			// be silently used (an attacker who can write there has the
			// user account anyway, but no reason to be loose).
			continue
		}
		return path
	}
	return ""
}
