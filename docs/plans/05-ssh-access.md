# Fáza 05: SSH Access

SSH prístup do cage s automatickým key management.

## Cieľ

- Automatické generovanie SSH kľúčov (Ed25519)
- Cloud-init injection
- `cage ssh` príkaz

## Závisí na

- Fáza 04 (VM lifecycle)

## Implementácia

### 1. SSH key generation

```go
// internal/ssh/keygen.go
func GenerateKeyPair(cageName string) error {
    keyDir := filepath.Join(config.KeysDir(), cageName)
    os.MkdirAll(keyDir, 0700)

    keyPath := filepath.Join(keyDir, "id_ed25519")

    // ssh-keygen -t ed25519 -f keyPath -N ""
    cmd := exec.Command("ssh-keygen",
        "-t", "ed25519",
        "-f", keyPath,
        "-N", "", // no passphrase
        "-C", fmt.Sprintf("cage@%s", cageName),
    )
    return cmd.Run()
}

func GetPublicKey(cageName string) (string, error) {
    keyPath := filepath.Join(config.KeysDir(), cageName, "id_ed25519.pub")
    data, err := os.ReadFile(keyPath)
    return string(data), err
}
```

### 2. Cloud-init generation

```go
// internal/cloudinit/generate.go
func GenerateISO(cageName string, pubKey string) error {
    // user-data
    userData := fmt.Sprintf(`#cloud-config
users:
  - name: cage
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - %s

ssh_pwauth: false

package_update: false

runcmd:
  - systemctl enable docker
  - systemctl start docker
`, pubKey)

    // meta-data
    metaData := fmt.Sprintf(`instance-id: %s
local-hostname: %s
`, cageName, cageName)

    // Write temp files
    tmpDir := filepath.Join(config.CagesDir(), cageName, "cloudinit")
    os.MkdirAll(tmpDir, 0755)
    os.WriteFile(filepath.Join(tmpDir, "user-data"), []byte(userData), 0644)
    os.WriteFile(filepath.Join(tmpDir, "meta-data"), []byte(metaData), 0644)

    // Generate ISO
    isoPath := filepath.Join(config.CagesDir(), cageName, "cloud-init.iso")
    cmd := exec.Command("cloud-localds", isoPath,
        filepath.Join(tmpDir, "user-data"),
        filepath.Join(tmpDir, "meta-data"))
    return cmd.Run()
}
```

### 3. SSH connection

```go
// internal/ssh/connect.go
func Connect(cageName string, command string) error {
    state := cage.LoadState(cageName)
    if state.Status != "running" {
        return ErrCageNotRunning
    }

    keyPath := filepath.Join(config.KeysDir(), cageName, "id_ed25519")
    knownHostsPath := filepath.Join(config.Dir(), "known_hosts")

    args := []string{
        "-i", keyPath,
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
        "-o", "LogLevel=ERROR",
        fmt.Sprintf("cage@%s", state.IP),
    }

    if command != "" {
        args = append(args, command)
    }

    cmd := exec.Command("ssh", args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### 4. Wait for SSH ready

```go
func WaitForSSH(cageName string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        err := Connect(cageName, "true")
        if err == nil {
            return nil
        }
        time.Sleep(1 * time.Second)
    }
    return ErrSSHTimeout
}
```

### 5. Update start workflow

```go
func Start(name string, opts StartOptions) error {
    // ... existing code ...

    // 5. Generate SSH keys
    ssh.GenerateKeyPair(name)
    pubKey, _ := ssh.GetPublicKey(name)

    // 6. Create cloud-init ISO
    cloudinit.GenerateISO(name, pubKey)

    // ... create domain with cloud-init disk ...

    // 10. Wait for SSH
    fmt.Print("Waiting for SSH... ")
    ssh.WaitForSSH(name, 60*time.Second)
    fmt.Println("ready")

    return nil
}
```

### 6. Key storage structure

```
~/.claude-cage/
├── keys/
│   ├── myproject/
│   │   ├── id_ed25519       # private key (0600)
│   │   └── id_ed25519.pub   # public key
│   └── another-cage/
│       ├── id_ed25519
│       └── id_ed25519.pub
└── known_hosts               # SSH known hosts
```

## Acceptance test

```bash
# Start (generates keys, waits for SSH)
./cage start --name test
# ✓ Cage 'test' started
# Waiting for SSH... ready

# SSH interactive
./cage ssh test
# cage@test:~$

# SSH with command
./cage ssh test "whoami"
# cage

./cage ssh test "docker --version"
# Docker version 24.0.x

# Cleanup
./cage stop test
```

## Deliverables

- [x] Ed25519 key generation
- [x] Per-cage key storage
- [x] Cloud-init user-data generation
- [x] Cloud-init ISO creation
- [x] SSH connection wrapper
- [x] Known hosts management
- [x] SSH ready detection
- [x] `cage ssh <name>` (interactive)
- [x] `cage ssh <name> "<command>"` (run command)
