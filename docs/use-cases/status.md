# cage status

Zobrazí detailný stav cage.

## Použitie

```bash
cage status <name> [flags]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage | áno |

## Flags

| Flag | Popis |
|------|-------|
| `--json` | JSON výstup |
| `--watch` | Kontinuálne sledovanie (ako `top`) |

## Príklady

```bash
# Základný status
cage status backend

# JSON
cage status backend --json

# Kontinuálne sledovanie
cage status backend --watch
```

## Výstup

```
Cage: backend
Status: running
Uptime: 2h 15m 30s

Resources:
  Profile: heavy
  vCPU:    8 (usage: 25%)
  Memory:  8192 MB (usage: 2100 MB / 26%)

Network:
  IP:      10.0.0.2
  Bridge:  cage-backend
  Ports:
    - 8080 → 80
    - 5432 → 5432

Docker:
  Containers: 3 running, 1 stopped
  Images:     12

Shares:
  - /home/user/projects/backend → /workspace

Processes (top 5 by CPU):
  PID    CPU%   MEM%   COMMAND
  1234   15.2   3.1    dockerd
  2345   8.5    12.4   node
  3456   1.2    0.5    postgres
```

## JSON výstup

```json
{
  "name": "backend",
  "status": "running",
  "uptime_seconds": 8130,
  "resources": {
    "profile": "heavy",
    "vcpu": 8,
    "memory_mb": 8192,
    "cpu_usage_percent": 25,
    "memory_usage_mb": 2100
  },
  "network": {
    "ip": "10.0.0.2",
    "bridge": "cage-backend",
    "ports": [
      {"host": 8080, "guest": 80},
      {"host": 5432, "guest": 5432}
    ]
  },
  "docker": {
    "containers_running": 3,
    "containers_stopped": 1,
    "images": 12
  }
}
```

## Watch mode

```bash
cage status backend --watch
# Aktualizuje sa každú sekundu
# Ctrl+C pre ukončenie
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
