# Fáza 04: VM Lifecycle

Základný životný cyklus VM - start, stop, list.

## Cieľ

- `cage start` - spustí VM
- `cage stop` - zastaví VM
- `cage list` - zobrazí bežiace cages

## Závisí na

- Fáza 03 (setup, base images)

## Implementácia

### 1. Libvirt wrapper

```go
// internal/libvirt/client.go
type Client struct {
    conn *libvirt.Connect
}

func NewClient() (*Client, error) {
    conn, err := libvirt.NewConnect("qemu:///session")
    if err != nil {
        return nil, err
    }
    return &Client{conn: conn}, nil
}

func (c *Client) CreateDomain(cfg *CageConfig) (*libvirt.Domain, error) {
    xml := generateDomainXML(cfg)
    return c.conn.DomainDefineXML(xml)
}
```

### 2. Domain XML template

```xml
<domain type='kvm'>
  <name>cage-{{.Name}}</name>
  <memory unit='MiB'>{{.MemoryMB}}</memory>
  <vcpu>{{.VCPU}}</vcpu>

  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>

  <features>
    <acpi/>
    <apic/>
  </features>

  <cpu mode='host-passthrough'/>

  <cputune>
    <shares>1024</shares>
    <period>100000</period>
    <quota>{{.CPUQuota}}</quota>
  </cputune>

  <memtune>
    <hard_limit unit='MiB'>{{.MemoryMB}}</hard_limit>
  </memtune>

  <devices>
    <!-- Disk (qcow2 overlay) -->
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='{{.DiskPath}}'/>
      <target dev='vda' bus='virtio'/>
    </disk>

    <!-- Cloud-init -->
    <disk type='file' device='cdrom'>
      <source file='{{.CloudInitISO}}'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>

    <!-- Network -->
    <interface type='network'>
      <source network='cage-{{.Name}}'/>
      <model type='virtio'/>
    </interface>

    <!-- Console -->
    <serial type='pty'>
      <target port='0'/>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
  </devices>
</domain>
```

### 3. Cage state management

```go
// internal/cage/state.go
type CageState struct {
    Name      string    `json:"name"`
    Status    string    `json:"status"` // running, stopped
    Image     string    `json:"image"`
    Profile   string    `json:"profile"`
    IP        string    `json:"ip"`
    Ports     []Port    `json:"ports"`
    StartedAt time.Time `json:"started_at"`
}

// ~/.claude-cage/cages/<name>/
//   state.json
//   disk.qcow2 (overlay)
//   cloud-init.iso
```

### 4. Start workflow

```go
func Start(name string, opts StartOptions) error {
    // 1. Validate name unique
    if cageExists(name) {
        return ErrCageExists
    }

    // 2. Load config and profile
    cfg := config.Load()
    profile := cfg.Profiles[opts.Profile]

    // 3. Create cage directory
    cageDir := filepath.Join(config.CagesDir(), name)
    os.MkdirAll(cageDir, 0755)

    // 4. Create qcow2 overlay
    baseImage := filepath.Join(config.ImagesDir(), opts.Image+".qcow2")
    overlay := filepath.Join(cageDir, "disk.qcow2")
    exec.Command("qemu-img", "create", "-f", "qcow2",
        "-b", baseImage, "-F", "qcow2", overlay).Run()

    // 5. Generate SSH keys (Fáza 05)
    // 6. Create cloud-init ISO (Fáza 05)
    // 7. Create network (Fáza 07)

    // 8. Create and start libvirt domain
    client := libvirt.NewClient()
    domain := client.CreateDomain(...)
    domain.Create()

    // 9. Save state
    saveState(name, CageState{...})

    return nil
}
```

### 5. Stop workflow

```go
func Stop(name string, force bool) error {
    client := libvirt.NewClient()
    domain, err := client.LookupDomainByName("cage-" + name)
    if err != nil {
        return ErrCageNotFound
    }

    if force {
        domain.Destroy() // immediate
    } else {
        domain.Shutdown() // graceful
        waitForShutdown(domain, 30*time.Second)
    }

    // Cleanup
    domain.Undefine()
    os.RemoveAll(filepath.Join(config.CagesDir(), name))

    return nil
}
```

### 6. List

```go
func List() ([]CageState, error) {
    client := libvirt.NewClient()
    domains, _ := client.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)

    var cages []CageState
    for _, domain := range domains {
        name, _ := domain.GetName()
        if strings.HasPrefix(name, "cage-") {
            state := loadState(strings.TrimPrefix(name, "cage-"))
            cages = append(cages, state)
        }
    }
    return cages, nil
}
```

## Acceptance test

```bash
# Start
./cage start --name test
# ✓ Cage 'test' started

# List
./cage list
# NAME   STATUS    IMAGE          PROFILE   UPTIME
# test   running   ubuntu-24.04   default   5s

# Stop
./cage stop test
# ✓ Cage 'test' stopped

# List (empty)
./cage list
# No cages running
```

## Deliverables

- [x] Libvirt client wrapper (using virsh CLI)
- [x] Domain XML generation
- [x] qcow2 overlay creation
- [x] `cage start --name <name>`
- [x] `cage start --profile <profile>`
- [x] `cage start --image <image>`
- [x] `cage stop <name>`
- [x] `cage stop --force`
- [x] `cage stop --all`
- [x] `cage list`
- [x] `cage list --json`
- [x] Cage state persistence
