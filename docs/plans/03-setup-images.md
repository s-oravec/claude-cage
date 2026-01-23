# Fáza 03: Setup & Base Images

Stiahnutie a príprava base images.

## Cieľ

- `cage setup` - stiahne base image a pripraví ho
- Image management (download, prepare, verify)

## Závisí na

- Fáza 02 (config)

## Implementácia

### 1. Image sources

```go
// internal/images/sources.go
var BaseImages = map[string]ImageSource{
    "ubuntu-24.04": {
        URL:      "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
        Checksum: "sha256:...",
        Size:     "~285MB",
    },
    "ubuntu-22.04": {
        URL:      "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
        Checksum: "sha256:...",
        Size:     "~270MB",
    },
    "debian-12": {
        URL:      "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2",
        Checksum: "sha256:...",
        Size:     "~250MB",
    },
}
```

### 2. Setup workflow

```go
func Setup(imageName string) error {
    // 1. Download cloud image
    if err := downloadImage(imageName); err != nil {
        return err
    }

    // 2. Verify checksum
    if err := verifyChecksum(imageName); err != nil {
        return err
    }

    // 3. Convert to qcow2 if needed
    if err := convertToQcow2(imageName); err != nil {
        return err
    }

    // 4. Customize image (virt-customize)
    if err := customizeImage(imageName); err != nil {
        return err
    }

    return nil
}
```

### 3. Image customization

```bash
# Použiť virt-customize (libguestfs)
virt-customize -a ~/.claude-cage/images/ubuntu-24.04.qcow2 \
    --install docker.io,openssh-server \
    --run-command 'systemctl enable docker ssh' \
    --run-command 'usermod -aG docker cage' \
    --selinux-relabel
```

Alebo pripraviť vlastný cloud-init, ktorý sa spustí pri prvom boot.

### 4. Adresárová štruktúra

```
~/.claude-cage/images/
├── ubuntu-24.04.qcow2      # base image (read-only)
├── debian-12.qcow2         # base image
└── checksums.json          # verified checksums
```

### 5. CLI

```go
// cage setup (interactive)
func setup(cmd *cobra.Command, args []string) error {
    // Prompt for image selection
    images := []string{"ubuntu-24.04", "ubuntu-22.04", "debian-12"}
    selected := promptSelect("Select base image:", images)

    return images.Setup(selected)
}

// cage setup --base ubuntu-24.04
func setupWithBase(cmd *cobra.Command, args []string) error {
    base, _ := cmd.Flags().GetString("base")
    return images.Setup(base)
}
```

### 6. Progress display

```
Downloading ubuntu-24.04...
  [████████████████████████] 100% (285 MB)

Verifying checksum...
  ✓ SHA256 verified

Installing Docker into image...
Installing cage-agent...
Configuring SSH...

✓ Base image ready: ubuntu-24.04
```

## Acceptance test

```bash
# Interactive
./cage setup
# ? Select base image: Ubuntu 24.04 LTS
# Downloading...
# ✓ Base image ready

# Non-interactive
./cage setup --base debian-12
# Downloading...
# ✓ Base image ready

# Verify
ls ~/.claude-cage/images/
# ubuntu-24.04.qcow2
```

## Deliverables

- [x] Image download with progress
- [ ] Checksum verification (deferred)
- [x] qcow2 conversion
- [ ] Image customization (Docker, SSH) - using cloud-init at boot instead
- [x] Interactive image selection (`--list`)
- [x] `cage setup` command
- [x] `cage setup --base <name>`
- [ ] `cage setup --all` (deferred)
