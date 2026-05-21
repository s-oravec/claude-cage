package images

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
)

// ImageSource defines a base image source
type ImageSource struct {
	Name        string
	URL         string
	Description string
}

// BaseAliasEntry mirrors the schema of base-aliases.json. Shared with cage-hub
// via a byte-identical vendored copy; see docs at
// ../cage-hub/docs/superpowers/specs/2026-05-18-base-aliases-unification-design.md.
type BaseAliasEntry struct {
	Name        string             `json:"name"`
	URLs        map[string]string  `json:"urls"`
	SHA256      map[string]*string `json:"sha256,omitempty"`
	Description string             `json:"description"`
}

//go:embed base-aliases.json
var baseAliasesData []byte

// baseAliases is the parsed canonical list, loaded once at init. The
// cage-hub server holds a byte-identical copy and validates manifest.base.name
// against the same set.
var baseAliases []BaseAliasEntry

func init() {
	if err := json.Unmarshal(baseAliasesData, &baseAliases); err != nil {
		panic("base-aliases.json invalid: " + err.Error())
	}
}

// imageAliases maps short names to canonical image names. CLI-only: the server
// never sees short names because the CLI resolves them before pushing.
var imageAliases = map[string]string{
	// Default aliases point to latest stable/LTS
	"alpine": "alpine-3.21",
	"ubuntu": "ubuntu-24.04",
	"debian": "debian-12",
}

// BaseImages returns available base images, indexed by canonical name.
func BaseImages() map[string]ImageSource {
	out := make(map[string]ImageSource, len(baseAliases))
	for _, e := range baseAliases {
		// URL is per-arch now; resolve via GetSource(name, arch).
		out[e.Name] = ImageSource{Name: e.Name, Description: e.Description}
	}
	return out
}

// AliasNames returns the sorted short alias names (e.g. alpine, debian, ubuntu).
// Used by the CLI to display the available aliases without hardcoding them.
func AliasNames() []string {
	names := make([]string, 0, len(imageAliases))
	for name := range imageAliases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveAlias resolves an image alias to canonical name
func ResolveAlias(name string) string {
	if canonical, ok := imageAliases[name]; ok {
		return canonical
	}
	return name
}

// ListAvailable returns names of available base images
func ListAvailable() []string {
	names := make([]string, 0, len(baseAliases))
	for _, e := range baseAliases {
		names = append(names, e.Name)
	}
	return names
}

// GetSource returns an image source by name (supports aliases) for the given
// architecture. The URL is resolved per-arch; if the entry has no URL for arch,
// ImageSource.URL is left empty (the build executor checks for empty later).
func GetSource(name, arch string) (*ImageSource, error) {
	name = ResolveAlias(name)
	for _, e := range baseAliases {
		if e.Name == name {
			return &ImageSource{
				Name:        e.Name,
				URL:         e.URLs[arch],
				Description: e.Description,
			}, nil
		}
	}
	return nil, errors.New("unknown image: " + name)
}
