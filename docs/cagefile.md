# Cagefile Reference

A Cagefile is a text document that contains instructions for building a cage image. The syntax is compatible with Dockerfile, making it familiar to Docker users.

## Quick Example

```dockerfile
FROM ubuntu-24.04

ARG NODE_VERSION=20

ENV NODE_ENV=development
ENV PATH=/usr/local/bin:$PATH

WORKDIR /app

RUN apt-get update && \
    apt-get install -y \
    curl \
    git

RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - && \
    apt-get install -y nodejs

COPY ./package.json /app/
RUN npm install

COPY ./src /app/src
```

Build with:
```bash
cage build -t my-dev-env .
```

## Instructions

### FROM

```dockerfile
FROM <image>
```

Sets the base image. **Required** and must be the **first instruction**.

The image must be available locally (downloaded via `cage setup`).

```dockerfile
FROM ubuntu-24.04
FROM alpine-3.21
FROM debian-12
```

Use `cage setup --list` to see available images.

---

### ARG

```dockerfile
ARG <name>[=<default>]
```

Defines a build-time variable that can be used in subsequent instructions.

```dockerfile
ARG VERSION=1.0
ARG DEBUG
```

Override at build time:
```bash
cage build -t myimage --build-arg VERSION=2.0 .
```

Use in instructions with `${VAR}` or `$VAR`:
```dockerfile
ARG NODE_VERSION=20
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash -
```

---

### ENV

```dockerfile
ENV <key>=<value>
```

Sets an environment variable that persists in the built image and is available to RUN commands.

```dockerfile
ENV NODE_ENV=production
ENV PATH=/usr/local/bin:$PATH
ENV DATABASE_URL=postgres://localhost/mydb
```

Values can contain `=` characters:
```dockerfile
ENV CONNECTION=host=localhost;port=5432
```

---

### WORKDIR

```dockerfile
WORKDIR <path>
```

Sets the working directory for subsequent `RUN`, `COPY` instructions.

```dockerfile
WORKDIR /app
RUN pwd          # outputs /app

WORKDIR /app/src
COPY . .         # copies to /app/src/
```

The directory is created if it doesn't exist.

---

### RUN

```dockerfile
RUN <command>
```

Executes a shell command in the image.

**Single line:**
```dockerfile
RUN apt-get update
RUN npm install
```

**Multiline with backslash continuation:**
```dockerfile
RUN apt-get update && \
    apt-get install -y \
    nodejs \
    npm \
    git
```

**Chained commands:**
```dockerfile
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*
```

Commands run as the `cage` user. Use `sudo` for root operations:
```dockerfile
RUN sudo apt-get update
```

---

### COPY

```dockerfile
COPY <src> <dest>
```

Copies files or directories from the build context into the image.

**Copy a file:**
```dockerfile
COPY ./config.json /app/config.json
```

**Copy a directory:**
```dockerfile
COPY ./src /app/src
```

**Relative to WORKDIR:**
```dockerfile
WORKDIR /app
COPY ./package.json .      # copies to /app/package.json
COPY ./src ./src           # copies to /app/src/
```

**Notes:**
- Source paths are relative to the build context directory
- Destination must be an absolute path or relative to WORKDIR
- Source cannot escape the build context (no `../`)

---

## Comments

Lines starting with `#` are comments:

```dockerfile
# This is a comment
FROM ubuntu-24.04

# Install dependencies
RUN apt-get update
```

---

## Line Continuation

Use `\` at the end of a line to continue on the next line:

```dockerfile
RUN apt-get update && \
    apt-get install -y \
        nodejs \
        npm \
        git && \
    rm -rf /var/lib/apt/lists/*
```

This is joined into a single command. Useful for:
- Long package lists
- Complex shell commands
- Better readability

---

## Variable Substitution

Variables defined with `ARG` can be used in instructions:

| Syntax | Description |
|--------|-------------|
| `${VAR}` | Substitute variable (preferred) |
| `$VAR` | Substitute variable (simple form) |

```dockerfile
ARG VERSION=1.0
ARG REPO=myrepo

RUN echo "Building version ${VERSION}"
RUN git clone https://github.com/${REPO}/app.git
ENV APP_VERSION=$VERSION
```

Override with `--build-arg`:
```bash
cage build -t myimage --build-arg VERSION=2.0 --build-arg REPO=otherrepo .
```

---

## Build Context

The build context is the directory passed to `cage build`:

```bash
cage build -t myimage .           # current directory
cage build -t myimage ./project   # ./project directory
```

All `COPY` sources are relative to this directory. Files outside the context cannot be accessed.

---

## Complete Example

```dockerfile
# Development environment for Node.js project
FROM ubuntu-24.04

# Build arguments
ARG NODE_VERSION=20
ARG NPM_REGISTRY=https://registry.npmjs.org

# Environment
ENV NODE_ENV=development
ENV NPM_CONFIG_REGISTRY=${NPM_REGISTRY}

# Install system dependencies
RUN sudo apt-get update && \
    sudo apt-get install -y \
        curl \
        git \
        build-essential && \
    sudo rm -rf /var/lib/apt/lists/*

# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | sudo bash - && \
    sudo apt-get install -y nodejs

# Setup application
WORKDIR /app

# Install dependencies first (better caching in future)
COPY ./package.json ./package-lock.json ./
RUN npm ci

# Copy application source
COPY ./src ./src
COPY ./tsconfig.json ./

# Build
RUN npm run build
```

Build:
```bash
cage build -t my-node-app .
cage build -t my-node-app --build-arg NODE_VERSION=18 .
```

Use the built image:
```bash
cage start myproject --image my-node-app
```

---

## Common Examples

### Node.js Development Environment

```dockerfile
FROM ubuntu

RUN sudo apt update -y && sudo apt upgrade -y

# Install Node.js 22 via NodeSource
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | sudo bash - && \
    sudo apt install -y nodejs

# Verify
RUN node -v && npm -v
```

### Python Development Environment

```dockerfile
FROM ubuntu

RUN sudo apt update -y && \
    sudo apt install -y \
    python3 \
    python3-pip \
    python3-venv

WORKDIR /app
COPY ./requirements.txt .
RUN python3 -m venv venv && \
    . venv/bin/activate && \
    pip install -r requirements.txt
```

### Go Development Environment

```dockerfile
FROM ubuntu

ARG GO_VERSION=1.22.0

RUN sudo apt update -y && \
    sudo apt install -y wget

RUN wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

RUN go version
```

### Rust Development Environment

```dockerfile
FROM ubuntu

RUN sudo apt update -y && \
    sudo apt install -y curl build-essential

# Install Rust via rustup
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

ENV PATH=/home/cage/.cargo/bin:$PATH

RUN rustc --version && cargo --version
```

---

## Differences from Dockerfile

| Feature | Cagefile | Dockerfile |
|---------|----------|------------|
| Multi-stage builds | ❌ Not supported | ✅ Supported |
| LABEL | ❌ Not supported | ✅ Supported |
| USER | ❌ Not supported | ✅ Supported |
| EXPOSE | ❌ Not supported | ✅ Supported |
| ENTRYPOINT | ❌ Not supported | ✅ Supported |
| CMD | ❌ Not supported | ✅ Supported |
| ADD | ❌ Not supported | ✅ Supported |
| VOLUME | ❌ Not supported | ✅ Supported |
| HEALTHCHECK | ❌ Not supported | ✅ Supported |
| SHELL | ❌ Not supported | ✅ Supported |
| Layer caching | ❌ Not supported | ✅ Supported |

Cagefile focuses on the core instructions needed for building development environments. Advanced Docker features like multi-stage builds and layer caching may be added in future versions.

---

## Tips

### Combine RUN commands
Reduce build time by combining related commands:

```dockerfile
# Less efficient (multiple steps)
RUN apt-get update
RUN apt-get install -y nodejs
RUN apt-get install -y npm

# More efficient (single step)
RUN apt-get update && \
    apt-get install -y nodejs npm
```

### Clean up in same RUN
Remove temporary files in the same RUN to keep image small:

```dockerfile
RUN apt-get update && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*
```

### Use ARG for versions
Make versions configurable:

```dockerfile
ARG PYTHON_VERSION=3.11
RUN sudo apt-get install -y python${PYTHON_VERSION}
```

### Debug failed builds
Use `--keep-on-error` to keep the temporary cage for debugging:

```bash
cage build -t myimage --keep-on-error .
# If build fails, you can SSH into the temp cage
cage list  # find cage-build-XXXXX
cage ssh cage-build-XXXXX
```
