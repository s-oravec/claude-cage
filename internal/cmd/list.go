package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/spf13/cobra"
)

// NewListCmd creates the list command
func NewListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List running cages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listCages(cmd, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func listCages(cmd *cobra.Command, jsonOutput bool) error {
	cages, err := cage.List()
	if err != nil {
		return err
	}

	if jsonOutput {
		data, err := json.MarshalIndent(cages, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	if len(cages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cages running")
		return nil
	}

	// Table header
	fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-10s %-18s %-10s %-20s %s\n",
		"NAME", "STATUS", "IMAGE", "PROFILE", "SSH", "UPTIME")

	for _, c := range cages {
		uptime := formatUptime(c.StartedAt)
		sshInfo := formatSSH(c)
		fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-10s %-18s %-10s %-20s %s\n",
			c.Name, c.Status, c.Image, c.Profile, sshInfo, uptime)
	}

	return nil
}

func formatSSH(c *cage.State) string {
	if c.SSHPort > 0 {
		return fmt.Sprintf("localhost:%d", c.SSHPort)
	}
	if c.IP != "" {
		return c.IP
	}
	return "-"
}

func formatUptime(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "-"
	}

	d := time.Since(startedAt)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}
