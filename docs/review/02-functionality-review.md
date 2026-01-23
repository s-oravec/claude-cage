# Claude Cage - Functionality Review

## Zhrnutie

Claude Cage predstavuje dobre navrhnuty CLI nastroj pre spravu izolovaných VM prostredí. Dizajn pokrýva väčšinu základných use casov pre vývoj s Claude Code v bezpečnom sandboxe. Celkovo je návrh solídny, ale existujú oblasti kde by bolo vhodné doplniť funkcionalitu pre produkčné nasadenie.

**Hodnotenie: 7.5/10** - Dobry základ, potrebuje doplnenie niektorých enterprise a DevOps funkcií.

---

## Silne stranky

### 1. Kompletne CLI pokrytie zákalných operácií

CLI poskytuje všetky základné príkazy pre lifecycle management:

| Kategória | Príkazy | Stav |
|-----------|---------|------|
| Inicializácia | `setup` | Kompletné |
| Lifecycle | `start`, `stop`, `list`, `status` | Kompletné |
| Prístup | `ssh` | Kompletné |
| Monitoring | `logs`, `status --watch` | Kompletné |
| Networking | `port add/remove/list` | Kompletné |
| Images | `image list/save/delete/inspect` | Kompletné |
| Konfigurácia | `config show/edit/path/init` | Kompletné |

### 2. Intuitivny workflow

Workflow je logicky a priamociary:
```bash
cage setup --base ubuntu-24.04  # jednorazovo
cage start --name projekt       # vytvor cage
cage ssh projekt                # pracuj
cage stop projekt               # ukonci
```

### 3. Flexibilne resource profily

Systém profilov (`default`, `heavy`, `light`) s možnosťou override cez `--memory` a `--cpus` je praktický a pokrýva väčšinu scenárov.

### 4. JSON výstup pre automatizáciu

Väčšina príkazov podporuje `--json` flag, čo umožňuje integráciu so skriptami a CI/CD.

### 5. Ephemeral by default

Bezpečný default - VM sa zmaže po `stop`, data v `/workspace` zostávajú na hoste cez virtio-fs.

### 6. Multi-cage podpora

Dizajn podporuje viacero súčasne bežiacich cage s:
- Unikátnymi názvami
- Separátnymi bridge interfaces
- Nezávislým port mappingom
- Izolovanými TLS certifikátmi

### 7. Custom images

Možnosť uložiť nakonfigurovaný cage ako reusable image (`cage image save`) je veľmi praktická pre tímové prostredia.

---

## Chýbajúca funkcionalita

### Kritické (P1)

#### 1. Chýba `cage restart`

**Problem:** Nie je možné reštartovať cage bez straty stavu.

**Scenár:** VM sa zasekne, používateľ chce soft restart.

**Odporúčanie:**
```bash
cage restart <name> [--force]
```

#### 2. Chýba `cage exec`

**Problem:** `cage ssh` vyžaduje plné SSH spojenie aj pre jednoduché príkazy.

**Scenár:** Scripting, rýchle príkazy.

**Odporúčanie:**
```bash
cage exec <name> <command>  # rýchlejšie ako ssh, direct exec
```

#### 3. Chýba snapshot management

**Problem:** Qcow2 podporuje snapshots, ale CLI ich nevyužíva.

**Scenár:** Používateľ chce uložiť stav pred experimentom.

**Odporúčanie:**
```bash
cage snapshot create <name> <snapshot-name>
cage snapshot list <name>
cage snapshot restore <name> <snapshot-name>
cage snapshot delete <name> <snapshot-name>
```

#### 4. Chýba image import/export

**Problem:** Custom images sa nedajú ľahko zdieľať (len manuálne kopírovanie).

**Odporúčanie:**
```bash
cage image export <name> --output my-image.tar.gz
cage image import my-image.tar.gz --name imported-env
```

### Dôležité (P2)

#### 5. Chýba `cage pause/resume`

**Problem:** Nie je možné pozastaviť VM bez zastavenia.

**Scenár:** Šetrenie resources bez straty stavu.

**Odporúčanie:**
```bash
cage pause <name>
cage resume <name>
```

#### 6. Chýba resource resize za behu

**Problem:** Zmena CPU/RAM vyžaduje restart.

**Odporúčanie:**
```bash
cage resize <name> --memory 8192 --cpus 4
```
(Libvirt/QEMU podporuje hot-add CPU/RAM)

#### 7. Chýba zdravotný check

**Problem:** `cage status` neukazuje health VM.

**Odporúčanie:**
```bash
cage status <name>
# ... existujúci výstup ...
# Health: healthy/unhealthy/unknown
# Last heartbeat: 5s ago
```

#### 8. Chýba prístup k Docker z hosta

**Problem:** V dizajne je zmienený TLS Docker prístup, ale chýba dokumentácia.

**Odporúčanie:** Pridať do `start` output a dokumentácie:
```bash
# V cage status:
Docker (from host):
  DOCKER_HOST=tcp://10.0.0.2:2376
  DOCKER_CERT_PATH=~/.claude-cage/certs/backend
```

#### 9. Chýba share management za behu

**Problem:** Shares sa dajú definovať len pri `start`.

**Odporúčanie:**
```bash
cage share <name> add ~/data:/data
cage share <name> remove /data
cage share <name> list
```

### Nice-to-have (P3)

#### 10. Chýba `cage clone`

```bash
cage clone <source> <new-name>  # klon bežiaceho cage
```

#### 11. Chýba `cage attach`

```bash
cage attach <name>  # pripojí sa ku konzole VM (QEMU monitor)
```

#### 12. Chýba template system

```bash
cage template create --name nodejs-app --base ubuntu-24.04 --port 3000:3000 --profile light
cage start --template nodejs-app --name my-app
```

#### 13. Chýba environment variables

```bash
cage start --name dev --env DATABASE_URL=postgres://... --env DEBUG=true
```

#### 14. Chýba resource limits monitoring

```bash
cage start --name dev --cpu-limit 50%  # max 50% host CPU
cage start --name dev --io-limit 100mb  # max 100MB/s disk IO
```

---

## Analýza konfigurácie

### Silné stránky

- YAML formát je čitateľný a editovateľný
- Profily umožňujú predefinované konfigurácie
- Network blocking je flexibilný (interfaces, DNS)
- Shares sú konfigurovateľné

### Chýbajúce možnosti

| Položka | Popis | Priorita |
|---------|-------|----------|
| `defaults.name_prefix` | Prefix pre auto-generované názvy | P3 |
| `defaults.auto_port` | Automatický port range pre nové cage | P2 |
| `profiles.*.disk_gb` | Disk limit per profil | P2 |
| `network.allowed_domains` | Whitelist pre sieťový prístup | P2 |
| `security.readonly_shares` | Readonly zdieľané adresáre | P2 |
| `hooks.pre_start/post_stop` | Lifecycle hooks | P3 |
| `logging.level` | Log level konfigurácia | P3 |
| `images.registry` | Custom registry pre images | P3 |

### Odporúčaná rozšírená konfigurácia

```yaml
# ~/.claude-cage/config.yaml

defaults:
  base: ubuntu-24.04
  profile: default
  name_prefix: cage-
  auto_port_range: 10000-10100

profiles:
  default:
    vcpu: 4
    memory_mb: 4096
    disk_gb: 20          # CHÝBA
  heavy:
    vcpu: 8
    memory_mb: 8192
    disk_gb: 50

network:
  blocked_interfaces:
    - tun0
    - tailscale0
  allowed_domains: []    # CHÝBA - whitelist
  dns:
    - 1.1.1.1
    - 8.8.8.8

security:                # CHÝBA
  readonly_shares: []
  max_cages: 5
  max_ports_per_cage: 50

shares:
  - host: ~/projects
    guest: /workspace
    readonly: false      # CHÝBA

hooks:                   # CHÝBA
  pre_start: []
  post_stop: []

logging:                 # CHÝBA
  level: info
  max_size_mb: 100
  retention_days: 7
```

---

## Multi-cage analýza

### Funguje správne

- Každý cage má unikátny názov (enforced)
- Separátne bridge interfaces (`cage-<name>`)
- Nezávislý port mapping
- Izolované qcow2 disky
- Separátne TLS certifikáty

### Potenciálne problémy

#### 1. Port konflikty

**Problem:** Nie je automatická detekcia port konfliktov medzi cages.

**Scenár:**
```bash
cage start --name api --port 8080:80
cage start --name web --port 8080:80  # Chyba, ale nevieme prečo
```

**Odporúčanie:** Lepšia chybová správa + `cage port check 8080`

#### 2. Resource contention

**Problem:** Nie je celkový limit na resources pre všetky cages.

**Scenár:** Používateľ spustí 10 "heavy" cages a vyčerpá host RAM.

**Odporúčanie:**
```yaml
limits:
  max_total_vcpu: 16
  max_total_memory_mb: 32768
  max_cages: 5
```

#### 3. Chýba cage grouping

**Scenár:** Microservices projekt s viacerými cages.

**Odporúčanie:**
```bash
cage group create myproject
cage start --name api --group myproject
cage start --name db --group myproject
cage group stop myproject  # zastavi všetky v skupine
```

---

## Workflow analýza

### Intuitívne časti

1. **Prvé použitie:** `setup` -> `start` -> `ssh` je jasne
2. **Denné použitie:** `start` -> `ssh` -> `stop` je minimalistické
3. **Custom images:** workflow je dobre zdokumentovaný

### Menej intuitívne časti

#### 1. Nejasný vzťah base vs custom image

Používateľ môže byť zmätený:
- `--base` = oficiálny cloud image
- `--image` = custom uložený cez `cage image save`

**Odporúčanie:** Zjednotiť na `--image` s prefixami:
```bash
cage start --image base:ubuntu-24.04
cage start --image custom:my-dev-env
# alebo bez prefixu - auto-detect
cage start --image ubuntu-24.04
cage start --image my-dev-env
```

#### 2. Chýba "development mode"

**Scenár:** Používateľ chce rýchly dev setup bez manuálnej konfigurácie.

**Odporúčanie:**
```bash
cage dev  # alias pre: cage start --name $(basename $PWD) --port auto
```

#### 3. Chýba onboarding wizard

**Odporúčanie:**
```bash
cage init  # interaktívny sprievodca pre nový projekt
```

---

## Edge cases a nepodporované scenáre

### 1. GPU passthrough

**Scenár:** ML/AI workloads vyžadujúce GPU.

**Status:** Nie je podporované v dizajne.

**Odporúčanie:** Pridať podporu pre VFIO GPU passthrough:
```bash
cage start --name ml --gpu nvidia0
```

### 2. Persistent cage

**Scenár:** Používateľ chce cage ktorý prežije reboot.

**Status:** Nie je podporované (ephemeral only).

**Odporúčanie:**
```bash
cage start --name db --persistent
```

### 3. Networking medzi cages

**Scenár:** Microservices komunikujúce medzi sebou.

**Status:** Nie je explicitne podporované.

**Odporúčanie:**
```bash
cage network create mynet
cage start --name api --network mynet
cage start --name db --network mynet
# api môže pristúpiť k db.mynet
```

### 4. Cloud-init customizácia

**Scenár:** Používateľ chce custom cloud-init.

**Status:** Nie je podporované.

**Odporúčanie:**
```bash
cage start --name dev --cloud-init ./my-cloud-init.yaml
```

### 5. USB passthrough

**Scenár:** Hardware development, IoT.

**Status:** Nie je podporované.

**Odporúčanie:**
```bash
cage start --name iot --usb vendor:product
```

### 6. Windows guest

**Scenár:** Windows development/testing.

**Status:** Len Linux images v dokumentácii.

**Odporúčanie:** Pridať podporu pre Windows images (QEMU to podporuje).

---

## Odporúčania

### Priorita 1 (Implementovať pred 1.0)

1. **Pridať `cage restart`** - základná operácia
2. **Pridať `cage snapshot`** - využiť qcow2 capabilities
3. **Pridať `cage image export/import`** - tímová spolupráca
4. **Zlepšiť error messages** - jasnejšie diagnostiky
5. **Pridať health check** - monitoring

### Priorita 2 (Post 1.0)

6. **Pridať `cage pause/resume`** - resource management
7. **Pridať share management za behu** - flexibilita
8. **Pridať resource limits** - bezpečnosť na multi-tenant systémoch
9. **Pridať cage networking** - microservices support
10. **Pridať lifecycle hooks** - automatizácia

### Priorita 3 (Nice-to-have)

11. **Template system** - predpripravené konfigurácie
12. **GPU passthrough** - ML workloads
13. **Persistent cages** - database scenarios
14. **Cloud-init customization** - advanced users

---

## Záver

Claude Cage dizajn je **solídny základ** pre bezpečné izolované vývojové prostredie. CLI je dobre štruktúrované a pokrýva základné operácie.

**Hlavné silné stránky:**
- Jednoduchý a intuitívny workflow
- Dobré základné CLI pokrytie
- Flexibilné profily a konfigurácia
- Multi-cage podpora

**Hlavné nedostatky:**
- Chýba snapshot management (kľúčová funkcia qcow2)
- Chýba image export/import pre tímovú spoluprácu
- Chýba networking medzi cages
- Limitované runtime modifikácie (shares, resources)

**Odporúčanie:** Implementovať P1 položky pred prvým release, najmä `snapshot` a `image export/import`, ktoré významne zvýšia hodnotu nástroja pre produkčné použitie.

---

*Review vytvorený: 2026-01-23*
*Reviewer: DevOps Expert (Claude Opus 4.5)*
