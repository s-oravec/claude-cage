# Scenár: Custom Images

Príprava predkonfigurovaných prostredí pre rýchly štart a zdieľanie v tíme.

## Problém

- Nechcem pri každom štarte inštalovať rovnaké balíky
- Tím potrebuje konzistentné prostredie
- Chcem rýchly štart bez čakania na setup

## Vytvorenie custom image

```bash
# 1. Spustiť cage s base image
cage start --name setup

# 2. SSH a nainštalovať stack
cage ssh setup

sudo apt update && sudo apt install -y \
    nodejs npm \
    python3 python3-pip \
    postgresql-client

sudo npm install -g yarn typescript
pip3 install numpy pandas

exit

# 3. Uložiť ako custom image
cage image save setup --name mystack --description "Node.js + Python stack"

# 4. Cleanup
cage stop setup
```

## Použitie custom image

```bash
# Okamžitý štart s predpripravenými nástrojmi
cage start --name dev --image mystack --port 3000:3000

cage ssh dev
node --version    # funguje
python3 --version # funguje
```

## Team workflow

```bash
# Lead vytvorí image
cage image save setup --name team-backend-v1

# Export pre tím
cp ~/.claude-cage/images/team-backend-v1.qcow2 /shared/team/

# Členovia importujú
cp /shared/team/team-backend-v1.qcow2 ~/.claude-cage/images/

# Všetci používajú rovnaké prostredie
cage start --name feature-x --image team-backend-v1
```

## Správa images

```bash
# Zoznam
cage image list

# NAME              TYPE    SIZE     CREATED
# ubuntu-24.04      base    285 MB   2024-01-20
# mystack           custom  450 MB   2024-01-23
# team-backend-v1   custom  520 MB   2024-01-23

# Detaily
cage image inspect mystack

# Zmazať
cage image delete old-image
```
