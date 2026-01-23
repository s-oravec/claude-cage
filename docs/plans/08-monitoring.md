# Fáza 08: Monitoring

Status, exec a logs príkazy pre monitoring a debugging.

## Cieľ

- `cage status` - detailný stav cage
- `cage exec` - spustenie príkazu bez TTY
- `cage logs` - logy z VM

## Závisí na

- Fáza 07 (network)

## Implementácia

### 1. cage status

```go
// internal/cmd/status.go
func status(cmd *cobra.Command, args []string) error {
    name := args[0]
    state := cage.LoadState(name)

    client := libvirt.NewClient()
    domain, _ := client.LookupDomainByName("cage-" + name)

    // Get resource usage
    info, _ := domain.GetInfo()
    cpuStats, _ := domain.GetCPUStats(-1, 0)
    memStats, _ := domain.MemoryStats(10, 0)

    // Get Docker info (via SSH)
    dockerInfo := getDockerInfo(name)

    // Format output
    fmt.Printf("Cage: %s\n", name)
    fmt.Printf("Status: %s\n", state.Status)
    fmt.Printf("Uptime: %s\n", formatUptime(state.StartedAt))
    fmt.Println()
    fmt.Println("Resources:")
    fmt.Printf("  Profile: %s\n", state.Profile)
    fmt.Printf("  vCPU:    %d (usage: %.1f%%)\n", info.NrVirtCpu, cpuUsage)
    fmt.Printf("  Memory:  %d MB (usage: %d MB / %.0f%%)\n",
        state.MemoryMB, memUsed, memPercent)
    fmt.Println()
    fmt.Println("Network:")
    fmt.Printf("  IP:      %s\n", state.IP)
    fmt.Printf("  Bridge:  cage-%s\n", name[:8])
    if len(state.Ports) > 0 {
        fmt.Println("  Ports:")
        for _, p := range state.Ports {
            fmt.Printf("    - %d → %d\n", p.Host, p.Guest)
        }
    }
    // ...
}
```

### 2. cage status --watch

```go
func statusWatch(name string) error {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        // Clear screen
        fmt.Print("\033[H\033[2J")

        // Print status
        printStatus(name)

        // Wait for next tick or Ctrl+C
        select {
        case <-ticker.C:
            continue
        case <-sigChan:
            return nil
        }
    }
}
```

### 3. cage exec

```go
// internal/cmd/exec.go
func execCmd(cmd *cobra.Command, args []string) error {
    name := args[0]
    command := args[1:] // everything after --

    state := cage.LoadState(name)
    if state.Status != "running" {
        return ErrCageNotRunning
    }

    keyPath := filepath.Join(config.KeysDir(), name, "id_ed25519")

    // SSH without TTY allocation (-T)
    sshArgs := []string{
        "-T", // no TTY
        "-i", keyPath,
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "LogLevel=ERROR",
        fmt.Sprintf("cage@%s", state.IP),
    }
    sshArgs = append(sshArgs, command...)

    sshCmd := exec.Command("ssh", sshArgs...)
    sshCmd.Stdout = os.Stdout
    sshCmd.Stderr = os.Stderr
    return sshCmd.Run()
}
```

### 4. cage logs

```go
// internal/cmd/logs.go
func logs(cmd *cobra.Command, args []string) error {
    name := args[0]
    follow, _ := cmd.Flags().GetBool("follow")
    lines, _ := cmd.Flags().GetInt("lines")

    // Get logs from journalctl in VM
    journalCmd := fmt.Sprintf("journalctl -n %d", lines)
    if follow {
        journalCmd += " -f"
    }

    return cage.SSH(name, journalCmd)
}
```

### 5. Helper: Get Docker info

```go
func getDockerInfo(name string) *DockerInfo {
    // Running containers
    out, _ := cage.Exec(name, "docker ps -q | wc -l")
    running, _ := strconv.Atoi(strings.TrimSpace(out))

    // Stopped containers
    out, _ = cage.Exec(name, "docker ps -aq | wc -l")
    total, _ := strconv.Atoi(strings.TrimSpace(out))

    // Images
    out, _ = cage.Exec(name, "docker images -q | wc -l")
    images, _ := strconv.Atoi(strings.TrimSpace(out))

    return &DockerInfo{
        Running: running,
        Stopped: total - running,
        Images:  images,
    }
}
```

### 6. Helper: Get top processes

```go
func getTopProcesses(name string, count int) []ProcessInfo {
    cmd := fmt.Sprintf("ps aux --sort=-pcpu | head -n %d", count+1)
    out, _ := cage.Exec(name, cmd)

    // Parse ps output
    var processes []ProcessInfo
    lines := strings.Split(out, "\n")
    for _, line := range lines[1:] { // skip header
        fields := strings.Fields(line)
        if len(fields) >= 11 {
            processes = append(processes, ProcessInfo{
                PID:     fields[1],
                CPU:     fields[2],
                MEM:     fields[3],
                Command: strings.Join(fields[10:], " "),
            })
        }
    }
    return processes
}
```

## Output format

```
Cage: backend
Status: running
Uptime: 2h 15m 30s

Resources:
  Profile: heavy
  vCPU:    8 (usage: 25%)
  Memory:  8192 MB (usage: 2100 MB / 26%)

Network:
  IP:      192.168.100.2
  Bridge:  cage-bac
  Ports:
    - 8080 → 80
    - 5432 → 5432

Docker:
  Containers: 3 running, 1 stopped
  Images:     12

Shares:
  - ~/projects/backend → /workspace

Processes (top 5 by CPU):
  PID    CPU%   MEM%   COMMAND
  1234   15.2   3.1    dockerd
  2345   8.5    12.4   node
  3456   1.2    0.5    postgres
```

## Acceptance test

```bash
# Start cage
./cage start --name test

# Status
./cage status test
# Cage: test
# Status: running
# ...

# Status JSON
./cage status test --json
# {"name": "test", ...}

# Exec (no TTY)
./cage exec test -- uname -a
# Linux test 6.x.x ...

./cage exec test -- docker ps
# CONTAINER ID ...

# Logs
./cage logs test -n 50
# (last 50 log lines)

# Logs follow
./cage logs test -f
# (streaming logs, Ctrl+C to stop)

# Cleanup
./cage stop test
```

## Deliverables

- [ ] `cage status <name>`
- [ ] `cage status <name> --json`
- [ ] `cage status <name> --watch`
- [ ] `cage exec <name> -- <command>`
- [ ] `cage logs <name>`
- [ ] `cage logs <name> -f`
- [ ] `cage logs <name> -n <lines>`
- [ ] Resource usage from libvirt
- [ ] Docker info collection
- [ ] Process listing
