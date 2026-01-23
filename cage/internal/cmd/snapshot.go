package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/snapshot"
)

// NewSnapshotCmd creates the snapshot command with subcommands
func NewSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage cage snapshots",
		Long: `Manage snapshots for cage VMs.

Snapshots allow you to save the state of a cage and restore it later.
This is useful for testing, experimentation, or recovery.`,
	}

	cmd.AddCommand(newSnapshotCreateCmd())
	cmd.AddCommand(newSnapshotListCmd())
	cmd.AddCommand(newSnapshotRestoreCmd())
	cmd.AddCommand(newSnapshotDeleteCmd())

	return cmd
}

func newSnapshotCreateCmd() *cobra.Command {
	var name string
	var description string

	cmd := &cobra.Command{
		Use:   "create <cage-name>",
		Short: "Create a snapshot of a cage",
		Long: `Create a snapshot of a cage VM.

The snapshot captures the current state of the VM's disk.
You can restore to this snapshot later using 'cage snapshot restore'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createSnapshot(cmd, args[0], name, description)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Snapshot name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Snapshot description")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newSnapshotListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <cage-name>",
		Short: "List snapshots of a cage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSnapshots(cmd, args[0])
		},
	}
}

func newSnapshotRestoreCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "restore <cage-name>",
		Short: "Restore a cage to a snapshot",
		Long: `Restore a cage VM to a previous snapshot.

Warning: This will discard any changes made since the snapshot was created.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return restoreSnapshot(cmd, args[0], name)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Snapshot name to restore (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newSnapshotDeleteCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete <cage-name>",
		Short: "Delete a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteSnapshot(cmd, args[0], name)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Snapshot name to delete (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func createSnapshot(cmd *cobra.Command, cageName, snapshotName, description string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Creating snapshot '%s' for cage '%s'...\n", snapshotName, cageName)

	if err := snapshot.Create(cageName, snapshotName, description); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Snapshot '%s' created\n", snapshotName)
	return nil
}

func listSnapshots(cmd *cobra.Command, cageName string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	snapshots, err := snapshot.List(cageName)
	if err != nil {
		return err
	}

	if len(snapshots) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No snapshots found for cage '%s'\n", cageName)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Snapshots for cage '%s':\n", cageName)
	fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %s\n", "NAME", "CREATED", "STATE", "DESCRIPTION")

	for _, s := range snapshots {
		created := ""
		if !s.CreatedAt.IsZero() {
			created = s.CreatedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %s\n",
			truncateString(s.Name, 20),
			created,
			s.State,
			truncateString(s.Description, 40))
	}

	return nil
}

func restoreSnapshot(cmd *cobra.Command, cageName, snapshotName string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Restoring cage '%s' to snapshot '%s'...\n", cageName, snapshotName)

	if err := snapshot.Restore(cageName, snapshotName); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Snapshot '%s' restored\n", snapshotName)
	return nil
}

func deleteSnapshot(cmd *cobra.Command, cageName, snapshotName string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deleting snapshot '%s' from cage '%s'...\n", snapshotName, cageName)

	if err := snapshot.Delete(cageName, snapshotName); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Snapshot '%s' deleted\n", snapshotName)
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
