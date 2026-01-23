# cage stop

Zastaví bežiacu cage VM.

## Použitie

```bash
cage stop <name> [flags]
cage stop --all [flags]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage na zastavenie | áno (ak nie `--all`) |

## Flags

| Flag | Popis |
|------|-------|
| `--all` | Zastaví všetky bežiace cage |
| `--force` | Okamžité zastavenie (SIGKILL) |
| `--timeout` | Timeout pre graceful shutdown (default: 30s) |

## Príklady

```bash
# Zastaviť konkrétny cage
cage stop backend

# Zastaviť všetky
cage stop --all

# Force stop (ak nereaguje)
cage stop backend --force

# S custom timeout
cage stop backend --timeout 60s
```

## Správanie

1. Pošle SIGTERM do VM
2. Počká na graceful shutdown (Docker kontajnery sa zastavia)
3. Zastaví virtiofsd
4. Odstráni bridge interface
5. Vyčistí iptables pravidlá
6. Odstráni runtime súbory

## Výstup

```
✓ Stopping cage "backend"...
  ✓ Docker containers stopped
  ✓ VM shutdown complete
  ✓ Network cleaned up
✓ Cage "backend" stopped
```

## Ephemeral cleanup

Po `cage stop`:
- VM disk je zničený
- Docker images/containers sú stratené
- Sieťová konfigurácia odstránená
- `/workspace` súbory zostávajú (sú na hoste)
- SSH kľúče zostávajú (pre opätovné použitie)

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
| `timeout waiting for shutdown` | VM nereaguje | Použiť `--force` |
