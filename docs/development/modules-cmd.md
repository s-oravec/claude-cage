# CLI Commands Module

This document details the command implementations in `internal/cmd/`.

## Command Structure

All commands are implemented using [cobra](https://github.com/spf13/cobra) and follow a consistent pattern:

```go
func NewXxxCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "xxx",
        Short: "Brief description",
        Long:  "Detailed description",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runXxx(cmd, args)
        },
    }
    // Add flags
    return cmd
}
```

## Command Registration

Commands are registered in `root.go`:

```go
func NewRootCmd() *cobra.Command {
    rootCmd := &cobra.Command{
        Use:   "cage",
        Short: "Claude Cage - Secure VM sandbox for Claude Code",
    }

    rootCmd.AddCommand(NewCreateCmd())
    rootCmd.AddCommand(NewStartCmd())
    // ... more commands

    return rootCmd
}
```

## Command Details

### Lifecycle Commands

#### init (`init.go`)
Initializes project configuration.

**Flow:**
1. Check if `.claude-cage.yml` already exists
2. Load default image from global config (if --image not specified)
3. Create `.claude-cage.yml` with image, shares, and options

**Flags:**
- `--image` - Base image (default: from `~/.claude-cage/config.yaml`)
- `--cage` - Cage name (default: directory name)
- `--memory` - Memory allocation (e.g., `4G`)
- `--vcpu` - Number of virtual CPUs
- `--disk` - Disk size in GB
- `--ssh` - SSH port (default: `auto`)
- `-f, --force` - Overwrite existing config
- `--dir` - Target directory (default: current)

#### start (`start.go`)
Starts a cage, creating it if needed.

**Flow:**
1. Resolve cage name (from args or `.claude-cage.yml`)
2. If cage doesn't exist and project config exists: create cage
3. If cage stopped: reconfigure if project config changed
4. Start virtiofsd (if configured)
5. Start libvirt domain
6. Setup port forwarding
7. Update state to "running"

**Flags:**
- `--port` - Additional port forwards

#### stop (`stop.go`)
Stops a running cage.

**Flow:**
1. Load cage state
2. Stop port forwarders
3. Stop virtiofsd
4. Graceful or force shutdown
5. Update state to "stopped"

**Flags:**
- `-f, --force` - Force immediate shutdown
- `-a, --all` - Stop all cages

#### remove (`remove.go`)
Removes a cage and all resources. Alias: `rm`

**Flow:**
1. Load cage state
2. Stop if running (with force)
3. Undefine libvirt domain
4. Cleanup network (bridge mode)
5. Delete SSH keys
6. Delete cage directory
7. Cleanup SSH known_hosts entries

**Flags:**
- `-f, --force` - Force removal of running cage
- `-a, --all` - Remove all cages

#### restart (`restart.go`)
Restarts a running cage.

**Flow:**
1. Stop the cage
2. Start the cage

**Flags:**
- `-f, --force` - Force stop before restart

### Connection Commands

#### ssh (`ssh.go`)
Connects to cage via SSH.

**Flow:**
1. Load cage state
2. Determine connection target (localhost:port or VM IP)
3. Execute SSH with cage's private key

#### exec (`exec.go`)
Executes command without TTY.

**Flow:**
1. Load cage state
2. Execute SSH in batch mode
3. Return output and exit code

#### console (`console.go`)
Connects to serial console.

**Flow:**
1. Execute `virsh console cage-<name>`

### Information Commands

#### list (`list.go`)
Lists all cages. Alias: `ls`

**Output:** Table with name, status, image, profile, network info

**Flags:**
- `--json` - JSON output

#### status (`status.go`)
Shows detailed cage status.

**Output:**
- Name, status, image, profile
- Network mode, SSH port, IP
- Started at timestamp
- Process PIDs

**Flags:**
- `--json` - JSON output
- `-w, --watch` - Continuous updates

#### logs (`logs.go`)
Views cage console logs.

**Flow:**
1. SSH into cage
2. Read from syslog or journald

**Flags:**
- `-n, --lines` - Number of lines
- `-f, --follow` - Follow output

### Management Commands

#### snapshot (`snapshot.go`)
Manages VM snapshots.

**Subcommands:**
- `create <cage> --name <snap>` - Create snapshot
- `list <cage>` - List snapshots (alias: `ls`)
- `restore <cage> --name <snap>` - Restore snapshot
- `remove <cage> --name <snap>` - Remove snapshot (aliases: `rm`, `delete`)

#### port (`port.go`)
Manages port forwarding.

**Subcommands:**
- `add <cage> <host:guest>` - Add forward
- `list <cage>` - List forwards (alias: `ls`)
- `remove <cage> <host>` - Remove forward

#### image (`image.go`)
Manages base and custom images.

**Subcommands:**
- `list` - List images (alias: `ls`)
- `save [cage] --name <img>` - Save stopped cage as image (cage name optional in project directory)
- `remove <img>` - Remove image (aliases: `rm`, `delete`)
- `inspect <img>` - Show image details

**Notes:**
- `save` requires cage to be stopped (prevents corrupted disk state)
- Uses `virt-customize` to prepare images for reuse (clears SSH keys, resets cloud-init)

### Setup Commands

#### pull (`pull.go`)
Downloads base images.

**Flow:**
1. Resolve image alias
2. Download from cloud image URL
3. Convert to qcow2 if needed
4. Save to images directory

**Flags:**
- `-b, --base` - Image to download
- `-l, --list` - List available images

#### config (`config.go`)
Manages configuration.

**Subcommands:**
- `init` - Create default config
- `show` - Display config
- `edit` - Open in editor
- `path` - Show config path

**Flags:**
- `--force` - Overwrite existing

#### doctor (`doctor.go`)
Checks system requirements.

**Flow:**
1. Run all checks (KVM, libvirt, tools)
2. Report pass/fail status
3. Show fix hints if requested

**Flags:**
- `--fix` - Show installation commands

#### verify (`verify.go`)
Tests network isolation.

**Flow:**
1. SSH into cage
2. Test internet connectivity
3. Test DNS resolution
4. Test blocked subnets (should fail)

### Utility Commands

#### version (`version.go`)
Shows version information.

## Error Handling

Commands follow a consistent error handling pattern:

```go
func runXxx(cmd *cobra.Command, args []string) error {
    // Early validation
    if err := validate(); err != nil {
        return err
    }

    // Main operation
    if err := doWork(); err != nil {
        // Cleanup on error
        cleanup()
        return fmt.Errorf("operation failed: %w", err)
    }

    return nil
}
```

## Output Conventions

- **Success:** `✓ Message` (green checkmark via terminal)
- **Progress:** `  Message...` (indented steps)
- **Warnings:** `  Warning: message` (non-fatal issues)
- **Errors:** Return error (displayed by cobra)

## Testing

Each command has a corresponding `*_test.go` file with:
- Unit tests for flag parsing
- Integration tests with mocked dependencies
- Table-driven tests for validation

## See Also

- [Modules Overview](modules.md) - All packages
- [Data Flow](data-flow.md) - Request lifecycle
