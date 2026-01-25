# Cage Build Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `cage build` command that creates custom images from Cagefile using Dockerfile syntax.

**Architecture:** Parse Cagefile, create temporary cage from base image, execute instructions via SSH, save result as custom image.

**Tech Stack:** Go, Cobra CLI, existing cage/ssh/images packages.

---

## Task 1: Create Cagefile Parser

**Files:**
- Create: `internal/build/parser.go`
- Create: `internal/build/parser_test.go`

**Step 1: Write failing test for FROM parsing**

```go
// internal/build/parser_test.go
package build

import (
	"strings"
	"testing"
)

func TestParseFrom(t *testing.T) {
	input := "FROM ubuntu:22.04"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 1 {
		t.Fatalf("expected 1 instruction, got %d", len(instructions))
	}
	if instructions[0].Type != "FROM" {
		t.Errorf("expected FROM, got %s", instructions[0].Type)
	}
	if instructions[0].Value != "ubuntu:22.04" {
		t.Errorf("expected ubuntu:22.04, got %s", instructions[0].Value)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/build/... -v -run TestParseFrom`
Expected: FAIL - package does not exist

**Step 3: Write minimal parser implementation**

```go
// internal/build/parser.go
package build

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Instruction represents a single Cagefile instruction
type Instruction struct {
	Type  string   // FROM, RUN, COPY, ENV, ARG, WORKDIR
	Value string   // The argument(s) to the instruction
	Args  []string // Parsed arguments for COPY (src, dest)
}

// Cagefile represents a parsed Cagefile
type Cagefile struct {
	BaseImage    string
	Instructions []Instruction
}

// Parse reads a Cagefile and returns parsed instructions
func Parse(r io.Reader) ([]Instruction, error) {
	var instructions []Instruction
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		instruction, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, instruction)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Cagefile: %w", err)
	}

	return instructions, nil
}

func parseLine(line string, lineNum int) (Instruction, error) {
	// Split into instruction and arguments
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 1 {
		return Instruction{}, fmt.Errorf("line %d: empty instruction", lineNum)
	}

	instType := strings.ToUpper(parts[0])
	value := ""
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}

	switch instType {
	case "FROM", "RUN", "ENV", "ARG", "WORKDIR":
		if value == "" {
			return Instruction{}, fmt.Errorf("line %d: %s requires an argument", lineNum, instType)
		}
		return Instruction{Type: instType, Value: value}, nil

	case "COPY":
		if value == "" {
			return Instruction{}, fmt.Errorf("line %d: COPY requires source and destination", lineNum)
		}
		args := strings.Fields(value)
		if len(args) < 2 {
			return Instruction{}, fmt.Errorf("line %d: COPY requires source and destination", lineNum)
		}
		return Instruction{Type: instType, Value: value, Args: args}, nil

	default:
		return Instruction{}, fmt.Errorf("line %d: unknown instruction %s", lineNum, instType)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/build/... -v -run TestParseFrom`
Expected: PASS

**Step 5: Add more parser tests**

```go
// Add to internal/build/parser_test.go

func TestParseRun(t *testing.T) {
	input := "RUN apt-get update && apt-get install -y curl"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "RUN" {
		t.Errorf("expected RUN, got %s", instructions[0].Type)
	}
	if instructions[0].Value != "apt-get update && apt-get install -y curl" {
		t.Errorf("unexpected value: %s", instructions[0].Value)
	}
}

func TestParseCopy(t *testing.T) {
	input := "COPY ./src /app/src"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "COPY" {
		t.Errorf("expected COPY, got %s", instructions[0].Type)
	}
	if len(instructions[0].Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(instructions[0].Args))
	}
	if instructions[0].Args[0] != "./src" || instructions[0].Args[1] != "/app/src" {
		t.Errorf("unexpected args: %v", instructions[0].Args)
	}
}

func TestParseEnv(t *testing.T) {
	input := "ENV NODE_ENV=production"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "ENV" {
		t.Errorf("expected ENV, got %s", instructions[0].Type)
	}
}

func TestParseArg(t *testing.T) {
	input := "ARG VERSION=1.0"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "ARG" {
		t.Errorf("expected ARG, got %s", instructions[0].Type)
	}
}

func TestParseWorkdir(t *testing.T) {
	input := "WORKDIR /app"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "WORKDIR" {
		t.Errorf("expected WORKDIR, got %s", instructions[0].Type)
	}
}

func TestParseMultipleInstructions(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG NODE_VERSION=18
ENV NODE_ENV=production
WORKDIR /app
RUN apt-get update
COPY ./src /app/src`

	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 6 {
		t.Fatalf("expected 6 instructions, got %d", len(instructions))
	}
}

func TestParseCommentsAndEmptyLines(t *testing.T) {
	input := `# This is a comment
FROM ubuntu:22.04

# Another comment
RUN echo hello`

	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 2 {
		t.Fatalf("expected 2 instructions, got %d", len(instructions))
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown instruction", "UNKNOWN value"},
		{"FROM without arg", "FROM"},
		{"RUN without arg", "RUN"},
		{"COPY without dest", "COPY ./src"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
```

**Step 6: Run all parser tests**

Run: `go test ./internal/build/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/build/
git commit -m "feat(build): add Cagefile parser

Parses Dockerfile-compatible syntax with FROM, RUN, COPY, ENV, ARG, WORKDIR.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Add Cagefile Validation

**Files:**
- Modify: `internal/build/parser.go`
- Modify: `internal/build/parser_test.go`

**Step 1: Write failing test for validation**

```go
// Add to internal/build/parser_test.go

func TestValidateCagefile(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid cagefile",
			input:   "FROM ubuntu:22.04\nRUN echo hello",
			wantErr: false,
		},
		{
			name:    "missing FROM",
			input:   "RUN echo hello",
			wantErr: true,
		},
		{
			name:    "FROM not first",
			input:   "RUN echo hello\nFROM ubuntu:22.04",
			wantErr: true,
		},
		{
			name:    "multiple FROM",
			input:   "FROM ubuntu:22.04\nFROM alpine:3.18",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAndValidate(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAndValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/build/... -v -run TestValidateCagefile`
Expected: FAIL - ParseAndValidate not defined

**Step 3: Implement validation**

```go
// Add to internal/build/parser.go

// ParseAndValidate parses a Cagefile and validates its structure
func ParseAndValidate(r io.Reader) (*Cagefile, error) {
	instructions, err := Parse(r)
	if err != nil {
		return nil, err
	}

	if len(instructions) == 0 {
		return nil, fmt.Errorf("Cagefile is empty")
	}

	// FROM must be first instruction
	if instructions[0].Type != "FROM" {
		return nil, fmt.Errorf("first instruction must be FROM")
	}

	// Only one FROM allowed
	fromCount := 0
	for _, inst := range instructions {
		if inst.Type == "FROM" {
			fromCount++
		}
	}
	if fromCount > 1 {
		return nil, fmt.Errorf("multiple FROM instructions not supported")
	}

	return &Cagefile{
		BaseImage:    instructions[0].Value,
		Instructions: instructions[1:], // Skip FROM
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/build/... -v -run TestValidateCagefile`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/build/
git commit -m "feat(build): add Cagefile validation

Validates FROM is first and only appears once.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Add ARG Substitution

**Files:**
- Modify: `internal/build/parser.go`
- Modify: `internal/build/parser_test.go`

**Step 1: Write failing test for ARG substitution**

```go
// Add to internal/build/parser_test.go

func TestArgSubstitution(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG VERSION=1.0
RUN echo ${VERSION}
ENV APP_VERSION=${VERSION}`

	buildArgs := map[string]string{}
	cf, err := ParseAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved := cf.ResolveArgs(buildArgs)

	// RUN should have ${VERSION} replaced with 1.0
	if resolved.Instructions[1].Value != "echo 1.0" {
		t.Errorf("expected 'echo 1.0', got '%s'", resolved.Instructions[1].Value)
	}

	// ENV should have ${VERSION} replaced with 1.0
	if resolved.Instructions[2].Value != "APP_VERSION=1.0" {
		t.Errorf("expected 'APP_VERSION=1.0', got '%s'", resolved.Instructions[2].Value)
	}
}

func TestArgOverride(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG VERSION=1.0
RUN echo ${VERSION}`

	buildArgs := map[string]string{"VERSION": "2.0"}
	cf, err := ParseAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved := cf.ResolveArgs(buildArgs)

	if resolved.Instructions[1].Value != "echo 2.0" {
		t.Errorf("expected 'echo 2.0', got '%s'", resolved.Instructions[1].Value)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/build/... -v -run TestArg`
Expected: FAIL - ResolveArgs not defined

**Step 3: Implement ARG resolution**

```go
// Add to internal/build/parser.go

import "regexp"

// ResolveArgs resolves ARG values in instructions
// buildArgs overrides default values from ARG instructions
func (cf *Cagefile) ResolveArgs(buildArgs map[string]string) *Cagefile {
	// Collect ARG definitions with defaults
	args := make(map[string]string)
	for _, inst := range cf.Instructions {
		if inst.Type == "ARG" {
			name, defaultVal := parseArgValue(inst.Value)
			args[name] = defaultVal
		}
	}

	// Override with build args
	for k, v := range buildArgs {
		args[k] = v
	}

	// Create new instructions with substituted values
	resolved := &Cagefile{
		BaseImage:    cf.BaseImage,
		Instructions: make([]Instruction, len(cf.Instructions)),
	}

	for i, inst := range cf.Instructions {
		resolved.Instructions[i] = Instruction{
			Type:  inst.Type,
			Value: substituteArgs(inst.Value, args),
			Args:  inst.Args,
		}
		// Also substitute in Args for COPY
		if len(inst.Args) > 0 {
			resolved.Instructions[i].Args = make([]string, len(inst.Args))
			for j, arg := range inst.Args {
				resolved.Instructions[i].Args[j] = substituteArgs(arg, args)
			}
		}
	}

	return resolved
}

// parseArgValue parses "NAME=default" or "NAME" format
func parseArgValue(value string) (name, defaultVal string) {
	parts := strings.SplitN(value, "=", 2)
	name = parts[0]
	if len(parts) > 1 {
		defaultVal = parts[1]
	}
	return
}

// substituteArgs replaces ${VAR} and $VAR with values from args map
func substituteArgs(s string, args map[string]string) string {
	// Match ${VAR} pattern
	re := regexp.MustCompile(`\$\{(\w+)\}`)
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if val, ok := args[varName]; ok {
			return val
		}
		return match // Keep original if not found
	})

	// Match $VAR pattern (word boundary)
	re2 := regexp.MustCompile(`\$(\w+)`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:] // Remove $
		if val, ok := args[varName]; ok {
			return val
		}
		return match
	})

	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/build/... -v -run TestArg`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/build/
git commit -m "feat(build): add ARG substitution support

Resolves ${VAR} in RUN, ENV, COPY instructions with --build-arg override.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Create Build Executor

**Files:**
- Create: `internal/build/executor.go`

**Step 1: Create executor structure**

```go
// internal/build/executor.go
package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/ssh"
)

// BuildConfig contains configuration for the build
type BuildConfig struct {
	Tag          string            // Output image name
	ContextDir   string            // Build context directory
	CagefilePath string            // Path to Cagefile
	BuildArgs    map[string]string // Build arguments
	KeepOnError  bool              // Keep temp cage on error
	Output       io.Writer         // Output writer for progress
}

// Executor handles the build process
type Executor struct {
	config    *BuildConfig
	cagefile  *Cagefile
	tempCage  string // Temporary cage name
	sshPort   int    // SSH port for temp cage
	workdir   string // Current WORKDIR in cage
	env       map[string]string // Current ENV vars
}

// NewExecutor creates a new build executor
func NewExecutor(config *BuildConfig) *Executor {
	return &Executor{
		config:  config,
		workdir: "/",
		env:     make(map[string]string),
	}
}

// Build executes the full build process
func (e *Executor) Build() error {
	// Step 1: Parse Cagefile
	if err := e.parseCagefile(); err != nil {
		return err
	}

	// Step 2: Create temporary cage
	if err := e.createTempCage(); err != nil {
		return err
	}

	// Ensure cleanup unless KeepOnError
	defer func() {
		if e.tempCage != "" && !e.config.KeepOnError {
			e.cleanup()
		}
	}()

	// Step 3: Start cage and wait for SSH
	if err := e.startCage(); err != nil {
		return err
	}

	// Step 4: Execute instructions
	if err := e.executeInstructions(); err != nil {
		return err
	}

	// Step 5: Stop cage
	if err := e.stopCage(); err != nil {
		return err
	}

	// Step 6: Save as image
	if err := e.saveImage(); err != nil {
		return err
	}

	// Cleanup temp cage
	e.cleanup()

	return nil
}

func (e *Executor) log(format string, args ...interface{}) {
	if e.config.Output != nil {
		fmt.Fprintf(e.config.Output, format+"\n", args...)
	}
}
```

**Step 2: Commit initial executor**

```bash
git add internal/build/executor.go
git commit -m "feat(build): add executor structure

Initial build executor with Build() orchestration method.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Implement Executor Methods - Parse & Cage Creation

**Files:**
- Modify: `internal/build/executor.go`

**Step 1: Implement parseCagefile**

```go
// Add to internal/build/executor.go

func (e *Executor) parseCagefile() error {
	e.log("Step 1: Parsing Cagefile...")

	f, err := os.Open(e.config.CagefilePath)
	if err != nil {
		return fmt.Errorf("failed to open Cagefile: %w", err)
	}
	defer f.Close()

	cf, err := ParseAndValidate(f)
	if err != nil {
		return fmt.Errorf("failed to parse Cagefile: %w", err)
	}

	// Resolve ARG substitutions
	e.cagefile = cf.ResolveArgs(e.config.BuildArgs)

	e.log(" ---> Base image: %s", e.cagefile.BaseImage)
	e.log(" ---> %d instruction(s) to execute", len(e.cagefile.Instructions))

	return nil
}
```

**Step 2: Implement createTempCage**

```go
// Add to internal/build/executor.go

import (
	"math/rand"
	"os/exec"

	"github.com/s-oravec/claude-cage/internal/cloudinit"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/runtime"
)

func (e *Executor) createTempCage() error {
	e.log("Step 2: Creating temporary cage...")

	// Generate unique temp cage name
	e.tempCage = fmt.Sprintf("cage-build-%d", rand.Int()%100000)

	// Resolve image alias
	imageName := images.ResolveAlias(e.cagefile.BaseImage)

	// Check image exists
	if !images.IsDownloaded(imageName) {
		return fmt.Errorf("base image '%s' not found, run 'cage setup --base %s' first", imageName, imageName)
	}

	e.log(" ---> Using base image: %s", imageName)

	// Create cage directory
	cageDir := cage.Dir(e.tempCage)
	if err := cage.EnsureDir(e.tempCage); err != nil {
		return fmt.Errorf("failed to create cage directory: %w", err)
	}

	// Find free SSH port
	sshPort, err := network.FindFreePort()
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to find free port: %w", err)
	}
	e.sshPort = sshPort

	// Create qcow2 overlay (10G default for build)
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageDir, "disk.qcow2")

	createCmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath, "10G")
	if out, err := createCmd.CombinedOutput(); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create overlay: %s", string(out))
	}

	// Generate SSH keys
	if err := ssh.GenerateKeyPair(e.tempCage); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to generate SSH keys: %w", err)
	}

	pubKey, err := ssh.GetPublicKey(e.tempCage)
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Create runtime directory
	if err := runtime.EnsureRuntimeDir(cageDir); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Create cloud-init ISO
	cloudInitPath, err := cloudinit.GenerateISOWithConfig(cageDir, &cloudinit.CloudInitConfig{
		CageName:      e.tempCage,
		PubKey:        pubKey,
		MountVirtiofs: false,
		UseRuntimeEnv: false,
		InstallSSH:    true,
	})
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create cloud-init: %w", err)
	}

	// Load default profile for VM resources
	globalCfg, _ := config.Load()
	memoryMB := 2048
	vcpu := 2
	if globalCfg != nil {
		if profile, _ := globalCfg.GetProfile("default"); profile != nil {
			memoryMB = profile.MemoryMB
			vcpu = profile.VCPU
		}
	}

	// Generate domain XML
	runtimeDir := runtime.RuntimeDir(cageDir)
	domainCfg := &libvirt.DomainConfig{
		Name:         e.tempCage,
		MemoryMB:     memoryMB,
		VCPU:         vcpu,
		DiskPath:     overlayPath,
		CloudInitISO: cloudInitPath,
		NetworkName:  "",
		RuntimeDir:   runtimeDir,
		SSHPort:      sshPort,
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Define domain
	client := libvirt.NewClient()
	if err := client.DefineDomain(xml); err != nil {
		ssh.DeleteKeys(e.tempCage)
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to define domain: %w", err)
	}

	// Save state
	state := &cage.State{
		Name:        e.tempCage,
		Status:      cage.StatusStopped,
		Image:       imageName,
		Profile:     "default",
		NetworkMode: cage.NetworkAuto,
		SSHPort:     sshPort,
	}

	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	e.log(" ---> Temporary cage: %s (SSH port %d)", e.tempCage, sshPort)

	return nil
}
```

**Step 3: Commit**

```bash
git add internal/build/executor.go
git commit -m "feat(build): implement parse and cage creation

Parses Cagefile and creates temporary cage from base image.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Implement Executor Methods - Start, Stop, Cleanup

**Files:**
- Modify: `internal/build/executor.go`

**Step 1: Implement startCage**

```go
// Add to internal/build/executor.go

func (e *Executor) startCage() error {
	e.log("Step 3: Starting cage...")

	client := libvirt.NewClient()

	if err := client.StartDomain(e.tempCage); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Update state to running
	state, _ := cage.LoadState(e.tempCage)
	state.Status = cage.StatusRunning
	state.StartedAt = time.Now()
	cage.SaveState(state)

	// Wait for SSH with retry
	e.log(" ---> Waiting for SSH...")
	if err := ssh.WaitForSSHWithPort(e.tempCage, "127.0.0.1", e.sshPort, 120*time.Second); err != nil {
		return fmt.Errorf("SSH timeout: %w", err)
	}

	e.log(" ---> SSH ready")
	return nil
}
```

**Step 2: Implement stopCage**

```go
// Add to internal/build/executor.go

func (e *Executor) stopCage() error {
	e.log("Stopping temporary cage...")

	client := libvirt.NewClient()

	if err := client.StopDomain(e.tempCage); err != nil {
		// Try force stop
		client.DestroyDomain(e.tempCage)
	}

	// Wait for domain to stop
	for i := 0; i < 30; i++ {
		active, _ := client.IsDomainActive(e.tempCage)
		if !active {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Update state
	state, _ := cage.LoadState(e.tempCage)
	state.Status = cage.StatusStopped
	cage.SaveState(state)

	return nil
}
```

**Step 3: Implement cleanup**

```go
// Add to internal/build/executor.go

func (e *Executor) cleanup() {
	if e.tempCage == "" {
		return
	}

	client := libvirt.NewClient()

	// Force stop if still running
	if active, _ := client.IsDomainActive(e.tempCage); active {
		client.DestroyDomain(e.tempCage)
	}

	// Undefine domain
	client.UndefineDomain(e.tempCage)

	// Delete SSH keys
	ssh.DeleteKeys(e.tempCage)

	// Remove known hosts entry
	ssh.RemoveKnownHost(fmt.Sprintf("[127.0.0.1]:%d", e.sshPort))

	// Delete cage state and files
	cage.DeleteState(e.tempCage)

	e.tempCage = ""
}
```

**Step 4: Add WaitForSSHWithPort to ssh package**

```go
// Add to internal/ssh/connect.go

// WaitForSSHWithPort waits for SSH to become available on specific port
func WaitForSSHWithPort(cageName, host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		err := SSHExecWithPort(cageName, host, port, "true", false)
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return ErrSSHTimeout
}
```

**Step 5: Commit**

```bash
git add internal/build/executor.go internal/ssh/connect.go
git commit -m "feat(build): implement start, stop, cleanup methods

Manages temporary cage lifecycle with SSH wait and cleanup.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Implement Instruction Execution

**Files:**
- Modify: `internal/build/executor.go`

**Step 1: Implement executeInstructions**

```go
// Add to internal/build/executor.go

func (e *Executor) executeInstructions() error {
	total := len(e.cagefile.Instructions)

	for i, inst := range e.cagefile.Instructions {
		stepNum := i + 1 // +1 because FROM was step 1
		e.log("Step %d/%d : %s %s", stepNum+1, total+1, inst.Type, truncate(inst.Value, 50))

		var err error
		switch inst.Type {
		case "ARG":
			// ARG already processed during parsing
			e.log(" ---> Build arg set")
		case "ENV":
			err = e.executeEnv(inst)
		case "WORKDIR":
			err = e.executeWorkdir(inst)
		case "RUN":
			err = e.executeRun(inst)
		case "COPY":
			err = e.executeCopy(inst)
		default:
			err = fmt.Errorf("unknown instruction: %s", inst.Type)
		}

		if err != nil {
			return fmt.Errorf("step %d failed: %w", stepNum+1, err)
		}
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
```

**Step 2: Implement executeEnv**

```go
// Add to internal/build/executor.go

func (e *Executor) executeEnv(inst Instruction) error {
	// Parse KEY=VALUE
	parts := strings.SplitN(inst.Value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid ENV format: %s", inst.Value)
	}

	key := parts[0]
	value := parts[1]

	e.env[key] = value
	e.log(" ---> ENV %s=%s", key, value)

	return nil
}
```

**Step 3: Implement executeWorkdir**

```go
// Add to internal/build/executor.go

func (e *Executor) executeWorkdir(inst Instruction) error {
	e.workdir = inst.Value

	// Create directory in cage
	cmd := fmt.Sprintf("mkdir -p %s", e.workdir)
	_, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, cmd)
	if err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	e.log(" ---> WORKDIR %s", e.workdir)
	return nil
}
```

**Step 4: Implement executeRun**

```go
// Add to internal/build/executor.go

func (e *Executor) executeRun(inst Instruction) error {
	e.log(" ---> Running in %s", e.tempCage)

	// Build command with ENV and WORKDIR
	var envExports string
	for k, v := range e.env {
		envExports += fmt.Sprintf("export %s=%q; ", k, v)
	}

	cmd := fmt.Sprintf("cd %s && %s%s", e.workdir, envExports, inst.Value)

	// Execute and stream output
	output, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, cmd)
	if output != "" {
		// Print output line by line
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			e.log("%s", line)
		}
	}

	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
```

**Step 5: Implement executeCopy**

```go
// Add to internal/build/executor.go

func (e *Executor) executeCopy(inst Instruction) error {
	if len(inst.Args) < 2 {
		return fmt.Errorf("COPY requires source and destination")
	}

	src := inst.Args[0]
	dest := inst.Args[len(inst.Args)-1]

	// Resolve source relative to context directory
	srcPath := filepath.Join(e.config.ContextDir, src)

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source not found: %s", src)
	}

	// Resolve destination relative to WORKDIR if not absolute
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(e.workdir, dest)
	}

	e.log(" ---> Copying %s to %s", src, dest)

	// Use SCP to copy files
	if srcInfo.IsDir() {
		return e.scpDir(srcPath, dest)
	}
	return e.scpFile(srcPath, dest)
}

func (e *Executor) scpFile(src, dest string) error {
	keyPath := ssh.KeyPath(e.tempCage)
	knownHostsPath := ssh.KnownHostsPath()

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		"-P", fmt.Sprintf("%d", e.sshPort),
		src,
		fmt.Sprintf("cage@127.0.0.1:%s", dest),
	}

	cmd := exec.Command("scp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp failed: %s", string(out))
	}

	return nil
}

func (e *Executor) scpDir(src, dest string) error {
	keyPath := ssh.KeyPath(e.tempCage)
	knownHostsPath := ssh.KnownHostsPath()

	// Create destination directory first
	mkdirCmd := fmt.Sprintf("mkdir -p %s", dest)
	ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, mkdirCmd)

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		"-P", fmt.Sprintf("%d", e.sshPort),
		"-r", // recursive
		src + "/.", // copy contents, not directory itself
		fmt.Sprintf("cage@127.0.0.1:%s", dest),
	}

	cmd := exec.Command("scp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp failed: %s", string(out))
	}

	return nil
}
```

**Step 6: Commit**

```bash
git add internal/build/executor.go
git commit -m "feat(build): implement instruction execution

Executes ENV, WORKDIR, RUN, COPY instructions via SSH/SCP.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Implement Image Saving

**Files:**
- Modify: `internal/build/executor.go`

**Step 1: Implement saveImage**

```go
// Add to internal/build/executor.go

func (e *Executor) saveImage() error {
	e.log("Saving image as '%s'...", e.config.Tag)

	result, err := images.Save(e.tempCage, e.config.Tag, fmt.Sprintf("Built from %s", e.cagefile.BaseImage))
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	if result.VirtCustomizeError != "" {
		e.log(" ---> Warning: %s", result.VirtCustomizeError)
	}

	e.log("Successfully built image: %s", e.config.Tag)

	return nil
}
```

**Step 2: Commit**

```bash
git add internal/build/executor.go
git commit -m "feat(build): implement image saving

Saves stopped temp cage as custom image.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Create Build Command

**Files:**
- Create: `internal/cmd/build.go`
- Modify: `internal/cmd/root.go`

**Step 1: Create build command**

```go
// internal/cmd/build.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/build"
	"github.com/s-oravec/claude-cage/internal/images"
)

// NewBuildCmd creates the build command
func NewBuildCmd() *cobra.Command {
	var tag string
	var cagefilePath string
	var buildArgs []string
	var keepOnError bool

	cmd := &cobra.Command{
		Use:   "build <context>",
		Short: "Build an image from a Cagefile",
		Long: `Build a custom image by executing Cagefile instructions.

The Cagefile uses Dockerfile-compatible syntax:
  FROM <base-image>    - Base image (required, must be first)
  ARG <name>=<value>   - Build argument
  ENV <key>=<value>    - Environment variable
  WORKDIR <path>       - Set working directory
  COPY <src> <dest>    - Copy files from context
  RUN <command>        - Execute shell command

Example Cagefile:
  FROM ubuntu:22.04
  ARG VERSION=1.0
  RUN apt-get update && apt-get install -y curl
  COPY ./app /app
  WORKDIR /app
  RUN ./setup.sh

Usage:
  cage build -t my-image .
  cage build -t my-image -f ./custom/Cagefile ./project
  cage build -t my-image --build-arg VERSION=2.0 .`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args[0], tag, cagefilePath, buildArgs, keepOnError)
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Name for the built image (required)")
	cmd.Flags().StringVarP(&cagefilePath, "file", "f", "", "Path to Cagefile (default: <context>/Cagefile)")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Build argument (KEY=VALUE)")
	cmd.Flags().BoolVar(&keepOnError, "keep-on-error", false, "Keep temporary cage on build failure")

	cmd.MarkFlagRequired("tag")

	return cmd
}

func runBuild(cmd *cobra.Command, context, tag, cagefilePath string, buildArgsList []string, keepOnError bool) error {
	// Validate tag
	if tag == "" {
		return fmt.Errorf("--tag is required")
	}

	// Check if image already exists
	if images.Exists(tag) {
		return fmt.Errorf("image '%s' already exists", tag)
	}

	// Resolve context path
	contextDir, err := filepath.Abs(context)
	if err != nil {
		return fmt.Errorf("invalid context path: %w", err)
	}

	// Check context exists
	info, err := os.Stat(contextDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("context directory not found: %s", context)
	}

	// Resolve Cagefile path
	if cagefilePath == "" {
		cagefilePath = filepath.Join(contextDir, "Cagefile")
	} else {
		cagefilePath, err = filepath.Abs(cagefilePath)
		if err != nil {
			return fmt.Errorf("invalid Cagefile path: %w", err)
		}
	}

	// Check Cagefile exists
	if _, err := os.Stat(cagefilePath); err != nil {
		return fmt.Errorf("Cagefile not found: %s", cagefilePath)
	}

	// Parse build args
	buildArgs := make(map[string]string)
	for _, arg := range buildArgsList {
		parts := splitFirst(arg, "=")
		if len(parts) != 2 {
			return fmt.Errorf("invalid build arg format: %s (expected KEY=VALUE)", arg)
		}
		buildArgs[parts[0]] = parts[1]
	}

	// Create and run executor
	executor := build.NewExecutor(&build.BuildConfig{
		Tag:          tag,
		ContextDir:   contextDir,
		CagefilePath: cagefilePath,
		BuildArgs:    buildArgs,
		KeepOnError:  keepOnError,
		Output:       cmd.OutOrStdout(),
	})

	return executor.Build()
}

// splitFirst splits string on first occurrence of sep
func splitFirst(s, sep string) []string {
	idx := -1
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}
```

**Step 2: Register command in root.go**

```go
// In internal/cmd/root.go, add after line 33:
rootCmd.AddCommand(NewBuildCmd())
```

**Step 3: Commit**

```bash
git add internal/cmd/build.go internal/cmd/root.go
git commit -m "feat(build): add cage build command

CLI command with -t, -f, --build-arg, --keep-on-error flags.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Add E2E Tests

**Files:**
- Create: `test/e2e/build_test.go`

**Step 1: Create build e2e tests**

```go
// test/e2e/build_test.go
package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	// Check prerequisites
	stdout, _, err := runCage("doctor")
	if err != nil {
		t.Skipf("Prerequisites not met: %v", err)
	}
	if !strings.Contains(stdout, "All checks passed") {
		t.Skip("Doctor checks failed, skipping e2e tests")
	}

	// Check base image available
	stdout, _, _ = runCage("image", "list")
	if !strings.Contains(stdout, testImage) {
		t.Skipf("Test image %s not available", testImage)
	}

	t.Run("build simple image", func(t *testing.T) {
		// Create temp directory for build context
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create Cagefile
		cagefile := `FROM ` + testImage + `
RUN echo "hello from build" > /tmp/build-test.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		// Build image
		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		if !strings.Contains(stdout, "Successfully built image") {
			t.Errorf("expected success message, got: %s", stdout)
		}

		// Verify image exists
		stdout, _, _ = runCage("image", "list")
		if !strings.Contains(stdout, imageName) {
			t.Errorf("image not found in list")
		}
	})

	t.Run("build with COPY", func(t *testing.T) {
		// Create temp directory for build context
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create file to copy
		if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Create Cagefile
		cagefile := `FROM ` + testImage + `
WORKDIR /app
COPY ./test.txt /app/test.txt
RUN cat /app/test.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		// Build image
		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		// Should see the file content in output
		if !strings.Contains(stdout, "test content") {
			t.Errorf("expected to see file content in output")
		}
	})

	t.Run("build with ARG", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		cagefile := `FROM ` + testImage + `
ARG MESSAGE=default
RUN echo ${MESSAGE} > /tmp/message.txt && cat /tmp/message.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, "--build-arg", "MESSAGE=custom-value", tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		if !strings.Contains(stdout, "custom-value") {
			t.Errorf("expected custom-value in output, got: %s", stdout)
		}
	})

	t.Run("build error handling", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Invalid Cagefile - missing FROM
		cagefile := `RUN echo hello`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		imageName := uniqueName(t) + "-image"
		_, stderr, err := runCage("build", "-t", imageName, tmpDir)
		if err == nil {
			t.Error("expected error for invalid Cagefile")
			runCage("image", "remove", imageName, "--force")
		}

		if !strings.Contains(stderr, "FROM") {
			t.Errorf("expected FROM error message, got: %s", stderr)
		}
	})
}
```

**Step 2: Run tests**

Run: `go test ./test/e2e/... -v -run TestBuild -timeout 15m`
Expected: PASS (requires base image and running system)

**Step 3: Commit**

```bash
git add test/e2e/build_test.go
git commit -m "test(build): add e2e tests for cage build

Tests simple build, COPY, ARG, and error handling.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `README.md` (add build command to command list)

**Step 1: Add build to command documentation**

Add to the commands section in README.md:

```markdown
### Build Commands
- `cage build -t <name> <context>` - Build image from Cagefile
  - `-f <path>` - Custom Cagefile location
  - `--build-arg KEY=VALUE` - Build argument
  - `--keep-on-error` - Keep temp cage on failure
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add cage build command to README

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Summary

**Total Tasks:** 11
**New Files:** 5
- `internal/build/parser.go`
- `internal/build/parser_test.go`
- `internal/build/executor.go`
- `internal/cmd/build.go`
- `test/e2e/build_test.go`

**Modified Files:** 3
- `internal/cmd/root.go`
- `internal/ssh/connect.go`
- `README.md`

**Key Features:**
- Dockerfile-compatible Cagefile syntax
- FROM, ARG, ENV, WORKDIR, COPY, RUN instructions
- Build arguments with --build-arg override
- SCP-based file transfer for COPY
- Temporary cage cleanup (or --keep-on-error for debugging)
- Integration with existing image management
