# Fáza 10: Advanced Lifecycle

Restart a snapshots pre pokročilú správu VM.

## Cieľ

- `cage restart` - reštart cage
- `cage snapshot` - vytvorenie, obnovenie, zmazanie snapshots

## Závisí na

- Fáza 09 (port forwarding)

## Implementácia

### 1. cage restart

```go
// internal/cmd/restart.go
func restart(cmd *cobra.Command, args []string) error {
    name := args[0]
    force, _ := cmd.Flags().GetBool("force")

    // Load current state to preserve config
    state := cage.LoadState(name)

    // Stop
    fmt.Printf("Stopping %s...\n", name)
    if err := cage.Stop(name, force); err != nil {
        return err
    }

    // Start with same options
    fmt.Printf("Starting %s...\n", name)
    opts := cage.StartOptions{
        Name:    name,
        Image:   state.Image,
        Profile: state.Profile,
        Ports:   state.Ports,
    }

    if err := cage.Start(name, opts); err != nil {
        return err
    }

    fmt.Printf("✓ Cage '%s' restarted\n", name)
    return nil
}
```

### 2. Snapshot types

```go
// internal/snapshot/types.go
type Snapshot struct {
    Name        string    `json:"name"`
    CageName    string    `json:"cage_name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    Size        int64     `json:"size_bytes"`
}
```

### 3. cage snapshot create

```go
// internal/snapshot/create.go
func Create(cageName, snapshotName, description string) error {
    client := libvirt.NewClient()
    domain, err := client.LookupDomainByName("cage-" + cageName)
    if err != nil {
        return ErrCageNotFound
    }

    // Create internal qcow2 snapshot
    xml := fmt.Sprintf(`
<domainsnapshot>
  <name>%s</name>
  <description>%s</description>
</domainsnapshot>`, snapshotName, description)

    _, err = domain.CreateSnapshotXML(xml, 0)
    if err != nil {
        return fmt.Errorf("failed to create snapshot: %w", err)
    }

    // Save metadata
    snap := Snapshot{
        Name:        snapshotName,
        CageName:    cageName,
        Description: description,
        CreatedAt:   time.Now(),
    }
    saveSnapshotMetadata(cageName, snap)

    return nil
}
```

### 4. cage snapshot list

```go
// internal/snapshot/list.go
func List(cageName string) ([]Snapshot, error) {
    client := libvirt.NewClient()
    domain, err := client.LookupDomainByName("cage-" + cageName)
    if err != nil {
        return nil, ErrCageNotFound
    }

    // Get all snapshots
    snapshots, err := domain.ListAllSnapshots(0)
    if err != nil {
        return nil, err
    }

    var result []Snapshot
    for _, snap := range snapshots {
        name, _ := snap.GetName()
        xml, _ := snap.GetXMLDesc(0)
        // Parse XML for details
        result = append(result, parseSnapshotXML(cageName, xml))
    }

    return result, nil
}
```

### 5. cage snapshot restore

```go
// internal/snapshot/restore.go
func Restore(cageName, snapshotName string) error {
    // Check cage is stopped
    state := cage.LoadState(cageName)
    if state.Status == "running" {
        return errors.New("cage must be stopped before restore")
    }

    client := libvirt.NewClient()
    domain, err := client.LookupDomainByName("cage-" + cageName)
    if err != nil {
        return ErrCageNotFound
    }

    // Find snapshot
    snap, err := domain.SnapshotLookupByName(snapshotName, 0)
    if err != nil {
        return ErrSnapshotNotFound
    }

    // Revert to snapshot
    err = snap.RevertToSnapshot(0)
    if err != nil {
        return fmt.Errorf("failed to restore snapshot: %w", err)
    }

    return nil
}
```

### 6. cage snapshot delete

```go
// internal/snapshot/delete.go
func Delete(cageName, snapshotName string) error {
    client := libvirt.NewClient()
    domain, err := client.LookupDomainByName("cage-" + cageName)
    if err != nil {
        return ErrCageNotFound
    }

    snap, err := domain.SnapshotLookupByName(snapshotName, 0)
    if err != nil {
        return ErrSnapshotNotFound
    }

    err = snap.Delete(0)
    if err != nil {
        return fmt.Errorf("failed to delete snapshot: %w", err)
    }

    // Remove metadata
    deleteSnapshotMetadata(cageName, snapshotName)

    return nil
}
```

### 7. CLI commands

```go
// cage snapshot create
cmd := &cobra.Command{
    Use:   "create <cage-name>",
    Short: "Create a snapshot",
    RunE: func(cmd *cobra.Command, args []string) error {
        name, _ := cmd.Flags().GetString("name")
        desc, _ := cmd.Flags().GetString("description")
        return snapshot.Create(args[0], name, desc)
    },
}
cmd.Flags().StringP("name", "n", "", "Snapshot name (required)")
cmd.Flags().StringP("description", "d", "", "Description")
cmd.MarkFlagRequired("name")

// cage snapshot list
cmd := &cobra.Command{
    Use:   "list <cage-name>",
    Short: "List snapshots",
    RunE: func(cmd *cobra.Command, args []string) error {
        snaps, err := snapshot.List(args[0])
        if err != nil {
            return err
        }
        // Format output
        fmt.Println("NAME               CREATED              SIZE     DESCRIPTION")
        for _, s := range snaps {
            fmt.Printf("%-18s %-20s %-8s %s\n",
                s.Name,
                s.CreatedAt.Format("2006-01-02 15:04:05"),
                formatSize(s.Size),
                s.Description)
        }
        return nil
    },
}

// cage snapshot restore
// cage snapshot delete
```

## Acceptance test

```bash
# Start cage
./cage start --name test

# Do some work
./cage ssh test "echo 'original state' > /tmp/state.txt"

# Create snapshot
./cage snapshot create test --name before-experiment
# ✓ Snapshot 'before-experiment' created

# List snapshots
./cage snapshot list test
# NAME               CREATED              SIZE
# before-experiment  2024-01-23 14:30:00  256 MB

# Make changes
./cage ssh test "echo 'modified' > /tmp/state.txt"
./cage ssh test "cat /tmp/state.txt"
# modified

# Restore snapshot
./cage stop test
./cage snapshot restore test --name before-experiment
# ✓ Snapshot restored

./cage start --name test
./cage ssh test "cat /tmp/state.txt"
# original state

# Restart test
./cage restart test
./cage ssh test "whoami"
# cage

# Delete snapshot
./cage snapshot delete test --name before-experiment
# ✓ Snapshot deleted

# Cleanup
./cage stop test
```

## Deliverables

- [ ] `cage restart <name>`
- [ ] `cage restart <name> --force`
- [ ] `cage snapshot create <name> --name <snapshot>`
- [ ] `cage snapshot create <name> --name <snapshot> --description <desc>`
- [ ] `cage snapshot list <name>`
- [ ] `cage snapshot restore <name> --name <snapshot>`
- [ ] `cage snapshot delete <name> --name <snapshot>`
- [ ] Snapshot metadata storage
- [ ] Validation (cage must be stopped for restore)
