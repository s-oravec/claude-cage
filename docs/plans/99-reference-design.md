# Claude Cage - Reference Design Document

> **Poznámka:** Toto je referenčný dokument s celkovým dizajnom. Implementačné detaily sú rozdelené do fáz v súboroch `01-*.md` až `11-*.md`. Pozri `00-overview.md` pre prehľad.

Bezpečný sandbox pre Claude Code experimenty, izolované dev prostredie s Docker, a testovanie AI agentov.

**Kľúčový koncept:** Claude Code (a všetko ostatné) beží VNÚTRI cage VM. Host sa pripája len cez SSH.

## Technológie

| Komponent | Technológia |
|-----------|-------------|
| CLI | Go (cobra + viper) |
| VM | QEMU/KVM (cez libvirt) |
| Libvirt SDK | libvirt/libvirt-go-module |
| Networking | libvirt networks + vishvananda/netlink |
| Config | YAML |

### Prečo QEMU/KVM namiesto Firecracker

| Aspekt | Firecracker | QEMU/KVM |
|--------|-------------|----------|
| Docker kompatibilita | 90% | **100%** |
| Boot time | ~125ms | ~3-5s |
| Privileged containers | ⚠️ | ✅ |
| Nested virtualization | ❌ | ✅ |
| "Vyzerá ako železo" | nie vždy | **áno** |
| Snapshots | obmedzené | **plné (qcow2)** |

Pre use case "Claude Code v yolo mode s plným Docker" potrebujeme 100% kompatibilitu.

## Štruktúra projektu

```
cage/
├── cmd/cage/              # CLI entry point
│   └── main.go
├── internal/
│   ├── config/            # YAML config loading
│   ├── libvirt/           # libvirt/QEMU wrapper
│   ├── network/           # iptables chains, bridge setup
│   ├── virtiofs/          # virtiofsd management (hardened)
│   ├── images/            # image management (qcow2)
│   └── ssh/               # SSH key generation, cloud-init
├── scripts/
│   ├── download-image.sh  # stiahne cloud image
│   └── prepare-image.sh   # pripraví qcow2 (Docker, SSH, ...)
├── configs/
│   └── default.yaml       # default config
└── systemd/
    └── claude-cage.service
```

## Architektúra

```
┌─────────────────────────────────────────────────────────────────┐
│                           HOST                                  │
│                                                                 │
│  Terminal                                                       │
│  └── cage ssh myproject                                         │
│             │                                                   │
│  ┌──────────┼───────────────────────────────────────────────┐  │
│  │          │              libvirt + QEMU/KVM                │  │
│  │          ▼                                                │  │
│  │  ┌─────────────────────────────────────────────────────┐ │  │
│  │  │              VM (Ubuntu/Debian/...)                  │ │  │
│  │  │              - vyzerá ako fyzický stroj              │ │  │
│  │  │              - 100% Docker kompatibilita             │ │  │
│  │  │                                                      │ │  │
│  │  │   ┌─────────────────────────────────────────────┐   │ │  │
│  │  │   │           Claude Code (yolo mode)            │   │ │  │
│  │  │   │                                              │   │ │  │
│  │  │   │   - Spúšťa príkazy                          │   │ │  │
│  │  │   │   - Používa Docker (plná funkčnosť)         │   │ │  │
│  │  │   │   - Edituje /workspace                      │   │ │  │
│  │  │   └─────────────────────────────────────────────┘   │ │  │
│  │  │                       │                              │ │  │
│  │  │                       ▼                              │ │  │
│  │  │   ┌─────────────────────────────────────────────┐   │ │  │
│  │  │   │         Docker daemon (plný)                 │   │ │  │
│  │  │   │         - privileged containers ✓            │   │ │  │
│  │  │   │         - všetky storage drivers ✓           │   │ │  │
│  │  │   └─────────────────────────────────────────────┘   │ │  │
│  │  │                                                      │ │  │
│  │  │   /workspace ←─── virtio-fs ───→ ~/projects/myapp   │ │  │
│  │  └─────────────────────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    iptables/nftables                     │   │
│  │   ✓ internet    ✗ tun0 (OpenVPN)    ✗ tailscale0        │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Komponenty

### Virtualizácia
| Vlastnosť | Hodnota |
|-----------|---------|
| Technológia | QEMU/KVM cez libvirt |
| OS | Ubuntu/Debian/Fedora/Alpine (voliteľné) |
| Image format | qcow2 (copy-on-write, snapshots) |
| Kernel | plný Linux kernel (ako na železe) |
| Persistence | ephemeral (čistá pri každom štarte) |
| Boot time | ~3-5 sekúnd |

### Docker
| Vlastnosť | Hodnota |
|-----------|---------|
| Typ | Docker-in-Docker (vo VM) |
| Prístup | Lokálny socket (žiadne TLS) |
| Použitie | Priamo z VM cez `docker` príkaz |

```bash
# Použitie - všetko vo VM
cage ssh myproject
docker ps
docker run -it ubuntu bash
```

### Images

| Typ | Popis |
|-----|-------|
| Base | Oficiálne cloud images (ubuntu-24.04, debian-12, ...) |
| Custom | Používateľom vytvorené cez `cage image save` |

```bash
# Setup - stiahne a pripraví base image
cage setup --base ubuntu-24.04

# Custom image
cage start --name temp
cage ssh temp
# ... inštaluj čo treba ...
exit
cage image save temp --name my-dev-env
```

### Sieť
| Vlastnosť | Hodnota |
|-----------|---------|
| Blokovať | VPN interfaces, privátne subnety (konfigurovateľné) |
| Povoliť | verejný internet |
| DNS | `1.1.1.1`, `8.8.8.8` (vynútené cez DNAT) |
| Port forwarding | default bind na `127.0.0.1` |
| Implementácia | dedikovaný iptables chain `CAGE-FILTER` |

**Iptables pravidlá:**
```bash
# Vytvoriť chain pre cage filtering
iptables -N CAGE-FILTER

# Default policy - DROP všetko, povoliť explicitne
iptables -A CAGE-FILTER -m state --state ESTABLISHED,RELATED -j ACCEPT

# Blokovať VPN interfaces
iptables -A CAGE-FILTER -o tun+ -j DROP
iptables -A CAGE-FILTER -o tailscale+ -j DROP
iptables -A CAGE-FILTER -o wg+ -j DROP

# Blokovať privátne subnety (RFC 1918)
iptables -A CAGE-FILTER -d 10.0.0.0/8 -j DROP
iptables -A CAGE-FILTER -d 172.16.0.0/12 -j DROP
iptables -A CAGE-FILTER -d 192.168.0.0/16 -j DROP

# Blokovať link-local a metadata
iptables -A CAGE-FILTER -d 169.254.0.0/16 -j DROP

# Povoliť DNS (len na povolené servery)
iptables -A CAGE-FILTER -p udp --dport 53 -d 1.1.1.1 -j ACCEPT
iptables -A CAGE-FILTER -p udp --dport 53 -d 8.8.8.8 -j ACCEPT
iptables -A CAGE-FILTER -p udp --dport 53 -j DROP

# Povoliť HTTP/HTTPS na verejný internet
iptables -A CAGE-FILTER -p tcp --dport 80 -j ACCEPT
iptables -A CAGE-FILTER -p tcp --dport 443 -j ACCEPT

# Povoliť ostatné porty na verejný internet (po blokovaní privátnych)
iptables -A CAGE-FILTER -j ACCEPT

# Aplikovať chain na cage bridge interface
iptables -A FORWARD -i cage-br0 -j CAGE-FILTER
```

**DNS enforcement (DNAT):**
```bash
# Vynútiť použitie verejného DNS
iptables -t nat -A PREROUTING -i cage-br0 -p udp --dport 53 \
  -j DNAT --to-destination 1.1.1.1:53
```

### Bezpečnosť

**Libvirt sandboxing:**
- QEMU beží ako neprivilegovaný user (`qemu:qemu`)
- SELinux/AppArmor profily (sVirt) - každá VM má unikátny label
- seccomp profil pre QEMU proces

**Cgroups limity (explicitné):**
```xml
<!-- libvirt domain XML -->
<cputune>
  <shares>1024</shares>
  <period>100000</period>
  <quota>400000</quota>  <!-- max 4 CPU -->
</cputune>
<memtune>
  <hard_limit unit='MiB'>4096</hard_limit>
  <soft_limit unit='MiB'>4096</soft_limit>
</memtune>
<blkiotune>
  <weight>500</weight>
</blkiotune>
```

| Limit | Default | Heavy | Light |
|-------|---------|-------|-------|
| CPU | 4 cores | 8 cores | 2 cores |
| RAM | 4 GB hard | 8 GB | 2 GB |
| I/O weight | 500 | 750 | 250 |
| Max processes | 4096 | 8192 | 2048 |

**Sieťová izolácia:**
- VM nemá prístup k VPN interfaces (tun+, tailscale+, wg+)
- Blokované privátne subnety (RFC 1918)
- DNS vynútený na verejné servery

**Filesystem izolácia:**
- Len /workspace je zdieľaný (hardened virtiofsd)
- Host filesystem nie je viditeľný
- VM nemôže zapisovať mimo workspace

**Izolácia medzi cages:**
- Každý cage má vlastnú izolovanú sieť
- Žiadna komunikácia medzi cages (default)

Claude Code je kompletne izolovaný vo VM.

### Zdieľanie súborov
| Vlastnosť | Hodnota |
|-----------|---------|
| Technológia | virtio-fs |
| Daemon | virtiofsd na hoste |
| Latencia | real-time (obojsmerné) |

**Virtiofsd hardening:**
```bash
virtiofsd \
  --socket-path=/run/cage-myproject/virtiofs.sock \
  --shared-dir=/home/user/projects/myproject \
  --sandbox chroot \        # chroot izolácia
  --seccomp=kill \          # seccomp filter
  --cache=auto
```

**Bezpečnostné opatrenia:**
- `--sandbox chroot` - virtiofsd beží v chroot
- `--seccomp=kill` - blokuje nebezpečné syscally
- Symlinks sú resolvované len v rámci shared-dir
- VM nemôže zapisovať mimo zdieľaný adresár

**⚠️ Workspace hygiene varovanie:**
Súbory v `/workspace` sú zdieľané s hostom a PREŽIJÚ reštart VM. Pred použitím na hoste skontroluj:
- Git hooks (`.git/hooks/`)
- Build skripty (`Makefile`, `package.json` scripts)
- Dotfiles (`.bashrc`, `.profile`)

### SSH

**Key management:**
| Vlastnosť | Hodnota |
|-----------|---------|
| Algoritmus | Ed25519 (moderný, bezpečný) |
| Scope | Per-cage (každý cage má vlastný keypair) |
| Uloženie | `~/.claude-cage/keys/<cage-name>/` |
| Injekcia | cloud-init pri štarte VM |

**Generovanie kľúčov:**
```bash
# Pri `cage start --name myproject`:
# 1. Vygeneruje Ed25519 keypair
ssh-keygen -t ed25519 -f ~/.claude-cage/keys/myproject/id_ed25519 -N ""

# 2. Vytvorí cloud-init user-data
#cloud-config
users:
  - name: cage
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-ed25519 AAAA... cage@myproject

# 3. Pripojí cloud-init ISO k VM
```

**SSH konfigurácia vo VM:**
```
# /etc/ssh/sshd_config
PasswordAuthentication no
PubkeyAuthentication yes
PermitRootLogin no
```

**Known hosts:**
- Každý cage má unikátny host key
- `cage ssh` automaticky pridá/aktualizuje known_hosts
- `~/.claude-cage/known_hosts` (separátny od `~/.ssh/known_hosts`)

## Konfigurácia

```yaml
# ~/.claude-cage/config.yaml
images:
  default: ubuntu-24.04

profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    io_weight: 500
    max_processes: 4096
  heavy:
    vcpu: 8
    memory_mb: 8192
    io_weight: 750
    max_processes: 8192
  light:
    vcpu: 2
    memory_mb: 2048
    io_weight: 250
    max_processes: 2048

network:
  # VPN interfaces (wildcard supported)
  blocked_interfaces:
    - tun+
    - tailscale+
    - wg+

  # Privátne subnety (RFC 1918 + link-local)
  blocked_subnets:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 169.254.0.0/16

  # Vynútené DNS servery
  dns:
    - 1.1.1.1
    - 8.8.8.8

  # Port forwarding binding
  port_bind: 127.0.0.1   # alebo 0.0.0.0 pre externý prístup

shares:
  - host: ~/projects
    guest: /workspace
    # mode: rw          # rw (default) alebo ro

security:
  max_cages: 10          # maximálny počet súčasne bežiacich cages
  virtiofsd_sandbox: true  # --sandbox chroot
```

## CLI

```bash
# Setup
cage setup                        # interaktívny výber base image
cage setup --base debian-12       # konkrétny base image

# Spustenie
cage start --name myproject                    # default image a profil
cage start --name dev --base alpine-3.19       # konkrétny base
cage start --name ml --image my-custom-image   # custom image
cage start --name heavy --profile heavy        # viac resources

# Správa
cage list                         # zobrazí bežiace cage
cage stop myproject               # zastaví cage
cage stop --all                   # zastaví všetky
cage restart myproject            # reštartuje cage (stop + start)

# Prístup
cage ssh myproject                # SSH do cage
cage ssh myproject "docker ps"    # spustiť príkaz
cage exec myproject -- docker ps  # exec bez alokácie TTY (rýchlejšie)

# Status
cage status myproject             # stav cage
cage status myproject --watch     # real-time monitoring
cage logs myproject               # aktivita vo VM
cage logs myproject -f            # follow mode

# Port forwarding
cage start --name api --port 8080:80 --port 5432:5432
cage port api add 3000:3000       # pridať za behu
cage port api list                # zobraziť mapované porty
cage port api remove 3000         # odstrániť

# Images
cage image list                   # dostupné images
cage image save myproject --name my-env   # uložiť ako custom image
cage image delete old-image       # zmazať image

# Snapshots (qcow2)
cage snapshot create myproject --name before-experiment
cage snapshot list myproject
cage snapshot restore myproject --name before-experiment
cage snapshot delete myproject --name before-experiment

# Diagnostika
cage doctor                       # skontroluje požiadavky (KVM, libvirt, ...)
```

Každý cage má vlastný:
- libvirt domain (VM definícia)
- qcow2 disk (copy-on-write z base image)
- virtiofsd socket pre /workspace
- bridge interface
- SSH prístup

## Systemd (voliteľný)

```bash
systemctl --user enable claude-cage   # auto-start pri login
systemctl --user start claude-cage    # manuálny štart
```

## Požiadavky

```bash
# KVM prístup
sudo usermod -aG kvm $USER      # ✅ hotovo

# Libvirt
sudo apt install qemu-kvm libvirt-daemon-system libvirt-clients
sudo usermod -aG libvirt $USER

# Virtiofsd (pre zdieľanie súborov)
sudo apt install virtiofsd
```
