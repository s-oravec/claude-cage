package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/s-oravec/cage/internal/config"
	"gopkg.in/yaml.v3"
)

// Entry is the stored credential for a single registry host.
type Entry struct {
	Token        string `yaml:"token"`
	RefreshToken string `yaml:"refresh_token,omitempty"`
	Username     string `yaml:"username,omitempty"`
	ObtainedAt   string `yaml:"obtained_at,omitempty"`
	ExpiresAt    string `yaml:"expires_at,omitempty"` // RFC3339; empty when unknown (e.g. PAT)
}

// Auth is the root structure of auth.yaml.
type Auth struct {
	Registries map[string]Entry `yaml:"registries"`
}

var dirOverride string

// SetDir overrides the auth file directory (testing).
func SetDir(d string) { dirOverride = d }

func dir() string {
	if dirOverride != "" {
		return dirOverride
	}
	return config.Dir()
}

func path() string { return filepath.Join(dir(), "auth.yaml") }

// Load reads ~/.cage/auth.yaml. Missing file returns empty Auth (no error).
// If the file has loose permissions, prints a warning and chmods to 0600.
func Load() (*Auth, error) {
	p := path()
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return &Auth{Registries: map[string]Entry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o600 {
		fmt.Fprintf(os.Stderr, "warning: auth.yaml has loose permissions %o; restoring to 0600\n", info.Mode().Perm())
		if err := os.Chmod(p, 0o600); err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var a Auth
	if err := yaml.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if a.Registries == nil {
		a.Registries = map[string]Entry{}
	}
	return &a, nil
}

// Save writes auth.yaml atomically with mode 0600.
func Save(a *Auth) error {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir(), ".auth.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path())
}

// AddHost stores a non-refreshable credential (e.g. a PAT) for a host.
func AddHost(host, token, username string) error {
	return AddHostFull(host, token, "", username, time.Time{})
}

// AddHostFull stores credentials for a host, replacing any existing entry.
// refreshToken and expiresAt may be empty/zero for non-refreshable tokens.
// ObtainedAt is set to the current time.
func AddHostFull(host, token, refreshToken, username string, expiresAt time.Time) error {
	a, err := Load()
	if err != nil {
		return err
	}
	e := Entry{
		Token:        token,
		RefreshToken: refreshToken,
		Username:     username,
		ObtainedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if !expiresAt.IsZero() {
		e.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}
	a.Registries[host] = e
	return Save(a)
}

// RemoveHost removes credentials for a host. Idempotent: returns nil if missing.
func RemoveHost(host string) error {
	a, err := Load()
	if err != nil {
		return err
	}
	delete(a.Registries, host)
	return Save(a)
}

// RemoveAll clears all stored credentials.
func RemoveAll() error {
	return Save(&Auth{Registries: map[string]Entry{}})
}

// Token returns the stored token for a host, ok==false if no entry.
func Token(host string) (string, bool) {
	a, err := Load()
	if err != nil {
		return "", false
	}
	e, ok := a.Registries[host]
	return e.Token, ok
}
