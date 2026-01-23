package network

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	ErrInvalidPortSpec = errors.New("invalid port specification")
	ErrPortOutOfRange  = errors.New("port number out of range (1-65535)")
	ErrInvalidProtocol = errors.New("invalid protocol (must be tcp or udp)")
	ErrInvalidBindAddr = errors.New("invalid bind address")
)

// PortForward represents a port forwarding rule
type PortForward struct {
	HostPort  int    `json:"host"`
	GuestPort int    `json:"guest"`
	Protocol  string `json:"protocol"`
	Bind      string `json:"bind"`
}

// ParsePortSpec parses a port specification string
// Formats:
//   - 8080:80           -> 127.0.0.1:8080:80/tcp
//   - 127.0.0.1:8080:80 -> explicit bind address
//   - 8080:80/udp       -> UDP protocol
//   - 0.0.0.0:8080:80   -> bind to all interfaces
func ParsePortSpec(spec string, defaultBind string) (*PortForward, error) {
	if spec == "" {
		return nil, ErrInvalidPortSpec
	}

	// Default values
	bind := defaultBind
	if bind == "" {
		bind = "127.0.0.1"
	}
	protocol := "tcp"

	// Check for protocol suffix
	if idx := strings.LastIndex(spec, "/"); idx != -1 {
		protocol = strings.ToLower(spec[idx+1:])
		spec = spec[:idx]
		if protocol != "tcp" && protocol != "udp" {
			return nil, ErrInvalidProtocol
		}
	}

	// Split by colons
	parts := strings.Split(spec, ":")

	var hostPort, guestPort int
	var err error

	switch len(parts) {
	case 2:
		// hostPort:guestPort
		hostPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid host port", ErrInvalidPortSpec)
		}
		guestPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid guest port", ErrInvalidPortSpec)
		}

	case 3:
		// bind:hostPort:guestPort
		bind = parts[0]
		if net.ParseIP(bind) == nil {
			return nil, ErrInvalidBindAddr
		}
		hostPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid host port", ErrInvalidPortSpec)
		}
		guestPort, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid guest port", ErrInvalidPortSpec)
		}

	default:
		return nil, ErrInvalidPortSpec
	}

	// Validate port ranges
	if hostPort < 1 || hostPort > 65535 {
		return nil, fmt.Errorf("%w: host port %d", ErrPortOutOfRange, hostPort)
	}
	if guestPort < 1 || guestPort > 65535 {
		return nil, fmt.Errorf("%w: guest port %d", ErrPortOutOfRange, guestPort)
	}

	return &PortForward{
		HostPort:  hostPort,
		GuestPort: guestPort,
		Protocol:  protocol,
		Bind:      bind,
	}, nil
}

// String returns a string representation of the port forward
func (p *PortForward) String() string {
	return fmt.Sprintf("%s:%d:%d/%s", p.Bind, p.HostPort, p.GuestPort, p.Protocol)
}

// ShortString returns a shorter string representation
func (p *PortForward) ShortString() string {
	if p.Bind == "127.0.0.1" {
		return fmt.Sprintf("%d:%d", p.HostPort, p.GuestPort)
	}
	return fmt.Sprintf("%s:%d:%d", p.Bind, p.HostPort, p.GuestPort)
}

// ParsePortSpecs parses multiple port specifications
func ParsePortSpecs(specs []string, defaultBind string) ([]PortForward, error) {
	var forwards []PortForward
	seen := make(map[int]bool)

	for _, spec := range specs {
		fwd, err := ParsePortSpec(spec, defaultBind)
		if err != nil {
			return nil, fmt.Errorf("invalid port spec '%s': %w", spec, err)
		}

		// Check for duplicate host ports
		if seen[fwd.HostPort] {
			return nil, fmt.Errorf("duplicate host port: %d", fwd.HostPort)
		}
		seen[fwd.HostPort] = true

		forwards = append(forwards, *fwd)
	}

	return forwards, nil
}

// PortConflict checks if a port is already in use on the host
func PortConflict(port int, bind string) bool {
	addr := fmt.Sprintf("%s:%d", bind, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true // port in use
	}
	ln.Close()
	return false
}
