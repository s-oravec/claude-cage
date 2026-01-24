package cmd

import (
	"fmt"
	"os"

	"github.com/s-oravec/claude-cage/internal/config"
)

// resolveCageName returns cage name from args or project config.
// Returns:
// - If args provided: (args[0], nil, nil)
// - If no args: load .claude-cage.yml from cwd, return (cfg.Cage, cfg, nil)
// - Error if no args and no config file
func resolveCageName(args []string) (string, *config.ProjectConfig, error) {
	if len(args) > 0 {
		return args[0], nil, nil
	}

	// No args - try to load project config from cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	if !config.ProjectConfigExists(cwd) {
		return "", nil, fmt.Errorf("cage name required or run from directory with %s", config.ProjectConfigFile)
	}

	cfg, err := config.LoadProjectConfig(cwd)
	if err != nil {
		return "", nil, err
	}

	return cfg.Cage, cfg, nil
}
