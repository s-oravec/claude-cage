# Fáza 06: Virtio-fs File Sharing

Real-time zdieľanie súborov medzi hostom a VM.

## Cieľ

- Spustiť virtiofsd s hardening
- Mount /workspace vo VM
- Real-time sync obojsmerne

## Závisí na

- Fáza 05 (SSH)

## Implementácia

### 1. Virtiofsd management

```go
// internal/virtiofs/daemon.go
type Daemon struct {
    Name       string
    SocketPath string
    SharedDir  string
    Process    *os.Process
}

func Start(cageName string, sharedDir string) (*Daemon, error) {
    socketDir := filepath.Join("/run/cage", cageName)
    os.MkdirAll(socketDir, 0755)
    socketPath := filepath.Join(socketDir, "virtiofs.sock")

    // Expand ~ in sharedDir
    sharedDir = expandPath(sharedDir)

    args := []string{
        "--socket-path=" + socketPath,
        "--shared-dir=" + sharedDir,
        "--sandbox=chroot",      // chroot isolation
        "--seccomp=kill",        // seccomp filter
        "--cache=auto",
    }

    cmd := exec.Command("virtiofsd", args...)
    cmd.Start()

    return &Daemon{
        Name:       cageName,
        SocketPath: socketPath,
        SharedDir:  sharedDir,
        Process:    cmd.Process,
    }, nil
}

func (d *Daemon) Stop() error {
    return d.Process.Signal(syscall.SIGTERM)
}
```

### 2. Update domain XML

```xml
<domain type='kvm'>
  <!-- ... existing ... -->

  <memoryBacking>
    <source type='memfd'/>
    <access mode='shared'/>
  </memoryBacking>

  <devices>
    <!-- ... existing ... -->

    <!-- Virtio-fs -->
    <filesystem type='mount' accessmode='passthrough'>
      <driver type='virtiofs' queue='1024'/>
      <source socket='{{.VirtiofsSocket}}'/>
      <target dir='workspace'/>
    </filesystem>
  </devices>
</domain>
```

### 3. VM mount configuration

V cloud-init pridať mount:

```yaml
#cloud-config
mounts:
  - [ workspace, /workspace, virtiofs, "defaults", "0", "0" ]

runcmd:
  - mkdir -p /workspace
  - mount -t virtiofs workspace /workspace || true
```

Alebo cez `/etc/fstab`:
```
workspace /workspace virtiofs defaults 0 0
```

### 4. Update start workflow

```go
func Start(name string, opts StartOptions) error {
    // ... existing code ...

    // 7. Start virtiofsd
    cfg := config.Load()
    share := cfg.Shares[0] // default share
    hostDir := expandPath(share.Host)

    if !dirExists(hostDir) {
        return fmt.Errorf("share directory does not exist: %s", hostDir)
    }

    daemon, err := virtiofs.Start(name, hostDir)
    if err != nil {
        return fmt.Errorf("failed to start virtiofsd: %w", err)
    }

    // Save daemon info for cleanup
    state.VirtiofsPID = daemon.Process.Pid

    // ... create domain with virtiofs socket ...

    return nil
}

func Stop(name string, force bool) error {
    // ... existing code ...

    // Stop virtiofsd
    state := loadState(name)
    if state.VirtiofsPID > 0 {
        syscall.Kill(state.VirtiofsPID, syscall.SIGTERM)
    }

    // Cleanup socket
    os.RemoveAll(filepath.Join("/run/cage", name))

    // ... rest of cleanup ...
}
```

### 5. Permission handling

```go
// Ensure host directory is accessible
func validateShareDir(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    if !info.IsDir() {
        return errors.New("not a directory")
    }
    // Check readable
    if info.Mode().Perm()&0444 == 0 {
        return errors.New("directory not readable")
    }
    return nil
}
```

### 6. Socket structure

```
/run/cage/
├── myproject/
│   └── virtiofs.sock
└── another-cage/
    └── virtiofs.sock
```

## Acceptance test

```bash
# Setup test directory
mkdir -p ~/projects/testproject
echo "hello from host" > ~/projects/testproject/test.txt

# Start with share (from config)
./cage start --name test

# Verify mount
./cage ssh test "ls -la /workspace"
# test.txt

./cage ssh test "cat /workspace/test.txt"
# hello from host

# Test sync host → VM
echo "updated" > ~/projects/testproject/test.txt
./cage ssh test "cat /workspace/test.txt"
# updated

# Test sync VM → host
./cage ssh test "echo 'from vm' > /workspace/fromvm.txt"
cat ~/projects/testproject/fromvm.txt
# from vm

# Cleanup
./cage stop test
```

## Deliverables

- [ ] Virtiofsd process management
- [ ] Socket handling
- [ ] Hardened virtiofsd flags (--sandbox, --seccomp)
- [ ] Domain XML with virtiofs
- [ ] Cloud-init mount configuration
- [ ] Automatic mount on VM boot
- [ ] Cleanup on stop
- [ ] Share path validation
- [ ] Permission handling
