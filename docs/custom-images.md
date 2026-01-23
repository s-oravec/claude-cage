# Scenario: Custom Images

Preparing pre-configured environments for quick start and team sharing.

## Problem

- I don't want to install the same packages every time I start
- The team needs a consistent environment
- I want quick start without waiting for setup

## Creating a Custom Image

```bash
# 1. Start cage with base image
cage create -n setup --ssh auto
cage start setup

# 2. SSH and install stack
cage ssh setup

sudo apt update && sudo apt install -y \
    nodejs npm \
    python3 python3-pip \
    postgresql-client

sudo npm install -g yarn typescript
pip3 install numpy pandas

exit

# 3. Save as custom image
cage image save setup --name mystack --description "Node.js + Python stack"

# 4. Cleanup
cage stop setup
cage remove setup
```

## Using Custom Image

```bash
# Instant start with pre-prepared tools
cage create -n dev -i mystack --ssh auto
cage start dev --port 3000:3000

cage ssh dev
node --version    # works
python3 --version # works
```

## Team Workflow

```bash
# Lead creates image
cage image save setup --name team-backend-v1

# Export for team
cp ~/.claude-cage/images/team-backend-v1.qcow2 /shared/team/

# Team members import
cp /shared/team/team-backend-v1.qcow2 ~/.claude-cage/images/

# Everyone uses the same environment
cage create -n feature-x -i team-backend-v1 --ssh auto
cage start feature-x
```

## Managing Images

```bash
# List
cage image list

# NAME              TYPE    SIZE     CREATED
# ubuntu-24.04      base    285 MB   2024-01-20
# mystack           custom  450 MB   2024-01-23
# team-backend-v1   custom  520 MB   2024-01-23

# Details
cage image inspect mystack

# Delete
cage image delete old-image
```
