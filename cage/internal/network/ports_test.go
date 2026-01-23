package network

import (
	"testing"
)

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		defaultBind string
		want        *PortForward
		wantErr     bool
	}{
		{
			name:        "simple port mapping",
			spec:        "8080:80",
			defaultBind: "127.0.0.1",
			want: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "127.0.0.1",
			},
		},
		{
			name:        "with explicit bind",
			spec:        "0.0.0.0:8080:80",
			defaultBind: "127.0.0.1",
			want: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "0.0.0.0",
			},
		},
		{
			name:        "with UDP protocol",
			spec:        "5353:53/udp",
			defaultBind: "127.0.0.1",
			want: &PortForward{
				HostPort:  5353,
				GuestPort: 53,
				Protocol:  "udp",
				Bind:      "127.0.0.1",
			},
		},
		{
			name:        "full specification",
			spec:        "192.168.1.1:8080:80/tcp",
			defaultBind: "127.0.0.1",
			want: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "192.168.1.1",
			},
		},
		{
			name:        "same port mapping",
			spec:        "3000:3000",
			defaultBind: "127.0.0.1",
			want: &PortForward{
				HostPort:  3000,
				GuestPort: 3000,
				Protocol:  "tcp",
				Bind:      "127.0.0.1",
			},
		},
		{
			name:        "empty default bind uses 127.0.0.1",
			spec:        "8080:80",
			defaultBind: "",
			want: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "127.0.0.1",
			},
		},
		{
			name:        "empty spec",
			spec:        "",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "invalid host port",
			spec:        "abc:80",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "invalid guest port",
			spec:        "8080:abc",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "port out of range - too low",
			spec:        "0:80",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "port out of range - too high",
			spec:        "70000:80",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "invalid protocol",
			spec:        "8080:80/sctp",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "invalid bind address",
			spec:        "notanip:8080:80",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "too many colons",
			spec:        "127.0.0.1:8080:80:extra",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
		{
			name:        "single port",
			spec:        "8080",
			defaultBind: "127.0.0.1",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortSpec(tt.spec, tt.defaultBind)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePortSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.HostPort != tt.want.HostPort {
				t.Errorf("HostPort = %v, want %v", got.HostPort, tt.want.HostPort)
			}
			if got.GuestPort != tt.want.GuestPort {
				t.Errorf("GuestPort = %v, want %v", got.GuestPort, tt.want.GuestPort)
			}
			if got.Protocol != tt.want.Protocol {
				t.Errorf("Protocol = %v, want %v", got.Protocol, tt.want.Protocol)
			}
			if got.Bind != tt.want.Bind {
				t.Errorf("Bind = %v, want %v", got.Bind, tt.want.Bind)
			}
		})
	}
}

func TestPortForwardString(t *testing.T) {
	fwd := &PortForward{
		HostPort:  8080,
		GuestPort: 80,
		Protocol:  "tcp",
		Bind:      "127.0.0.1",
	}

	expected := "127.0.0.1:8080:80/tcp"
	if got := fwd.String(); got != expected {
		t.Errorf("String() = %v, want %v", got, expected)
	}
}

func TestPortForwardShortString(t *testing.T) {
	tests := []struct {
		name string
		fwd  *PortForward
		want string
	}{
		{
			name: "localhost bind",
			fwd: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "127.0.0.1",
			},
			want: "8080:80",
		},
		{
			name: "non-localhost bind",
			fwd: &PortForward{
				HostPort:  8080,
				GuestPort: 80,
				Protocol:  "tcp",
				Bind:      "0.0.0.0",
			},
			want: "0.0.0.0:8080:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fwd.ShortString(); got != tt.want {
				t.Errorf("ShortString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePortSpecs(t *testing.T) {
	t.Run("multiple valid specs", func(t *testing.T) {
		specs := []string{"8080:80", "5432:5432", "3000:3000"}
		forwards, err := ParsePortSpecs(specs, "127.0.0.1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(forwards) != 3 {
			t.Errorf("expected 3 forwards, got %d", len(forwards))
		}
	})

	t.Run("duplicate host port", func(t *testing.T) {
		specs := []string{"8080:80", "8080:8080"}
		_, err := ParsePortSpecs(specs, "127.0.0.1")
		if err == nil {
			t.Error("expected error for duplicate host port")
		}
	})

	t.Run("invalid spec in list", func(t *testing.T) {
		specs := []string{"8080:80", "invalid"}
		_, err := ParsePortSpecs(specs, "127.0.0.1")
		if err == nil {
			t.Error("expected error for invalid spec")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		forwards, err := ParsePortSpecs(nil, "127.0.0.1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if forwards != nil {
			t.Error("expected nil for empty list")
		}
	})
}
