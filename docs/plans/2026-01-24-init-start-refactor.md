# Init+Start Refactoring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor cage from create+start to init+start model where `cage start` in a project directory automatically creates/reconfigures the cage based on `.claude-cage.yml`.

**Architecture:** Remove `cage create` command. Add `cage init` to generate project config. Modify `cage start` to create cage if not exists, reconfigure if exists (env/shares/network), and start. Environment injection via virtiofs shared file instead of cloud-init baked values.

**Tech Stack:** Go, libvirt, virtiofs, YAML config

---

## Overview

### Current State
- `cage create` creates VM with baked-in cloud-init config
- `cage start` only starts existing VM
- Env vars baked at create time, cannot be changed
- No project-level config file

### Target State
- `cage init` creates `.claude-cage.yml` in project directory
- `cage start` reads config, creates/reconfigures cage, starts it
- Env vars injected via virtiofs at start time
- Shares/network reconfigured at start time (requires restart)
- Global config provides defaults and profiles

---

## Task 1: Add Project Config Type

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

### Step 1.1: Write failing test for ProjectConfig parsing

```go
// Add to config_test.go
func TestLoadProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude-cage.yml")

	content := `
cage: my-project
image: ubuntu-24.04
profile: default
memory: 8G

network:
  ssh: auto
  ports:
    - "8080:80"
    - "3000:3000"

shares:
  - host: ./src
    guest: /home/cage/src
  - host: ./data
    guest: /data
    mode: ro

env:
  NODE_ENV: development
  DEBUG: "true"
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "my-project", cfg.Cage)
	assert.Equal(t, "ubuntu-24.04", cfg.Image)
	assert.Equal(t, "default", cfg.Profile)
	assert.Equal(t, "8G", cfg.Memory)
	assert.Equal(t, "auto", cfg.Network.SSH)
	assert.Len(t, cfg.Network.Ports, 2)
	assert.Len(t, cfg.Shares, 2)
	assert.Equal(t, "development", cfg.Env["NODE_ENV"])
}

func TestLoadProjectConfig_CageNameFromDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude-cage.yml")

	content := `
image: ubuntu-24.04
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)

	// Cage name should be derived from directory name
	assert.Equal(t, filepath.Base(tmpDir), cfg.Cage)
}

func TestLoadProjectConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadProjectConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
```

### Step 1.2: Run test to verify it fails

```bash
go test ./internal/config/... -run TestLoadProjectConfig -v
```
Expected: FAIL - `LoadProjectConfig` undefined

### Step 1.3: Add ProjectConfig type and loader

```go
// Add to config.go

// ProjectConfig represents .claude-cage.yml in a project directory
type ProjectConfig struct {
	Cage    string            `yaml:"cage,omitempty"`    // cage name, defaults to dir name
	Image   string            `yaml:"image"`             // required
	Profile string            `yaml:"profile,omitempty"` // references global profile
	Memory  string            `yaml:"memory,omitempty"`  // overrides profile
	VCPU    int               `yaml:"vcpu,omitempty"`    // overrides profile
	DiskGB  int               `yaml:"disk,omitempty"`    // overrides profile
	Network ProjectNetwork    `yaml:"network,omitempty"`
	Shares  []ShareConfig     `yaml:"shares,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type ProjectNetwork struct {
	SSH   string   `yaml:"ssh,omitempty"`   // port number or "auto"
	Ports []string `yaml:"ports,omitempty"` // "host:guest" format
}

const ProjectConfigFile = ".claude-cage.yml"

// LoadProjectConfig loads project config from directory
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	configPath := filepath.Join(dir, ProjectConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project config %s not found in %s", ProjectConfigFile, dir)
		}
		return nil, fmt.Errorf("reading project config: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}

	// Default cage name to directory name
	if cfg.Cage == "" {
		cfg.Cage = filepath.Base(dir)
	}

	// Validate required fields
	if cfg.Image == "" {
		return nil, fmt.Errorf("image is required in %s", ProjectConfigFile)
	}

	return &cfg, nil
}

// ProjectConfigExists checks if project config exists in directory
func ProjectConfigExists(dir string) bool {
	configPath := filepath.Join(dir, ProjectConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}
```

### Step 1.4: Run tests to verify they pass

```bash
go test ./internal/config/... -run TestLoadProjectConfig -v
```
Expected: PASS

### Step 1.5: Commit

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ProjectConfig type for .claude-cage.yml"
```

---

## Task 2: Add ResolvedConfig Type

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

### Step 2.1: Write failing test for config resolution

```go
// Add to config_test.go
func TestResolveProjectConfig(t *testing.T) {
	globalCfg := &Config{
		Profiles: map[string]Profile{
			"default": {VCPU: 2, MemoryMB: 4096, DiskGB: 20},
			"heavy":   {VCPU: 4, MemoryMB: 8192, DiskGB: 50},
		},
		Images: ImagesConfig{
			BaseDir: "/var/lib/cage/images",
		},
	}

	projectCfg := &ProjectConfig{
		Cage:    "my-project",
		Image:   "ubuntu-24.04",
		Profile: "default",
		Memory:  "8G", // override profile
		Network: ProjectNetwork{
			SSH:   "auto",
			Ports: []string{"8080:80"},
		},
		Shares: []ShareConfig{
			{Host: "./src", Guest: "/home/cage/src"},
		},
		Env: map[string]string{"NODE_ENV": "dev"},
	}

	resolved, err := ResolveProjectConfig(globalCfg, projectCfg, "/home/user/myproject")
	require.NoError(t, err)

	assert.Equal(t, "my-project", resolved.CageName)
	assert.Equal(t, "ubuntu-24.04", resolved.Image)
	assert.Equal(t, 2, resolved.VCPU)           // from profile
	assert.Equal(t, 8192, resolved.MemoryMB)    // overridden (8G = 8192MB)
	assert.Equal(t, 20, resolved.DiskGB)        // from profile
	assert.Equal(t, "auto", resolved.SSHPort)
	assert.Len(t, resolved.Ports, 1)
	assert.Len(t, resolved.Shares, 1)
	// Share host path should be absolute
	assert.Equal(t, "/home/user/myproject/src", resolved.Shares[0].Host)
}
```

### Step 2.2: Run test to verify it fails

```bash
go test ./internal/config/... -run TestResolveProjectConfig -v
```
Expected: FAIL - `ResolveProjectConfig` undefined

### Step 2.3: Add ResolvedConfig type and resolver

```go
// Add to config.go

// ResolvedConfig is the fully resolved configuration for a cage
type ResolvedConfig struct {
	CageName  string
	Image     string
	ImagePath string
	VCPU      int
	MemoryMB  int
	DiskGB    int
	SSHPort   string // port number or "auto"
	Ports     []PortMapping
	Shares    []ShareConfig
	Env       map[string]string
	// From global config
	Network  NetworkConfig
	Security SecurityConfig
}

type PortMapping struct {
	Host  int
	Guest int
}

// ResolveProjectConfig merges global config, profile, and project config
func ResolveProjectConfig(global *Config, project *ProjectConfig, projectDir string) (*ResolvedConfig, error) {
	resolved := &ResolvedConfig{
		CageName: project.Cage,
		Image:    project.Image,
		Env:      project.Env,
		Network:  global.Network,
		Security: global.Security,
	}

	// Get profile (default if not specified)
	profileName := project.Profile
	if profileName == "" {
		profileName = "default"
	}
	profile, ok := global.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", profileName)
	}

	// Apply profile values
	resolved.VCPU = profile.VCPU
	resolved.MemoryMB = profile.MemoryMB
	resolved.DiskGB = profile.DiskGB

	// Apply project overrides
	if project.Memory != "" {
		mb, err := parseMemory(project.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory value: %w", err)
		}
		resolved.MemoryMB = mb
	}
	if project.VCPU > 0 {
		resolved.VCPU = project.VCPU
	}
	if project.DiskGB > 0 {
		resolved.DiskGB = project.DiskGB
	}

	// SSH port
	resolved.SSHPort = project.Network.SSH
	if resolved.SSHPort == "" {
		resolved.SSHPort = "auto"
	}

	// Parse port mappings
	for _, p := range project.Network.Ports {
		pm, err := parsePortMapping(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port mapping %q: %w", p, err)
		}
		resolved.Ports = append(resolved.Ports, pm)
	}

	// Resolve share paths to absolute
	for _, s := range project.Shares {
		share := s
		if !filepath.IsAbs(share.Host) {
			share.Host = filepath.Join(projectDir, share.Host)
		}
		resolved.Shares = append(resolved.Shares, share)
	}

	// Resolve image path
	resolved.ImagePath = filepath.Join(global.Images.BaseDir, project.Image+".qcow2")

	return resolved, nil
}

func parseMemory(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasSuffix(s, "G") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "G"))
		if err != nil {
			return 0, err
		}
		return val * 1024, nil
	}
	if strings.HasSuffix(s, "M") {
		return strconv.Atoi(strings.TrimSuffix(s, "M"))
	}
	return strconv.Atoi(s)
}

func parsePortMapping(s string) (PortMapping, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return PortMapping{}, fmt.Errorf("expected host:guest format")
	}
	host, err := strconv.Atoi(parts[0])
	if err != nil {
		return PortMapping{}, fmt.Errorf("invalid host port: %w", err)
	}
	guest, err := strconv.Atoi(parts[1])
	if err != nil {
		return PortMapping{}, fmt.Errorf("invalid guest port: %w", err)
	}
	return PortMapping{Host: host, Guest: guest}, nil
}
```

### Step 2.4: Run tests to verify they pass

```bash
go test ./internal/config/... -run TestResolveProjectConfig -v
```
Expected: PASS

### Step 2.5: Commit

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ResolvedConfig and config resolution"
```

---

## Task 3: Implement `cage init` Command

**Files:**
- Create: `internal/cmd/init.go`
- Test: `internal/cmd/init_test.go`
- Modify: `internal/cmd/root.go`

### Step 3.1: Write failing test for init command

```go
// internal/cmd/init_test.go
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand_CreatesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	cmd := NewInitCmd()
	cmd.SetArgs([]string{"--image", "ubuntu-24.04"})
	err := cmd.Execute()
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, ".claude-cage.yml")
	assert.FileExists(t, configPath)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "image: ubuntu-24.04")
}

func TestInitCommand_FailsIfConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude-cage.yml")
	os.WriteFile(configPath, []byte("existing"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	cmd := NewInitCmd()
	cmd.SetArgs([]string{"--image", "ubuntu-24.04"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
```

### Step 3.2: Run test to verify it fails

```bash
go test ./internal/cmd/... -run TestInitCommand -v
```
Expected: FAIL - `NewInitCmd` undefined

### Step 3.3: Implement init command

```go
// internal/cmd/init.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"claude-cage/internal/config"
)

func NewInitCmd() *cobra.Command {
	var opts struct {
		image   string
		cage    string
		memory  string
		vcpu    int
		disk    int
		sshPort string
		force   bool
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize cage configuration in current directory",
		Long: `Creates a .claude-cage.yml configuration file in the current directory.
This file configures how 'cage start' will create and run the cage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			configPath := filepath.Join(cwd, config.ProjectConfigFile)

			// Check if config already exists
			if _, err := os.Stat(configPath); err == nil && !opts.force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", config.ProjectConfigFile)
			}

			// Build config
			cfg := config.ProjectConfig{
				Image: opts.image,
				Network: config.ProjectNetwork{
					SSH: opts.sshPort,
				},
			}

			if opts.cage != "" {
				cfg.Cage = opts.cage
			}
			if opts.memory != "" {
				cfg.Memory = opts.memory
			}
			if opts.vcpu > 0 {
				cfg.VCPU = opts.vcpu
			}
			if opts.disk > 0 {
				cfg.DiskGB = opts.disk
			}

			// Add default share for current directory
			cfg.Shares = []config.ShareConfig{
				{Host: ".", Guest: "/workspace"},
			}

			// Write config
			data, err := yaml.Marshal(&cfg)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			header := "# Cage configuration for this project\n# See: https://github.com/s-oravec/claude-cage\n\n"
			if err := os.WriteFile(configPath, []byte(header+string(data)), 0644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Printf("Created %s\n", config.ProjectConfigFile)
			fmt.Println("Run 'cage start' to create and start the cage")
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.image, "image", "", "Base image for the cage (required)")
	cmd.Flags().StringVar(&opts.cage, "cage", "", "Cage name (default: directory name)")
	cmd.Flags().StringVar(&opts.memory, "memory", "", "Memory size (e.g., 4G, 8G)")
	cmd.Flags().IntVar(&opts.vcpu, "vcpu", 0, "Number of virtual CPUs")
	cmd.Flags().IntVar(&opts.disk, "disk", 0, "Disk size in GB")
	cmd.Flags().StringVar(&opts.sshPort, "ssh", "auto", "SSH port (number or 'auto')")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Overwrite existing config")

	cmd.MarkFlagRequired("image")

	return cmd
}
```

### Step 3.4: Register init command in root.go

```go
// Add to NewRootCmd() in root.go
rootCmd.AddCommand(NewInitCmd())
```

### Step 3.5: Run tests to verify they pass

```bash
go test ./internal/cmd/... -run TestInitCommand -v
```
Expected: PASS

### Step 3.6: Commit

```bash
git add internal/cmd/init.go internal/cmd/init_test.go internal/cmd/root.go
git commit -m "feat(cmd): add cage init command"
```

---

## Task 4: Add Runtime Env Injection via Virtiofs

**Files:**
- Create: `internal/runtime/env.go`
- Test: `internal/runtime/env_test.go`
- Modify: `internal/cloudinit/generate.go`

### Step 4.1: Write failing test for env file generation

```go
// internal/runtime/env_test.go
package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	env := map[string]string{
		"NODE_ENV":  "development",
		"API_KEY":   "secret123",
		"WITH_QUOTE": "it's ok",
	}

	err := WriteEnvFile(envPath, env)
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "export NODE_ENV='development'")
	assert.Contains(t, s, "export API_KEY='secret123'")
	assert.Contains(t, s, "export WITH_QUOTE='it'\\''s ok'") // escaped quote
}

func TestWriteEnvFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	err := WriteEnvFile(envPath, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Cage runtime environment")
}
```

### Step 4.2: Run test to verify it fails

```bash
go test ./internal/runtime/... -run TestWriteEnvFile -v
```
Expected: FAIL - package does not exist

### Step 4.3: Implement runtime env writer

```go
// internal/runtime/env.go
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RuntimeDir returns the runtime directory for a cage
func RuntimeDir(cageDir string) string {
	return filepath.Join(cageDir, "runtime")
}

// EnvFilePath returns the path to the env file for a cage
func EnvFilePath(cageDir string) string {
	return filepath.Join(RuntimeDir(cageDir), "env.sh")
}

// WriteEnvFile writes environment variables to a shell-sourceable file
func WriteEnvFile(path string, env map[string]string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating runtime directory: %w", err)
	}

	var lines []string
	lines = append(lines, "# Cage runtime environment")
	lines = append(lines, "# This file is auto-generated - do not edit")
	lines = append(lines, "")

	// Sort keys for deterministic output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := env[k]
		// Escape single quotes for shell
		escaped := strings.ReplaceAll(v, "'", "'\\''")
		lines = append(lines, fmt.Sprintf("export %s='%s'", k, escaped))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// EnsureRuntimeDir creates the runtime directory for a cage
func EnsureRuntimeDir(cageDir string) error {
	return os.MkdirAll(RuntimeDir(cageDir), 0755)
}
```

### Step 4.4: Run tests to verify they pass

```bash
go test ./internal/runtime/... -v
```
Expected: PASS

### Step 4.5: Commit

```bash
git add internal/runtime/
git commit -m "feat(runtime): add env file writer for virtiofs injection"
```

---

## Task 5: Update Cloud-init to Source Runtime Env

**Files:**
- Modify: `internal/cloudinit/generate.go`
- Test: `internal/cloudinit/generate_test.go`

### Step 5.1: Write failing test for runtime env sourcing

```go
// Add to generate_test.go
func TestGenerateCloudConfig_RuntimeEnvSource(t *testing.T) {
	cfg := &CloudInitConfig{
		CageName:         "test-cage",
		PubKey:           "ssh-rsa AAAA...",
		MountVirtiofs:    true,
		UseRuntimeEnv:    true, // new field
		InstallSSH:       true,
	}

	cloudConfig, err := GenerateCloudConfig(cfg)
	require.NoError(t, err)

	// Should source runtime env file
	assert.Contains(t, cloudConfig, "/cage/runtime/env.sh")
	assert.Contains(t, cloudConfig, "cage-runtime-env.sh")
}
```

### Step 5.2: Run test to verify it fails

```bash
go test ./internal/cloudinit/... -run TestGenerateCloudConfig_RuntimeEnvSource -v
```
Expected: FAIL - `UseRuntimeEnv` field does not exist

### Step 5.3: Add runtime env sourcing to cloud-init

```go
// Modify CloudInitConfig in generate.go
type CloudInitConfig struct {
	CageName      string
	PubKey        string
	MountVirtiofs bool
	UseRuntimeEnv bool              // NEW: source env from virtiofs
	Env           map[string]string // baked-in env (legacy, for create without project)
	InstallSSH    bool
}

// Add to generateWriteFiles() or similar
func generateRuntimeEnvProfile() string {
	return `# Source cage runtime environment
if [ -f /cage/runtime/env.sh ]; then
    . /cage/runtime/env.sh
fi
`
}

// In GenerateCloudConfig, add to write_files section when UseRuntimeEnv is true:
// - path: /etc/profile.d/cage-runtime-env.sh
//   content: |
//     # Source cage runtime environment
//     if [ -f /cage/runtime/env.sh ]; then
//         . /cage/runtime/env.sh
//     fi
//   permissions: '0644'
```

### Step 5.4: Run tests to verify they pass

```bash
go test ./internal/cloudinit/... -v
```
Expected: PASS

### Step 5.5: Commit

```bash
git add internal/cloudinit/generate.go internal/cloudinit/generate_test.go
git commit -m "feat(cloudinit): add runtime env sourcing from virtiofs"
```

---

## Task 6: Add Runtime Virtiofs Mount

**Files:**
- Modify: `internal/libvirt/domain.go`
- Test: `internal/libvirt/domain_test.go`

### Step 6.1: Write failing test for runtime virtiofs mount

```go
// Add to domain_test.go
func TestGenerateDomainXML_RuntimeMount(t *testing.T) {
	cfg := &DomainConfig{
		Name:           "test",
		MemoryMB:       4096,
		VCPU:           2,
		DiskPath:       "/path/to/disk.qcow2",
		CloudInitISO:   "/path/to/cloud-init.iso",
		RuntimeDir:     "/home/user/.claude-cage/cages/test/runtime", // NEW
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Should have runtime virtiofs mount
	assert.Contains(t, xml, `<target dir='cage-runtime'/>`)
	assert.Contains(t, xml, cfg.RuntimeDir)
}
```

### Step 6.2: Run test to verify it fails

```bash
go test ./internal/libvirt/... -run TestGenerateDomainXML_RuntimeMount -v
```
Expected: FAIL

### Step 6.3: Add runtime mount to domain XML

```go
// Add to DomainConfig in domain.go
type DomainConfig struct {
	Name           string
	MemoryMB       int
	VCPU           int
	DiskPath       string
	CloudInitISO   string
	NetworkName    string
	VirtiofsSocket string
	RuntimeDir     string // NEW: host path for runtime virtiofs
	SSHPort        int
}

// Add to domain XML template (filesystem section)
// When RuntimeDir is set, add a second virtiofs mount:
{{if .RuntimeDir}}
<filesystem type='mount' accessmode='passthrough'>
  <driver type='virtiofs'/>
  <source dir='{{.RuntimeDir}}'/>
  <target dir='cage-runtime'/>
</filesystem>
{{end}}
```

Note: This uses direct directory mount instead of virtiofsd socket for simplicity (read-only small files).

### Step 6.4: Update cloud-init to mount runtime virtiofs

```yaml
# Add to cloud-init mounts when UseRuntimeEnv is true
mounts:
  - [ cage-runtime, /cage/runtime, virtiofs, "ro,nofail", "0", "0" ]
```

### Step 6.5: Run tests to verify they pass

```bash
go test ./internal/libvirt/... -v
```
Expected: PASS

### Step 6.6: Commit

```bash
git add internal/libvirt/domain.go internal/libvirt/domain_test.go
git commit -m "feat(libvirt): add runtime virtiofs mount for env injection"
```

---

## Task 7: Refactor `cage start` to Create if Not Exists

**Files:**
- Modify: `internal/cmd/start.go`
- Modify: `internal/cage/state.go` (add SourceImage field)
- Test: `internal/cmd/start_test.go`

This is the largest task. Break into sub-steps.

### Step 7.1: Add SourceImage to State

```go
// Add to State in cage/state.go
type State struct {
	Name         string
	Status       string
	Image        string      // NEW: source image used to create cage
	Profile      string
	// ... rest unchanged
}
```

### Step 7.2: Write failing test for start creating cage

```go
// Add to start_test.go
func TestStartCommand_CreatesCageIfNotExists(t *testing.T) {
	// Setup: create project config but no cage
	tmpDir := t.TempDir()
	// ... setup mocks for libvirt, images, etc.

	// Expect: cage is created and started
}
```

### Step 7.3: Refactor start command

The new start flow:

```go
func runStart(cmd *cobra.Command, args []string) error {
	// 1. Determine cage name
	cwd, _ := os.Getwd()
	var cageName string
	var projectCfg *config.ProjectConfig
	var resolved *config.ResolvedConfig

	if len(args) > 0 {
		// Explicit cage name - legacy mode
		cageName = args[0]
	} else {
		// Project mode - load from .claude-cage.yml
		var err error
		projectCfg, err = config.LoadProjectConfig(cwd)
		if err != nil {
			return fmt.Errorf("no cage name provided and %v", err)
		}
		cageName = projectCfg.Cage

		// Load global config and resolve
		globalCfg, _ := config.Load()
		resolved, err = config.ResolveProjectConfig(globalCfg, projectCfg, cwd)
		if err != nil {
			return err
		}
	}

	// 2. Check if cage exists
	cageDir := cage.Dir(cageName)
	exists := cage.Exists(cageName)

	if !exists {
		if resolved == nil {
			return fmt.Errorf("cage %q does not exist", cageName)
		}
		// Create the cage
		if err := createCage(resolved, cageDir); err != nil {
			return err
		}
	} else {
		// Cage exists - validate and reconfigure
		state, err := cage.LoadState(cageName)
		if err != nil {
			return err
		}

		if resolved != nil {
			// Check image hasn't changed
			if state.Image != "" && state.Image != resolved.Image {
				return fmt.Errorf("cannot change image of existing cage (was: %s, config: %s). Delete and recreate the cage.", state.Image, resolved.Image)
			}

			// Reconfigure if stopped
			if state.Status != "running" {
				if err := reconfigureCage(resolved, cageDir, state); err != nil {
					return err
				}
			} else {
				fmt.Println("Cage is running. Restart to apply config changes.")
			}
		}
	}

	// 3. Write runtime env (if project mode)
	if resolved != nil && len(resolved.Env) > 0 {
		envPath := runtime.EnvFilePath(cageDir)
		if err := runtime.WriteEnvFile(envPath, resolved.Env); err != nil {
			return fmt.Errorf("writing env: %w", err)
		}
	}

	// 4. Start the cage (existing logic)
	return startDomain(cageName)
}

func createCage(cfg *config.ResolvedConfig, cageDir string) error {
	// Similar to current create.go but uses ResolvedConfig
	// 1. Create disk overlay
	// 2. Generate SSH keys
	// 3. Generate cloud-init (with UseRuntimeEnv: true)
	// 4. Generate domain XML (with RuntimeDir)
	// 5. Define domain
	// 6. Save state (with Image field set)
	return nil
}

func reconfigureCage(cfg *config.ResolvedConfig, cageDir string, state *cage.State) error {
	// 1. Update domain XML for shares/network changes
	// 2. Redefine domain in libvirt
	return nil
}
```

### Step 7.4: Run tests

```bash
go test ./internal/cmd/... -run TestStartCommand -v
```

### Step 7.5: Commit

```bash
git add internal/cmd/start.go internal/cage/state.go
git commit -m "feat(start): create cage from project config if not exists"
```

---

## Task 8: Implement Cage Reconfiguration

**Files:**
- Create: `internal/cage/reconfigure.go`
- Test: `internal/cage/reconfigure_test.go`

### Step 8.1: Write failing test

```go
// internal/cage/reconfigure_test.go
func TestReconfigure_UpdatesShares(t *testing.T) {
	// Test that shares in domain XML are updated
}

func TestReconfigure_UpdatesNetwork(t *testing.T) {
	// Test that network config is updated
}
```

### Step 8.2: Implement reconfiguration

```go
// internal/cage/reconfigure.go
package cage

import (
	"claude-cage/internal/config"
	"claude-cage/internal/libvirt"
)

// Reconfigure updates cage configuration (shares, network)
// Cage must be stopped.
func Reconfigure(cageName string, cfg *config.ResolvedConfig) error {
	state, err := LoadState(cageName)
	if err != nil {
		return err
	}

	if state.Status == "running" {
		return fmt.Errorf("cage must be stopped to reconfigure")
	}

	// Generate new domain XML
	domainCfg := &libvirt.DomainConfig{
		Name:       cageName,
		MemoryMB:   cfg.MemoryMB,
		VCPU:       cfg.VCPU,
		DiskPath:   filepath.Join(Dir(cageName), "disk.qcow2"),
		CloudInitISO: filepath.Join(Dir(cageName), "cloud-init.iso"),
		RuntimeDir: filepath.Join(Dir(cageName), "runtime"),
		// ... shares, network
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		return err
	}

	// Redefine domain
	client := libvirt.NewClient()
	return client.RedefineDomain(cageName, xml)
}
```

### Step 8.3: Add RedefineDomain to libvirt client

```go
// Add to internal/libvirt/client.go
func (c *Client) RedefineDomain(name, xml string) error {
	// Undefine existing
	if err := c.UndefineDomain(name); err != nil {
		// Ignore error if domain doesn't exist
	}
	// Define new
	return c.DefineDomain(xml)
}
```

### Step 8.4: Run tests

```bash
go test ./internal/cage/... -run TestReconfigure -v
```

### Step 8.5: Commit

```bash
git add internal/cage/reconfigure.go internal/cage/reconfigure_test.go internal/libvirt/client.go
git commit -m "feat(cage): add reconfiguration support for shares/network"
```

---

## Task 9: Update Other Commands to Read Project Config

**Files:**
- Modify: `internal/cmd/stop.go`
- Modify: `internal/cmd/ssh.go`
- Modify: `internal/cmd/remove.go`

### Step 9.1: Add helper for cage name resolution

```go
// internal/cmd/helpers.go
package cmd

import (
	"os"
	"claude-cage/internal/config"
)

// resolveCageName returns cage name from args or project config
func resolveCageName(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cfg, err := config.LoadProjectConfig(cwd)
	if err != nil {
		return "", fmt.Errorf("no cage name provided and %v", err)
	}

	return cfg.Cage, nil
}
```

### Step 9.2: Update stop command

```go
// In stop.go, change:
// name := args[0]
// To:
name, err := resolveCageName(args)
if err != nil {
	return err
}
```

### Step 9.3: Update ssh command

Same pattern as stop.

### Step 9.4: Update remove command

Same pattern as stop.

### Step 9.5: Run tests

```bash
go test ./internal/cmd/... -v
```

### Step 9.6: Commit

```bash
git add internal/cmd/helpers.go internal/cmd/stop.go internal/cmd/ssh.go internal/cmd/remove.go
git commit -m "feat(cmd): resolve cage name from project config in stop/ssh/rm"
```

---

## Task 10: Remove `cage create` Command

**Files:**
- Delete: `internal/cmd/create.go`
- Delete: `internal/cmd/create_test.go`
- Modify: `internal/cmd/root.go`

### Step 10.1: Remove create command registration

```go
// In root.go, remove:
rootCmd.AddCommand(NewCreateCmd())
```

### Step 10.2: Delete create.go and create_test.go

```bash
rm internal/cmd/create.go internal/cmd/create_test.go
```

### Step 10.3: Run all tests

```bash
go test ./... -v
```

### Step 10.4: Commit

```bash
git add -A
git commit -m "refactor(cmd): remove cage create command

cage start now handles creation when project config exists.
Use 'cage init' to create project config, then 'cage start'."
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/` (if exists)

### Step 11.1: Update README with new workflow

Document:
- `cage init --image <image>` - creates `.claude-cage.yml`
- `cage start` - creates/reconfigures/starts cage from config
- Project config format
- Global config profiles

### Step 11.2: Commit

```bash
git add README.md docs/
git commit -m "docs: update for init+start workflow"
```

---

## Task 12: E2E Testing

**Files:**
- Modify: `test/e2e/`

### Step 12.1: Add E2E test for init+start workflow

```go
func TestE2E_InitStartWorkflow(t *testing.T) {
	// 1. Create temp project directory
	// 2. Run cage init --image ubuntu-24.04
	// 3. Verify .claude-cage.yml created
	// 4. Run cage start
	// 5. Verify cage is running
	// 6. Run cage stop
	// 7. Modify .claude-cage.yml (add env var)
	// 8. Run cage start again
	// 9. SSH in and verify env var is set
	// 10. Cleanup
}
```

### Step 12.2: Run E2E tests

```bash
go test ./test/e2e/... -v
```

### Step 12.3: Commit

```bash
git add test/e2e/
git commit -m "test(e2e): add init+start workflow test"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add ProjectConfig type | config.go |
| 2 | Add ResolvedConfig | config.go |
| 3 | Implement cage init | init.go |
| 4 | Runtime env via virtiofs | runtime/env.go |
| 5 | Cloud-init runtime sourcing | cloudinit/generate.go |
| 6 | Runtime virtiofs mount | libvirt/domain.go |
| 7 | Start creates cage | start.go |
| 8 | Cage reconfiguration | cage/reconfigure.go |
| 9 | Update stop/ssh/rm | cmd/*.go |
| 10 | Remove create command | create.go |
| 11 | Update docs | README.md |
| 12 | E2E tests | test/e2e/ |

---

Plan complete and saved to `docs/plans/2026-01-24-init-start-refactor.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
