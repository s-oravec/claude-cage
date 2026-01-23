package config

import (
	"errors"
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
}

// ImagesConfig holds image settings
type ImagesConfig struct {
	Default string `yaml:"default"`
}

// Profile defines resource limits for a cage
type Profile struct {
	VCPU         int `yaml:"vcpu"`
	MemoryMB     int `yaml:"memory_mb"`
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

// Exists returns true if config file exists
func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Images: ImagesConfig{
			Default: "ubuntu-24.04",
		},
		Profiles: map[string]Profile{
			"default": {VCPU: 4, MemoryMB: 4096, IOWeight: 500, MaxProcesses: 4096},
			"heavy":   {VCPU: 8, MemoryMB: 8192, IOWeight: 750, MaxProcesses: 8192},
			"light":   {VCPU: 2, MemoryMB: 2048, IOWeight: 250, MaxProcesses: 2048},
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
