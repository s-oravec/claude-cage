package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// configDir can be overridden in tests
var configDir string

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
	Mode  string `yaml:"mode"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	MaxCages        int  `yaml:"max_cages"`
	VirtiofsSandbox bool `yaml:"virtiofsd_sandbox"`
}

// Dir returns the cage config directory
func Dir() string {
	if configDir != "" {
		return configDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-cage")
}

// Path returns the config file path
func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

// ProjectConfigFile is the name of the project-level config file
const ProjectConfigFile = ".claude-cage.yml"

// ProjectNetwork holds network settings for a project
type ProjectNetwork struct {
	SSH   string   `yaml:"ssh,omitempty"`   // port number or "auto"
	Ports []string `yaml:"ports,omitempty"` // "host:guest" format
}

// ProjectConfig holds project-level configuration from .claude-cage.yml
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

// LoadProjectConfig loads the project-level config from a directory
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, ProjectConfigFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project config file %s not found in %s", ProjectConfigFile, dir)
		}
		return nil, err
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ProjectConfigFile, err)
	}

	// Default cage name to directory name if not specified
	if cfg.Cage == "" {
		cfg.Cage = filepath.Base(dir)
	}

	// Validate required fields
	if cfg.Image == "" {
		return nil, fmt.Errorf("image is required in %s", ProjectConfigFile)
	}

	return &cfg, nil
}

// ProjectConfigExists returns true if the project config file exists in the directory
func ProjectConfigExists(dir string) bool {
	path := filepath.Join(dir, ProjectConfigFile)
	_, err := os.Stat(path)
	return err == nil
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

// Load reads config from file
func Load() (*Config, error) {
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
