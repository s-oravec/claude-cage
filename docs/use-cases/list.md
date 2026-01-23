# cage list

Zobrazí všetky bežiace cage.

## Použitie

```bash
cage list [flags]
```

## Flags

| Flag | Skratka | Popis |
|------|---------|-------|
| `--quiet` | `-q` | Len názvy (pre scripting) |
| `--json` | | JSON výstup |
| `--all` | `-a` | Vrátane zastavených (ak existujú kľúče) |

## Príklady

```bash
# Základný list
cage list

# Len názvy
cage list -q

# JSON pre scripting
cage list --json

# Vrátane historických
cage list --all
```

## Výstup

```
NAME       STATUS    PROFILE  PORTS              VCPU  MEM     UPTIME
backend    running   heavy    8080:80,5432:5432  8     8.0G    2h 15m
frontend   running   light    3000:3000          2     2.0G    1h 30m
test       running   default  -                  4     4.0G    5m
```

## JSON výstup

```json
[
  {
    "name": "backend",
    "status": "running",
    "profile": "heavy",
    "ports": [
      {"host": 8080, "guest": 80},
      {"host": 5432, "guest": 5432}
    ],
    "resources": {
      "vcpu": 8,
      "memory_mb": 8192,
      "cpu_usage_percent": 25,
      "memory_usage_mb": 2100
    },
    "uptime_seconds": 8100,
    "shares": [
      {"host": "/home/user/projects/backend", "guest": "/workspace"}
    ]
  }
]
```

## Quiet výstup

```
backend
frontend
test
```

Užitočné pre scripting:
```bash
# Zastaviť všetky cage s "test" v názve
cage list -q | grep test | xargs -I {} cage stop {}
```
