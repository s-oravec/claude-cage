# Fáza 09: Port Forwarding

Dynamické port forwarding pre prístup k službám v cage.

## Cieľ

- `--port` flag pri `cage start`
- `cage port` subcommands (add, list, remove)
- Default bind na 127.0.0.1

## Závisí na

- Fáza 08 (monitoring)

## Implementácia

### 1. Port forwarding types

```go
// internal/network/ports.go
type PortForward struct {
    HostPort  int    `json:"host"`
    GuestPort int    `json:"guest"`
    Protocol  string `json:"protocol"` // tcp, udp
    Bind      string `json:"bind"`     // 127.0.0.1 or 0.0.0.0
}

func ParsePortFlag(spec string) (*PortForward, error) {
    // Format: [bind:]hostPort:guestPort[/protocol]
    // Examples: 8080:80, 127.0.0.1:8080:80, 8080:80/udp

    parts := strings.Split(spec, ":")
    // ... parse logic ...
}
```

### 2. SSH-based port forwarding

```go
// internal/network/forward.go
type Forwarder struct {
    CageName string
    Forwards []PortForward
    SSHProc  *os.Process
}

func StartForwarding(cageName string, forwards []PortForward) (*Forwarder, error) {
    state := cage.LoadState(cageName)
    keyPath := filepath.Join(config.KeysDir(), cageName, "id_ed25519")

    args := []string{
        "-N", // no command
        "-T", // no TTY
        "-i", keyPath,
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "ExitOnForwardFailure=yes",
    }

    for _, fwd := range forwards {
        args = append(args, "-L",
            fmt.Sprintf("%s:%d:%s:%d",
                fwd.Bind, fwd.HostPort,
                state.IP, fwd.GuestPort))
    }

    args = append(args, fmt.Sprintf("cage@%s", state.IP))

    cmd := exec.Command("ssh", args...)
    cmd.Start()

    return &Forwarder{
        CageName: cageName,
        Forwards: forwards,
        SSHProc:  cmd.Process,
    }, nil
}

func (f *Forwarder) Stop() error {
    return f.SSHProc.Signal(syscall.SIGTERM)
}
```

### 3. Alternative: iptables DNAT

```go
// For more persistent forwarding, use iptables
func AddPortForward(cageName string, fwd PortForward) error {
    state := cage.LoadState(cageName)

    // DNAT for incoming traffic
    cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
        "-p", fwd.Protocol,
        "-d", fwd.Bind,
        "--dport", strconv.Itoa(fwd.HostPort),
        "-j", "DNAT",
        "--to-destination", fmt.Sprintf("%s:%d", state.IP, fwd.GuestPort))

    return cmd.Run()
}
```

### 4. cage start --port

```go
func Start(name string, opts StartOptions) error {
    // ... existing code ...

    // Parse port flags
    var forwards []PortForward
    for _, spec := range opts.Ports {
        fwd, err := ParsePortFlag(spec)
        if err != nil {
            return err
        }
        forwards = append(forwards, *fwd)
    }

    // ... create VM ...

    // Start port forwarding
    if len(forwards) > 0 {
        forwarder, err := network.StartForwarding(name, forwards)
        if err != nil {
            return err
        }
        state.ForwarderPID = forwarder.SSHProc.Pid
    }

    // Save forwards to state
    state.Ports = forwards
    saveState(name, state)

    return nil
}
```

### 5. cage port commands

```go
// cage port list
func portList(cmd *cobra.Command, args []string) error {
    name := args[0]
    state := cage.LoadState(name)

    fmt.Println("HOST       GUEST    PROTOCOL")
    for _, p := range state.Ports {
        fmt.Printf("%s:%-5d → %-5d    %s\n",
            p.Bind, p.HostPort, p.GuestPort, p.Protocol)
    }
    return nil
}

// cage port add
func portAdd(cmd *cobra.Command, args []string) error {
    name := args[0]
    spec := args[1]

    fwd, err := ParsePortFlag(spec)
    if err != nil {
        return err
    }

    // Check port not already used
    state := cage.LoadState(name)
    for _, existing := range state.Ports {
        if existing.HostPort == fwd.HostPort {
            return fmt.Errorf("port %d already forwarded", fwd.HostPort)
        }
    }

    // Add forward
    network.AddForward(name, *fwd)

    // Update state
    state.Ports = append(state.Ports, *fwd)
    saveState(name, state)

    fmt.Printf("Added: %s:%d → %d\n", fwd.Bind, fwd.HostPort, fwd.GuestPort)
    return nil
}

// cage port remove
func portRemove(cmd *cobra.Command, args []string) error {
    name := args[0]
    hostPort, _ := strconv.Atoi(args[1])

    state := cage.LoadState(name)

    // Find and remove
    for i, p := range state.Ports {
        if p.HostPort == hostPort {
            network.RemoveForward(name, p)
            state.Ports = append(state.Ports[:i], state.Ports[i+1:]...)
            saveState(name, state)
            fmt.Printf("Removed port %d\n", hostPort)
            return nil
        }
    }

    return fmt.Errorf("port %d not found", hostPort)
}
```

### 6. Config default bind

```yaml
# ~/.claude-cage/config.yaml
network:
  port_bind: 127.0.0.1  # default: localhost only
  # port_bind: 0.0.0.0  # allow external access
```

## Acceptance test

```bash
# Start with ports
./cage start --name test --port 8080:80 --port 5432:5432

# List ports
./cage port list test
# HOST            GUEST    PROTOCOL
# 127.0.0.1:8080  → 80     tcp
# 127.0.0.1:5432  → 5432   tcp

# Test access
./cage ssh test "python3 -m http.server 80 &"
curl http://localhost:8080
# (should work)

# Add port at runtime
./cage port add test 3000:3000
./cage port list test
# (shows 3 ports)

# Remove port
./cage port remove test 3000
./cage port list test
# (shows 2 ports)

# Stop
./cage stop test
```

## Deliverables

- [x] Port spec parser (`8080:80`, `127.0.0.1:8080:80/tcp`)
- [x] SSH-based port forwarding
- [x] `--port` flag on `cage start`
- [x] Multiple `--port` flags support
- [x] `cage port list <name>`
- [x] `cage port add <name> <spec>`
- [x] `cage port remove <name> <hostPort>`
- [x] Default bind from config
- [x] Port conflict detection
- [x] Cleanup on stop
