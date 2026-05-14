// Package tty saves and restores the invoking shell's terminal modes
// around cage's lifetime. Subprocesses cage spawns (ssh, virsh, dpkg
// inside the cage via ssh, …) sometimes leave the controlling tty in
// a degraded state when they exit abnormally — most visibly disabling
// ONLCR so subsequent newlines no longer return the cursor to column
// zero. Snapshotting and restoring termios guarantees the user's shell
// is left exactly as it started, regardless of what children did.
package tty

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// State is an opaque, restorable snapshot of /dev/tty's termios.
type State struct {
	encoded string
}

// Save snapshots /dev/tty's current modes. Returns nil if no controlling
// terminal is available (e.g. running under a CI runner with stdin
// redirected) or if `stty` is missing — in those cases there's nothing
// to restore.
func Save() *State {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	defer f.Close()

	if _, err := exec.LookPath("stty"); err != nil {
		return nil
	}

	cmd := exec.Command("stty", "-g")
	cmd.Stdin = f
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	return &State{encoded: strings.TrimSpace(out.String())}
}

// Restore re-applies a previously Save()d termios. Safe on a nil receiver
// (when Save() returned nil because there was no tty).
func (s *State) Restore() {
	if s == nil || s.encoded == "" {
		return
	}
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer f.Close()

	cmd := exec.Command("stty", s.encoded)
	cmd.Stdin = f
	_ = cmd.Run()
}
