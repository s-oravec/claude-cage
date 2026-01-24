# Scenario: Custom Images

Preparing pre-configured environments for quick start and team sharing.

## Problem

- I don't want to install the same packages every time I start
- The team needs a consistent environment
- I want quick start without waiting for setup

## Creating a Custom Image

```bash
# 1. Create a temporary directory and initialize a cage
mkdir /tmp/image-setup && cd /tmp/image-setup
cage init --image ubuntu-24.04 --cage setup
cage start

# 2. SSH and install stack
cage ssh

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
cage stop
cage remove
cd ~ && rm -rf /tmp/image-setup
```

## Using Custom Image

```bash
# In your project directory
cd ~/projects/myapp

# Initialize with custom image
cage init --image mystack

# Start the cage (port forwarding can be in config or command line)
cage start --port 3000:3000

# Connect and verify
cage ssh
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
cd ~/projects/feature-x
cage init --image team-backend-v1
cage start
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
