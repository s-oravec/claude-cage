# Scenár: Full Yolo Agentic Development

Claude Code beží v yolo mode vnútri izolovaného cage VM s plným prístupom k Dockeru.

## Problém

Chcem používať Claude Code v yolo mode (automatické schvaľovanie príkazov) ale:
- Nesmie mať prístup k VPN (firemná sieť, tailscale, wireguard)
- Nesmie vidieť domácu sieť (192.168.x.x)
- Nesmie mať prístup k citlivým súborom (~/.ssh, ~/.aws, ~/.config)
- Potrebujem plnú funkčnosť Dockeru (nie obmedzený Docker-in-Docker)

## Riešenie

```bash
# 1. Spustiť cage
cage start --name dev --profile heavy --port 3000:3000 --port 5432:5432

# 2. SSH do cage
cage ssh dev

# 3. Spustiť Claude Code v yolo mode
claude --dangerously-skip-permissions

# Claude teraz môže:
# - spúšťať akékoľvek príkazy
# - používať Docker/docker-compose
# - inštalovať balíky
# - všetko v bezpečnom sandboxe
```

## Architektúra

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              HOST                                        │
│                                                                          │
│  Terminal                                                                │
│  └── cage ssh dev                                                        │
│            │                                                             │
│            ▼                                                             │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                         CAGE VM (QEMU/KVM)                         │  │
│  │                                                                    │  │
│  │  ┌─────────────────────────────────────────────────────────────┐  │  │
│  │  │                   Claude Code (yolo mode)                    │  │  │
│  │  │                                                              │  │  │
│  │  │   ✓ Plný shell prístup                                      │  │  │
│  │  │   ✓ Plný Docker prístup (natívny daemon)                    │  │  │
│  │  │   ✓ Editácia súborov v /workspace                           │  │  │
│  │  │   ✓ Inštalácia balíkov (apt, npm, pip)                      │  │  │
│  │  │   ✓ Prístup na verejný internet                             │  │  │
│  │  └─────────────────────────────────────────────────────────────┘  │  │
│  │                              │                                     │  │
│  │                              ▼                                     │  │
│  │  ┌─────────────────────────────────────────────────────────────┐  │  │
│  │  │              Docker daemon (natívny vo VM)                   │  │  │
│  │  │                                                              │  │  │
│  │  │   - Plná funkčnosť (privileged, volumes, networks)          │  │  │
│  │  │   - docker-compose, docker build                            │  │  │
│  │  │   - Žiadne obmedzenia ako pri Docker-in-Docker              │  │  │
│  │  └─────────────────────────────────────────────────────────────┘  │  │
│  │                                                                    │  │
│  │  /workspace ←───── virtio-fs ─────→ ~/projects/myapp              │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  BLOKOVANÉ (iptables CAGE-FILTER chain):                                │
│  ├── tun+ (OpenVPN)                                                     │
│  ├── tailscale+ (Tailscale)                                             │
│  ├── wg+ (WireGuard)                                                    │
│  ├── 10.0.0.0/8 (RFC 1918)                                              │
│  ├── 172.16.0.0/12 (RFC 1918)                                           │
│  ├── 192.168.0.0/16 (RFC 1918)                                          │
│  └── 169.254.0.0/16 (link-local)                                        │
│                                                                          │
│  POVOLENÉ:                                                               │
│  └── Verejný internet (cez host NAT)                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

## Bezpečnostné vrstvy

| Vrstva | Ochrana | Implementácia |
|--------|---------|---------------|
| VM izolácia | Úplná separácia od hosta | QEMU/KVM s vlastným kernelom |
| Sieťová izolácia | Blokáda VPN a interných sietí | iptables CAGE-FILTER chain |
| Filesystem izolácia | Len /workspace je zdieľaný | virtiofsd s --sandbox chroot |
| Resource limity | Kontrola CPU/RAM/IO | cgroups v2 |
| Ephemeral prostredie | Zmeny sa nestratia len v /workspace | qcow2 copy-on-write |
| DNS enforcement | Kontrola DNS queries | DNAT na 1.1.1.1/8.8.8.8 |

## Čo Claude Code MÔŽE (vnútri cage)

- Spúšťať akékoľvek shell príkazy
- Používať Docker (build, run, compose, exec, logs...)
- Spúšťať privileged kontajnery
- Vytvárať Docker networks a volumes
- Inštalovať systémové balíky (apt, dnf)
- Inštalovať dev dependencies (npm, pip, cargo)
- Pristupovať na verejný internet (GitHub, npm, PyPI)
- Modifikovať čokoľvek v /workspace

## Čo Claude Code NEMÔŽE

- Pristúpiť k VPN sieťam (firemná sieť)
- Pristúpiť k Tailscale/WireGuard sieťam
- Vidieť domácu sieť (192.168.x.x, 10.x.x.x)
- Čítať host filesystem (okrem /workspace)
- Pristúpiť k ~/.ssh, ~/.aws, ~/.config na hoste
- Modifikovať host systém
- Komunikovať s inými VM/kontajnermi na hoste
- Pristúpiť k link-local adresám

## Typický workflow

```bash
# === RÁNO - Začiatok práce ===
cd ~/projects/myapp
cage start --name dev --profile heavy --port 3000:3000 --port 5432:5432

# SSH do cage
cage ssh dev

# Spustiť development stack
cd /workspace
docker compose up -d

# Spustiť Claude Code v yolo mode
claude --dangerously-skip-permissions

# === PRÁCA ===
# Claude môže:
# - editovať kód
# - spúšťať testy
# - reštartovať kontajnery
# - inštalovať závislosti
# - debugovať
# - všetko automaticky

# === VEČER - Koniec práce ===
exit                    # ukončiť Claude
docker compose down     # zastaviť kontajnery
exit                    # opustiť cage
cage stop dev           # zničiť VM

# Stav:
# - Zmeny v ~/projects/myapp (host) sú zachované
# - Všetko ostatné vo VM je stratené
# - Žiadne rezíduá, kontajnery, volumes
```

## Docker Compose príklad

```bash
# Na hoste: ~/projects/myapp/docker-compose.yaml
cage ssh dev
cd /workspace
docker compose up -d

# Aplikácia:
# - localhost:3000 → web app (port forwarded z cage)
# - localhost:5432 → postgres (port forwarded z cage)
```

## Monitoring a kontrola

```bash
# V inom termináli na hoste:

# Sledovať resource usage
cage status dev --watch

# Sledovať logy z VM
cage logs dev -f

# Spustiť príkaz bez SSH session
cage exec dev -- docker ps
cage exec dev -- ps aux

# Ak niečo podozrivé - okamžité zastavenie
cage stop dev --force
```

## Snapshot pre experimenty

```bash
# Pred riskantným experimentom
cage snapshot create dev --name before-experiment

# Claude robí experiment...
# Niečo sa pokazilo!

# Obnoviť stav
cage stop dev
cage snapshot restore dev --name before-experiment
cage start --name dev

# Späť v pôvodnom stave
```

## Fail-safe

Ak Claude Code robí niečo podozrivé:

```bash
# Okamžité zastavenie (z hosta)
cage stop dev --force

# VM je okamžite zničená
# - Claude stráca všetok stav
# - Žiadne rezíduá
# - /workspace na hoste zostáva (môžeš skontrolovať zmeny)

# Ak treba - git reset zmeny
cd ~/projects/myapp
git diff                 # skontrolovať čo sa zmenilo
git checkout .           # zahodiť zmeny ak treba
```

## Prečo QEMU/KVM a nie kontajnery

| Docker kontajner | Cage VM |
|------------------|---------|
| Zdieľaný kernel s hostom | Vlastný kernel |
| Container escape možný | VM escape extrémne ťažký |
| Privileged = full host access | Privileged = len vo VM |
| Sieťová izolácia komplikovaná | Sieťová izolácia natívna |
| Docker-in-Docker obmedzený | Docker natívny, plná funkčnosť |
