package images

import (
	_ "embed"
	"encoding/json"
	"errors"
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
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	SHA256      *string `json:"sha256"`
	Description string  `json:"description"`
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
	"alpine":   "alpine-3.21",
	"ubuntu":   "ubuntu-24.04",
	"debian":   "debian-12",
	"rocky":    "rocky-9",
	"alma":     "alma-9",
	"fedora":   "fedora-41",
	"opensuse": "opensuse-15.6",
	"centos":   "centos-stream-9",
}

// BaseImages returns available base images, indexed by canonical name.
func BaseImages() map[string]ImageSource {
	out := make(map[string]ImageSource, len(baseAliases))
	for _, e := range baseAliases {
		out[e.Name] = ImageSource{Name: e.Name, URL: e.URL, Description: e.Description}
	}
	return out
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

// GetSource returns an image source by name (supports aliases)
func GetSource(name string) (*ImageSource, error) {
	name = ResolveAlias(name)
	sources := BaseImages()
	src, ok := sources[name]
	if !ok {
		return nil, errors.New("unknown image: " + name)
	}
	return &src, nil
}
