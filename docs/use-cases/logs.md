# cage logs

Zobrazí aktivitu vo VM.

## Použitie

```bash
cage logs <name> [flags]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage | áno |

## Flags

| Flag | Skratka | Popis |
|------|---------|-------|
| `--follow` | `-f` | Sledovať nové záznamy |
| `--tail` | `-n` | Počet posledných záznamov (default: 100) |
| `--since` | | Od času (napr. "1h", "2024-01-01") |

## Príklady

```bash
# Posledných 100 záznamov
cage logs backend

# Sledovať live
cage logs backend -f

# Posledných 20
cage logs backend -n 20

# Od poslednej hodiny
cage logs backend --since 1h
```

## Výstup

```
2024-01-23 14:30:15 [boot] VM started
2024-01-23 14:30:16 [systemd] Docker service started
2024-01-23 14:30:20 [ssh] Connection from host
2024-01-23 14:35:22 [docker] Container started: nginx
2024-01-23 14:35:30 [docker] Container started: postgres
2024-01-23 14:40:00 [docker] Container stopped: nginx
```

## Čo sa loguje

| Event | Popis |
|-------|-------|
| `boot` | VM štart/stop |
| `systemd` | Služby (docker, ssh, ...) |
| `ssh` | SSH pripojenia |
| `docker` | Container lifecycle |
| `network` | Sieťová aktivita |

## Umiestnenie logov

Logy sú uložené na hoste:
```
~/.claude-cage/logs/<name>/vm.log
```

## Poznámka

Pre detailné Docker logy použi SSH:
```bash
cage ssh backend
docker logs mycontainer
docker compose logs -f
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
| `no logs available` | Cage ešte nebol spustený | Spustiť `cage start` |
