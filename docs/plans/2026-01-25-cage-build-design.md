# Design: `cage build` Command

## Overview

Declarative image building using `Cagefile` with Dockerfile syntax. Enables sharing configurations across team members and CI/CD automation.

## CLI Interface

```bash
cage build -t <image-name> <context>
```

### Required Arguments

- `-t <name>` - name of the resulting image
- `<context>` - directory for COPY operations (build context)

### Optional Arguments

- `-f <path>` - path to Cagefile (default: `<context>/Cagefile`)
- `--build-arg KEY=VALUE` - build arguments (repeatable)
- `--keep-on-error` - keep temporary cage on failure for debugging

## Cagefile Syntax

Dockerfile-compatible syntax with the following instructions:

| Instruction | Description |
|-------------|-------------|
| `FROM <base-image>` | Base image (required, must be first) |
| `ARG <name>[=<default>]` | Build-time argument |
| `ENV <key>=<value>` | Environment variable |
| `WORKDIR <path>` | Working directory |
| `COPY <src> <dest>` | Copy files from build context |
| `RUN <command>` | Execute shell command |

### Example Cagefile

```dockerfile
FROM ubuntu:22.04

ARG NODE_VERSION=18

ENV NODE_ENV=development
ENV PATH=/usr/local/bin:$PATH

WORKDIR /app

RUN apt-get update && apt-get install -y curl git
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash -
RUN apt-get install -y nodejs

COPY ./package.json /app/
RUN npm install

COPY ./src /app/src
```

## Build Process

1. **Parse Cagefile** - load and validate syntax
2. **Create temporary cage** - `cage-build-<random>` from base image (FROM)
3. **Start cage** - wait for SSH availability (with retry)
4. **Execute instructions** - sequentially, top to bottom:
   - `ARG` - store in build context
   - `ENV` - `export` in shell session
   - `WORKDIR` - `mkdir -p && cd`
   - `COPY` - transfer files (virtiofs or SCP)
   - `RUN` - execute via SSH, stream output
5. **Save image** - `cage image save -n <tag>`
6. **Cleanup** - stop and remove temporary cage

### Build Output

```
Step 1/6 : FROM ubuntu:22.04
 ---> Using base image ubuntu:22.04
Step 2/6 : ARG NODE_VERSION=18
 ---> Build arg NODE_VERSION=18
Step 3/6 : RUN apt-get update
 ---> Running in cage-build-a1b2c3
Get:1 http://archive.ubuntu.com/ubuntu jammy InRelease
...
Step 4/6 : COPY ./src /app/src
 ---> Copying 15 files
Step 5/6 : RUN npm install
 ---> Running in cage-build-a1b2c3
...
Successfully built image: my-dev-env
```

## COPY Mechanism

**Strategy:** Use virtiofs if available, fallback to SCP.

### Detection at build start

1. Check if virtiofs is available (virtiofsd binary + permissions)
2. If yes - mount build context into cage as `/mnt/build-context`
3. If no - use SCP for each COPY

### Supported syntax

```dockerfile
COPY ./src /app/src           # directory
COPY ./config.json /app/      # file
COPY ./scripts/* /app/bin/    # glob pattern
```

### Constraints

- `COPY` only from build context (no absolute host paths)
- Destination must be absolute path in cage
- Non-existent source → error

### WORKDIR interaction

```dockerfile
WORKDIR /app
COPY ./src ./src    # -> /app/src (relative to WORKDIR)
```

## Error Handling

| Situation | Action |
|-----------|--------|
| Invalid Cagefile | Error + exit before starting cage |
| FROM image doesn't exist | Error + exit before starting cage |
| SSH timeout | Error, cleanup cage (unless `--keep-on-error`) |
| RUN command fails | Error, show exit code + stderr, cleanup |
| COPY src doesn't exist | Error, cleanup |
| Disk full | Error from cage, cleanup |

## Integration with Existing Commands

```bash
# After successful build
cage build -t my-env .

# Use in .claude-cage.yml
# image: my-env

# Or directly
cage start my-cage --image my-env

# Image management
cage image list        # shows built images
cage image remove my-env
cage image inspect my-env
```

Built images are saved to `~/.claude-cage/images/` as custom images - same as `cage image save`.

## Design Decisions

1. **Dockerfile syntax over YAML** - familiar to users, preserves instruction order naturally
2. **No HW config in Cagefile** - hardware (CPU, RAM, disk) stays in `.claude-cage.yml` for runtime configuration
3. **No EXPOSE instruction** - port forwarding is runtime config
4. **Temporary cage approach** - boot cage, execute via SSH, save image (vs. libguestfs direct modification)
5. **No layer caching initially** - YAGNI, can add later if needed
6. **Explicit build context** - same as Docker, allows building from different directories

## Usage Examples

### CI/CD (GitHub Actions)

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build cage image
        run: |
          cage build -t ci-image --build-arg NODE_VERSION=20 .
      - name: Run tests
        run: |
          cage start ci-test --image ci-image
          cage exec ci-test npm test
          cage remove ci-test
```

### Team sharing

```bash
# Colleague with same repo
git clone repo
cage build -t dev-env .
cage start dev --image dev-env
```
