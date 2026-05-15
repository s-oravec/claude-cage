package imgstore

import (
	"fmt"
	"strings"
)

// Ref is a parsed image reference. Host==Owner=="" => local-only ref.
type Ref struct {
	Host, Owner, Name, Tag string
}

// IsRegistry reports whether the ref is registry-qualified (has a Host).
func (r Ref) IsRegistry() bool { return r.Host != "" }

// RefPath returns the on-disk ref path under the current store Root,
// selecting the registry or local layout based on IsRegistry.
func (r Ref) RefPath() string {
	if r.IsRegistry() {
		return RegistryRefPath(r.Host, r.Owner, r.Name, r.Tag)
	}
	return LocalRefPath(r.Name, r.Tag)
}

// ParseRef recognizes 3-segment registry refs (host/owner/name[:tag])
// and single-segment local refs (name[:tag]). 2-segment refs without
// a registry are not supported.
func ParseRef(s string) (Ref, error) {
	if s == "" {
		return Ref{}, fmt.Errorf("ref is empty")
	}
	tag := "latest"
	if i := strings.LastIndex(s, ":"); i > 0 && !strings.Contains(s[i+1:], "/") {
		tag = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 1:
		return Ref{Name: parts[0], Tag: tag}, nil
	case 3:
		return Ref{Host: parts[0], Owner: parts[1], Name: parts[2], Tag: tag}, nil
	default:
		return Ref{}, fmt.Errorf("ref must be a bare name or host/owner/name:tag, got %q", strings.Join(parts, "/"))
	}
}
