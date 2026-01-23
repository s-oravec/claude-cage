# cage snapshot

Spravuje snapshots cage VM (využíva qcow2 snapshots).

## Použitie

```bash
cage snapshot <command> <name> [flags]
```

## Príkazy

| Príkaz | Popis |
|--------|-------|
| `create` | Vytvorí snapshot |
| `list` | Zobrazí snapshots |
| `restore` | Obnoví snapshot |
| `delete` | Zmaže snapshot |

## cage snapshot create

```bash
cage snapshot create <cage-name> --name <snapshot-name>
```

Príklad:
```bash
# Pred experimentom
cage snapshot create backend --name before-experiment

# S popisom
cage snapshot create backend --name v1.0 --description "Stable state"
```

## cage snapshot list

```bash
cage snapshot list <cage-name>
```

Výstup:
```
NAME               CREATED              SIZE     DESCRIPTION
before-experiment  2024-01-23 14:30:00  256 MB   -
v1.0               2024-01-23 15:00:00  512 MB   Stable state
clean-state        2024-01-22 10:00:00  128 MB   -
```

## cage snapshot restore

```bash
cage snapshot restore <cage-name> --name <snapshot-name>
```

Príklad:
```bash
# Experiment zlyhal, obnoviť
cage snapshot restore backend --name before-experiment
```

**Poznámka:** Cage musí byť zastavený pred restore.

## cage snapshot delete

```bash
cage snapshot delete <cage-name> --name <snapshot-name>
```

## Workflow

```bash
# 1. Spustiť cage
cage start --name dev

# 2. Nakonfigurovať prostredie
cage ssh dev
# ... inštaluj závislosti ...
exit

# 3. Vytvoriť snapshot
cage snapshot create dev --name configured

# 4. Experimentovať
cage ssh dev
# ... riskantné operácie ...
# Niečo sa pokazilo!
exit

# 5. Obnoviť
cage stop dev
cage snapshot restore dev --name configured
cage start --name dev
# Späť v pôvodnom stave
```

## Výhody oproti `cage image save`

| `cage image save` | `cage snapshot` |
|-------------------|-----------------|
| Nový image | In-place |
| Pomalšie | Rýchlejšie |
| Zdieľateľné | Len lokálne |
| Nezávislé od cage | Viazané na cage |

## Technická implementácia

Používa QEMU qcow2 internal snapshots:
```bash
virsh snapshot-create-as domain snapshot-name
virsh snapshot-revert domain snapshot-name
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
| `snapshot not found` | Snapshot neexistuje | Skontrolovať `cage snapshot list` |
| `cage must be stopped` | Cage beží pri restore | Najprv `cage stop` |
| `insufficient disk space` | Málo miesta pre snapshot | Uvoľniť miesto na disku |
