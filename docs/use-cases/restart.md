# cage restart

Reštartuje bežiaci cage (stop + start).

## Použitie

```bash
cage restart <name> [flags]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage | áno |

## Flags

| Flag | Popis |
|------|-------|
| `--force` | Force stop (SIGKILL) |
| `--timeout` | Timeout pre graceful shutdown (default: 30s) |

## Príklady

```bash
# Základný reštart
cage restart backend

# Force reštart (ak nereaguje)
cage restart backend --force

# S custom timeout
cage restart backend --timeout 60s
```

## Správanie

1. Zastaví cage (`cage stop`)
2. Spustí cage s rovnakými parametrami (`cage start`)
3. Zachová:
   - Meno
   - Image
   - Profil
   - Port mappings
   - Shares

## Kedy použiť

- VM je v nekonzistentnom stave
- Po zmene konfigurácie (niektoré zmeny vyžadujú reštart)
- Pre "čistý štart" bez straty konfigurácie

## Poznámka

Reštart **zničí** všetky zmeny vo VM (ephemeral). Súbory v `/workspace` zostávajú.

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
| `timeout waiting for shutdown` | VM nereaguje | Použiť `--force` |
