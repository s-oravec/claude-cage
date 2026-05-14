package config

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// configDir can be overridden in tests
var configDir string

// vmArtifactsDir can be overridden in tests
var vmArtifactsDir string

// SetDir overrides the config directory (for testing)
func SetDir(dir string) {
	configDir = dir
}

// Config holds all cage configuration
type Config struct {
	Images   ImagesConfig       `yaml:"images"`
	Profiles map[string]Profile `yaml:"profiles"`
	Network  NetworkConfig      `yaml:"network"`
	Shares   []ShareConfig      `yaml:"shares"`
	Security SecurityConfig     `yaml:"security"`
	Env      map[string]string  `yaml:"env,omitempty"`
}

// ImagesConfig holds image settings
type ImagesConfig struct {
	Default string `yaml:"default"`
}

// Profile defines resource limits for a cage
type Profile struct {
	VCPU         int `yaml:"vcpu"`
	MemoryMB     int `yaml:"memory_mb"`
	DiskGB       int `yaml:"disk_gb"`
	IOWeight     int `yaml:"io_weight"`
	MaxProcesses int `yaml:"max_processes"`
}

// NetworkConfig holds network settings
type NetworkConfig struct {
	BlockedInterfaces []string `yaml:"blocked_interfaces"`
	BlockedSubnets    []string `yaml:"blocked_subnets"`
	DNS               []string `yaml:"dns"`
	PortBind          string   `yaml:"port_bind"`
}

// ShareConfig defines a host-guest directory share
type ShareConfig struct {
	Host  string `yaml:"host"`
	Guest string `yaml:"guest"`
	Mode  string `yaml:"mode,omitempty"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	MaxCages        int  `yaml:"max_cages"`
	VirtiofsSandbox bool `yaml:"virtiofsd_sandbox"`
}

// Dir returns the metadata directory: state.json, SSH keys, known_hosts.
// Always lives under the invoking user's home, including when running
// under sudo (SUDO_USER env var). This keeps user-facing artifacts
// readable and manageable from the user's normal shell.
func Dir() string {
	if configDir != "" {
		return configDir
	}
	if u := os.Getenv("SUDO_USER"); u != "" {
		if usr, err := user.Lookup(u); err == nil {
			return filepath.Join(usr.HomeDir, ".claude-cage")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-cage")
}

// VMArtifactsDir returns the directory for libvirt-readable VM artifacts:
// disk overlays, cloud-init ISOs, virtiofs-mount sources.
//
//   - User mode: same as Dir() (~/.claude-cage). libvirt session runs
//     QEMU as the user, no apparmor or perm issues.
//   - Root mode: /var/lib/libvirt/images/cage. Lives under the default
//     libvirt virt-aa-helper apparmor allow-list, so disk files and
//     cloud-init ISOs are readable by libvirt-qemu without per-host
//     apparmor surgery.
func VMArtifactsDir() string {
	if vmArtifactsDir != "" {
		return vmArtifactsDir
	}
	if os.Geteuid() == 0 {
		return "/var/lib/libvirt/images/cage"
	}
	return Dir()
}

// SetVMArtifactsDir overrides VMArtifactsDir (for tests).
func SetVMArtifactsDir(dir string) {
	vmArtifactsDir = dir
}

// SudoUserIDs returns the uid/gid of $SUDO_USER if set, or (-1, -1) otherwise.
// Use to chown files created under sudo back to the invoking user.
func SudoUserIDs() (uid, gid int) {
	name := os.Getenv("SUDO_USER")
	if name == "" {
		return -1, -1
	}
	usr, err := user.Lookup(name)
	if err != nil {
		return -1, -1
	}
	uid, _ = strconv.Atoi(usr.Uid)
	gid, _ = strconv.Atoi(usr.Gid)
	return uid, gid
}

// ChownToSudoUser recursively chowns a path tree to $SUDO_USER. No-op when
// not running under sudo. Used after creating cage metadata (state.json,
// SSH keys) under root so the invoking user retains ownership.
func ChownToSudoUser(root string) error {
	uid, gid := SudoUserIDs()
	if uid < 0 {
		return nil
	}
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

// Path returns the config file path
func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

// ProjectConfigFile is the canonical name written by `cage init`.
const ProjectConfigFile = ".cage.yml"

// ProjectConfigFiles are the project-config filenames cage will read,
// in priority order. Writes always use ProjectConfigFile.
var ProjectConfigFiles = []string{".cage.yml", ".cage.yaml"}

// FindProjectConfig returns the absolute path to whichever
// ProjectConfigFiles entry exists in dir, or empty string if none does.
func FindProjectConfig(dir string) string {
	for _, name := range ProjectConfigFiles {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ProjectNetwork holds network settings for a project
type ProjectNetwork struct {
	SSH   string   `yaml:"ssh,omitempty"`   // port number or "auto"
	Ports []string `yaml:"ports,omitempty"` // "host:guest" format
}

// ProjectConfig holds project-level configuration from .cage.yml
type ProjectConfig struct {
	Cage    string            `yaml:"cage,omitempty"`    // cage name, optional, defaults to directory name
	Image   string            `yaml:"image"`             // required, base image
	Profile string            `yaml:"profile,omitempty"` // references global profile
	Memory  string            `yaml:"memory,omitempty"`  // e.g. "4G", "8G"
	VCPU    int               `yaml:"vcpu,omitempty"`
	DiskGB  int               `yaml:"disk,omitempty"`
	Network ProjectNetwork    `yaml:"network,omitempty"`
	Shares  []ShareConfig     `yaml:"shares,omitempty"` // reuses existing ShareConfig
	Env     map[string]string `yaml:"env,omitempty"`
}

// LoadProjectConfig loads the project-level config from a directory.
// Tries ProjectConfigFiles in order; reports the canonical name in errors.
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	path := FindProjectConfig(dir)
	if path == "" {
		return nil, fmt.Errorf("project config file %s not found in %s", ProjectConfigFile, dir)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filepath.Base(path), err)
	}

	// Default cage name to directory name if not specified
	if cfg.Cage == "" {
		cfg.Cage = filepath.Base(dir)
	}

	// Validate required fields
	if cfg.Image == "" {
		return nil, fmt.Errorf("image is required in %s", filepath.Base(path))
	}

	return &cfg, nil
}

// ProjectConfigExists returns true if any accepted project config file
// (`.cage.yml` or `.cage.yaml`) exists in the directory.
func ProjectConfigExists(dir string) bool {
	return FindProjectConfig(dir) != ""
}

// PortMapping represents a host:guest port mapping
type PortMapping struct {
	Host  int
	Guest int
}

// ResolvedConfig is the fully resolved configuration for a cage
type ResolvedConfig struct {
	CageName  string
	Image     string
	ImagePath string // full path to image file
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

// ParseMemory parses a memory string like "4G" or "512M" to megabytes
func ParseMemory(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, errors.New("empty memory value")
	}

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

// ParsePortMapping parses a port mapping string like "8080:80" to PortMapping
func ParsePortMapping(s string) (PortMapping, error) {
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

// ResolveProjectConfig merges global config, profile, and project config into a final resolved config
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
		mb, err := ParseMemory(project.Memory)
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
		pm, err := ParsePortMapping(p)
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

	// Resolve image path - use the images directory from Dir()
	resolved.ImagePath = filepath.Join(Dir(), "images", project.Image+".qcow2")

	return resolved, nil
}

// Exists returns true if config file exists
func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Images: ImagesConfig{
			Default: "alpine",
		},
		Profiles: map[string]Profile{
			"default": {VCPU: 4, MemoryMB: 4096, DiskGB: 20, IOWeight: 500, MaxProcesses: 4096},
			"heavy":   {VCPU: 8, MemoryMB: 8192, DiskGB: 50, IOWeight: 750, MaxProcesses: 8192},
			"light":   {VCPU: 2, MemoryMB: 2048, DiskGB: 10, IOWeight: 250, MaxProcesses: 2048},
		},
		Network: NetworkConfig{
			BlockedInterfaces: []string{"tun+", "tailscale+", "wg+"},
			BlockedSubnets:    []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"},
			DNS:               []string{"1.1.1.1", "8.8.8.8"},
			PortBind:          "127.0.0.1",
		},
		Shares: []ShareConfig{
			{Host: "~/projects", Guest: "/workspace", Mode: "rw"},
		},
		Security: SecurityConfig{
			MaxCages:        10,
			VirtiofsSandbox: true,
		},
	}
}

// Load reads config from file, creating default config if it doesn't exist
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			// Auto-create default config on first use
			cfg := DefaultConfig()
			if saveErr := Save(cfg); saveErr != nil {
				// If we can't save, just return the default in memory
				return cfg, nil
			}
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadGlobal reads only the global config without project config merge
func LoadGlobal() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Merge merges another config into this one (other wins on conflicts)
func (c *Config) Merge(other *Config) {
	// Images - scalar, other wins if set
	if other.Images.Default != "" {
		c.Images.Default = other.Images.Default
	}

	// Profiles - merge map, other wins on key conflicts
	if other.Profiles != nil {
		if c.Profiles == nil {
			c.Profiles = make(map[string]Profile)
		}
		for k, v := range other.Profiles {
			c.Profiles[k] = v
		}
	}

	// Network - merge fields
	if len(other.Network.BlockedInterfaces) > 0 {
		c.Network.BlockedInterfaces = other.Network.BlockedInterfaces
	}
	if len(other.Network.BlockedSubnets) > 0 {
		c.Network.BlockedSubnets = other.Network.BlockedSubnets
	}
	if len(other.Network.DNS) > 0 {
		c.Network.DNS = other.Network.DNS
	}
	if other.Network.PortBind != "" {
		c.Network.PortBind = other.Network.PortBind
	}

	// Shares - array, other replaces
	if len(other.Shares) > 0 {
		c.Shares = other.Shares
	}

	// Security - scalars
	if other.Security.MaxCages > 0 {
		c.Security.MaxCages = other.Security.MaxCages
	}
	// VirtiofsSandbox is tricky - false is valid, so we always take other if Security was set
	// We'll use a simple heuristic: if MaxCages is set, assume Security section was specified
	if other.Security.MaxCages > 0 {
		c.Security.VirtiofsSandbox = other.Security.VirtiofsSandbox
	}

	// Env - merge map, other wins on key conflicts
	if other.Env != nil {
		if c.Env == nil {
			c.Env = make(map[string]string)
		}
		for k, v := range other.Env {
			c.Env[k] = v
		}
	}
}

// Save writes config to file
func Save(cfg *Config) error {
	// Ensure directory exists
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(Path(), data, 0644)
}

// CreateDefault creates the default config file
func CreateDefault() error {
	if Exists() {
		return errors.New("config already exists, use --force to overwrite")
	}
	return Save(DefaultConfig())
}

// CreateDefaultForce creates the default config, overwriting if exists
func CreateDefaultForce() error {
	return Save(DefaultConfig())
}

// GetProfile returns a profile by name
func (c *Config) GetProfile(name string) (*Profile, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return nil, errors.New("profile not found: " + name)
	}
	return &profile, nil
}
