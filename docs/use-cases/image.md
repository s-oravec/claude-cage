# cage image

Spravuje cage images.

## Použitie

```bash
cage image <command> [args...]
```

## Príkazy

| Príkaz | Popis |
|--------|-------|
| `list` | Zobrazí dostupné images |
| `save` | Uloží bežiaci cage ako nový image |
| `delete` | Zmaže image |
| `inspect` | Zobrazí detaily image |

## cage image list

```bash
cage image list
```

Výstup:
```
NAME              TYPE    SIZE     CREATED
ubuntu-24.04      base    285 MB   2024-01-20
debian-12         base    250 MB   2024-01-20
nodejs-dev        custom  320 MB   2024-01-22
python-ml         custom  1.2 GB   2024-01-23
```

## cage image save

Uloží bežiaci cage ako nový image.

```bash
cage image save <cage-name> --name <image-name>
```

Príklad:
```bash
# 1. Spustiť cage a customizovať
cage start --name temp

cage ssh temp
sudo apt install nodejs npm python3 pip
pip install numpy pandas
exit

# 2. Uložiť ako image
cage image save temp --name data-science

# 3. Zmazať temporary cage
cage stop temp

# 4. Použiť nový image
cage start --name project --image data-science
```

Flags:
| Flag | Popis |
|------|-------|
| `--name` | Názov nového image (povinný) |
| `--description` | Popis image |

## cage image delete

```bash
cage image delete <image-name>
```

Príklad:
```bash
cage image delete old-image

# Nedovolí zmazať base images
cage image delete ubuntu-24.04
# Error: cannot delete base image, use --force
```

## cage image inspect

```bash
cage image inspect <image-name>
```

Výstup:
```
Name:        data-science
Type:        custom
Base:        ubuntu-24.04
Size:        1.2 GB
Created:     2024-01-23 14:30:00
Description: Python + Node.js pre data science

Installed packages:
  - nodejs 20.x
  - python3 3.12
  - numpy 1.26
  - pandas 2.1
```

## Umiestnenie images

```
~/.claude-cage/images/
├── ubuntu-24.04.qcow2      # base
├── debian-12.qcow2         # base
├── data-science.qcow2      # custom
└── nodejs-dev.qcow2        # custom
```

## Tips

```bash
# Vytvoriť image pre konkrétny projekt
cage start --name setup
cage ssh setup
# ... inštaluj závislosti ...
exit
cage image save setup --name myproject-base
cage stop setup

# Zdieľať s tímom (export)
cp ~/.claude-cage/images/myproject-base.qcow2 /shared/
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `image not found` | Image neexistuje | Skontrolovať `cage image list` |
| `cage not running` | Cage pre save nebeží | Spustiť `cage start` |
| `name already exists` | Image s názvom už existuje | Použiť iný názov |
| `cannot delete base image` | Pokus o zmazanie base image | Použiť `--force` ak naozaj treba |
| `insufficient disk space` | Málo miesta pre uloženie | Uvoľniť miesto na disku |
