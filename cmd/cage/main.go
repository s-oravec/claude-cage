package main

import (
	"os"

	"github.com/s-oravec/claude-cage/internal/cmd"
	"github.com/s-oravec/claude-cage/internal/tty"
)

func main() {
	// Snapshot the controlling tty so any termios damage done by a
	// subprocess (ssh, dpkg via ssh, virsh, …) is undone when cage exits.
	ttyState := tty.Save()
	defer ttyState.Restore()

	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		ttyState.Restore()
		os.Exit(1)
	}
}
