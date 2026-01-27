package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/spf13/cobra"
)

// StatusInfo holds detailed cage status information
type StatusInfo struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Image     string        `json:"image"`
	Profile   string        `json:"profile"`
	Uptime    string        `json:"uptime"`
	StartedAt time.Time     `json:"started_at"`
	Network   NetworkInfo   `json:"network"`
	Resources ResourceInfo  `json:"resources,omitempty"`
	Docker    *DockerInfo   `json:"docker,omitempty"`
	Shares    []ShareInfo   `json:"shares,omitempty"`
	Processes []ProcessInfo `json:"processes,omitempty"`
}

// NetworkInfo holds network status
type NetworkInfo struct {
	IP     string      `json:"ip"`
	Bridge string      `json:"bridge"`
	Ports  []cage.Port `json:"ports,omitempty"`
}

// ResourceInfo holds resource usage
type ResourceInfo struct {
	VCPU       int    `json:"vcpu"`
	MemoryMB   int    `json:"memory_mb"`
	CPUPercent string `json:"cpu_percent,omitempty"`
	MemPercent string `json:"mem_percent,omitempty"`
}

// DockerInfo holds docker status
type DockerInfo struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Images  int `json:"images"`
}

// ShareInfo holds share status
type ShareInfo struct {
	Host  string `json:"host"`
	Guest string `json:"guest"`
}

// ProcessInfo holds process information
type ProcessInfo struct {
	PID     string `json:"pid"`
	CPU     string `json:"cpu"`
	MEM     string `json:"mem"`
	Command string `json:"command"`
}

// NewStatusCmd creates the status command
func NewStatusCmd() *cobra.Command {
	var jsonOutput bool
	var watch bool

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed cage status",
		Long: `Display detailed status information for a cage.

Shows resource usage, network configuration, Docker status,
shared directories, and top processes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if watch {
				return watchStatus(cmd, name)
			}
			return showStatus(cmd, name, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Continuously update status")

	return cmd
}

func showStatus(cmd *cobra.Command, name string, jsonOutput bool) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Load state
	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	// Load config for profile info
	cfg, _ := config.Load()
	var profile *config.Profile
	if cfg != nil {
		profile, _ = cfg.GetProfile(state.Profile)
	}

	// Build status info
	info := StatusInfo{
		Name:      state.Name,
		Status:    state.Status,
		Image:     state.Image,
		Profile:   state.Profile,
		Uptime:    formatUptime(state.StartedAt),
		StartedAt: state.StartedAt,
		Network: NetworkInfo{
			IP:     state.IP,
			Bridge: network.BridgeName(name),
			Ports:  state.Ports,
		},
	}

	// Add resource info from profile
	if profile != nil {
		info.Resources = ResourceInfo{
			VCPU:     profile.VCPU,
			MemoryMB: profile.MemoryMB,
		}
	}

	// Add share info from config
	if cfg != nil && len(cfg.Shares) > 0 {
		for _, s := range cfg.Shares {
			info.Shares = append(info.Shares, ShareInfo{
				Host:  s.Host,
				Guest: s.Guest,
			})
		}
	}

	// If running, try to get docker info
	if state.Status == cage.StatusRunning && state.IP != "" && ssh.KeyExists(name) {
		info.Docker = getDockerInfo(name)
		info.Processes = getTopProcesses(name, 5)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Print formatted output
	printStatus(cmd, &info)
	return nil
}

func printStatus(cmd *cobra.Command, info *StatusInfo) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Cage: %s\n", info.Name)
	fmt.Fprintf(out, "Status: %s\n", info.Status)
	fmt.Fprintf(out, "Uptime: %s\n", info.Uptime)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Resources:")
	fmt.Fprintf(out, "  Profile: %s\n", info.Profile)
	if info.Resources.VCPU > 0 {
		fmt.Fprintf(out, "  vCPU:    %d\n", info.Resources.VCPU)
		fmt.Fprintf(out, "  Memory:  %d MB\n", info.Resources.MemoryMB)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Network:")
	fmt.Fprintf(out, "  IP:      %s\n", info.Network.IP)
	fmt.Fprintf(out, "  Bridge:  %s\n", info.Network.Bridge)
	if len(info.Network.Ports) > 0 {
		fmt.Fprintln(out, "  Ports:")
		for _, p := range info.Network.Ports {
			fmt.Fprintf(out, "    - %d → %d\n", p.Host, p.Guest)
		}
	}

	if info.Docker != nil {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Docker:")
		fmt.Fprintf(out, "  Containers: %d running, %d stopped\n", info.Docker.Running, info.Docker.Stopped)
		fmt.Fprintf(out, "  Images:     %d\n", info.Docker.Images)
	}

	if len(info.Shares) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Shares:")
		for _, s := range info.Shares {
			fmt.Fprintf(out, "  - %s → %s\n", s.Host, s.Guest)
		}
	}

	if len(info.Processes) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Processes (top 5 by CPU):")
		fmt.Fprintf(out, "  %-8s %-8s %-8s %s\n", "PID", "CPU%", "MEM%", "COMMAND")
		for _, p := range info.Processes {
			fmt.Fprintf(out, "  %-8s %-8s %-8s %s\n", p.PID, p.CPU, p.MEM, p.Command)
		}
	}
}

func watchStatus(cmd *cobra.Command, name string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		// Clear screen
		fmt.Fprint(cmd.OutOrStdout(), "\033[H\033[2J")

		// Print status
		if err := showStatus(cmd, name, false); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "(Press Ctrl+C to exit)")

		// Wait for next tick or Ctrl+C
		select {
		case <-ticker.C:
			continue
		case <-sigChan:
			return nil
		}
	}
}

func getDockerInfo(name string) *DockerInfo {
	// Running containers
	runningOut, err := sshExecCapture(name, "docker ps -q 2>/dev/null | wc -l")
	if err != nil {
		return nil
	}
	running := parseIntOrZero(runningOut)

	// All containers
	totalOut, _ := sshExecCapture(name, "docker ps -aq 2>/dev/null | wc -l")
	total := parseIntOrZero(totalOut)

	// Images
	imagesOut, _ := sshExecCapture(name, "docker images -q 2>/dev/null | wc -l")
	images := parseIntOrZero(imagesOut)

	return &DockerInfo{
		Running: running,
		Stopped: total - running,
		Images:  images,
	}
}

func getTopProcesses(name string, count int) []ProcessInfo {
	cmd := fmt.Sprintf("ps aux --sort=-pcpu 2>/dev/null | head -n %d", count+1)
	out, err := sshExecCapture(name, cmd)
	if err != nil {
		return nil
	}

	var processes []ProcessInfo
	lines := splitLines(out)

	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		fields := splitFields(line)
		if len(fields) >= 11 {
			processes = append(processes, ProcessInfo{
				PID:     fields[1],
				CPU:     fields[2],
				MEM:     fields[3],
				Command: truncate(joinFields(fields[10:]), 40),
			})
		}
	}
	return processes
}

func sshExecCapture(cageName, command string) (string, error) {
	state, err := cage.LoadState(cageName)
	if err != nil {
		return "", err
	}
	if state.IP == "" {
		return "", fmt.Errorf("cage has no IP")
	}
	return ssh.ExecCapture(cageName, state.IP, command)
}

func parseIntOrZero(s string) int {
	var n int
	_, _ = fmt.Sscanf(trimSpace(s), "%d", &n)
	return n
}

func splitLines(s string) []string {
	var lines []string
	line := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(c)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	field := ""
	inField := false
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if inField {
				fields = append(fields, field)
				field = ""
				inField = false
			}
		} else {
			field += string(c)
			inField = true
		}
	}
	if field != "" {
		fields = append(fields, field)
	}
	return fields
}

func joinFields(fields []string) string {
	result := ""
	for i, f := range fields {
		if i > 0 {
			result += " "
		}
		result += f
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
