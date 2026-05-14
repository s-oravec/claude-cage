package snapshot

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/s-oravec/claude-cage/internal/mode"
)

// virsh runs a virsh command against the libvirt URI for the current mode.
func virsh(args ...string) *exec.Cmd {
	fullArgs := append([]string{"-c", mode.Current().URI()}, args...)
	return exec.Command("virsh", fullArgs...)
}

var (
	ErrCageNotFound     = errors.New("cage not found")
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrCageRunning      = errors.New("cage must be stopped for this operation")
	ErrSnapshotExists   = errors.New("snapshot already exists")
)

// Snapshot represents a VM snapshot
type Snapshot struct {
	Name        string    `json:"name"`
	CageName    string    `json:"cage_name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	State       string    `json:"state"` // running, shutoff
}

// domainSnapshot is the XML structure for libvirt snapshots
type domainSnapshot struct {
	XMLName      xml.Name `xml:"domainsnapshot"`
	Name         string   `xml:"name"`
	Description  string   `xml:"description,omitempty"`
	State        string   `xml:"state,omitempty"`
	CreationTime int64    `xml:"creationTime,omitempty"`
}

// Create creates a new snapshot of a cage
func Create(cageName, snapshotName, description string) error {
	domainName := "cage-" + cageName

	// Check if domain exists
	cmd := virsh("dominfo", domainName)
	if err := cmd.Run(); err != nil {
		return ErrCageNotFound
	}

	// Check if snapshot already exists
	cmd = virsh("snapshot-info", domainName, snapshotName)
	if cmd.Run() == nil {
		return ErrSnapshotExists
	}

	// Build snapshot XML
	snapXML := fmt.Sprintf(`<domainsnapshot>
  <name>%s</name>
  <description>%s</description>
</domainsnapshot>`, snapshotName, description)

	// Create snapshot
	cmd = virsh("snapshot-create", domainName, "--xmldesc", "/dev/stdin")
	cmd.Stdin = bytes.NewBufferString(snapXML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// List returns all snapshots for a cage
func List(cageName string) ([]Snapshot, error) {
	domainName := "cage-" + cageName

	// Check if domain exists
	cmd := virsh("dominfo", domainName)
	if err := cmd.Run(); err != nil {
		return nil, ErrCageNotFound
	}

	// List snapshot names
	cmd = virsh("snapshot-list", domainName, "--name")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var snapshots []Snapshot
	names := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Get snapshot details
		snap, err := getSnapshotInfo(domainName, name)
		if err != nil {
			continue // Skip snapshots we can't read
		}
		snap.CageName = cageName
		snapshots = append(snapshots, *snap)
	}

	return snapshots, nil
}

// getSnapshotInfo retrieves detailed information about a snapshot
func getSnapshotInfo(domainName, snapshotName string) (*Snapshot, error) {
	cmd := virsh("snapshot-dumpxml", domainName, snapshotName)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var snapXML domainSnapshot
	if err := xml.Unmarshal(output, &snapXML); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		Name:        snapXML.Name,
		Description: snapXML.Description,
		State:       snapXML.State,
	}

	if snapXML.CreationTime > 0 {
		snap.CreatedAt = time.Unix(snapXML.CreationTime, 0)
	}

	return snap, nil
}

// Restore reverts a cage to a snapshot
func Restore(cageName, snapshotName string) error {
	domainName := "cage-" + cageName

	// Check if domain exists
	cmd := virsh("dominfo", domainName)
	if err := cmd.Run(); err != nil {
		return ErrCageNotFound
	}

	// Check if snapshot exists
	cmd = virsh("snapshot-info", domainName, snapshotName)
	if err := cmd.Run(); err != nil {
		return ErrSnapshotNotFound
	}

	// Revert to snapshot
	cmd = virsh("snapshot-revert", domainName, snapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// Delete removes a snapshot
func Delete(cageName, snapshotName string) error {
	domainName := "cage-" + cageName

	// Check if domain exists
	cmd := virsh("dominfo", domainName)
	if err := cmd.Run(); err != nil {
		return ErrCageNotFound
	}

	// Check if snapshot exists
	cmd = virsh("snapshot-info", domainName, snapshotName)
	if err := cmd.Run(); err != nil {
		return ErrSnapshotNotFound
	}

	// Delete snapshot
	cmd = virsh("snapshot-delete", domainName, snapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// Exists checks if a snapshot exists
func Exists(cageName, snapshotName string) bool {
	domainName := "cage-" + cageName
	cmd := virsh("snapshot-info", domainName, snapshotName)
	return cmd.Run() == nil
}
