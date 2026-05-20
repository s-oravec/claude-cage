package images

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseAliasEntry_ParsesArchMaps(t *testing.T) {
	var e BaseAliasEntry
	json.Unmarshal([]byte(`{"name":"x","urls":{"amd64":"https://a","arm64":"https://b"},"sha256":{"amd64":null,"arm64":null},"description":"x"}`), &e)
	assert.Equal(t, "https://a", e.URLs["amd64"])
	assert.Equal(t, "https://b", e.URLs["arm64"])
}

func TestBaseImages_ContainsExpected(t *testing.T) {
	sources := BaseImages()

	assert.Contains(t, sources, "ubuntu-24.04")
	assert.Contains(t, sources, "ubuntu-22.04")
	assert.Contains(t, sources, "debian-12")
}

func TestImageSource_HasRequiredFields(t *testing.T) {
	sources := BaseImages()
	ubuntu := sources["ubuntu-24.04"]

	// BaseImages populates Name + Description; URL is per-arch (via GetSource).
	assert.NotEmpty(t, ubuntu.Name)
	assert.NotEmpty(t, ubuntu.Description)

	src, err := GetSource("ubuntu-24.04", "amd64")
	assert.NoError(t, err)
	assert.Contains(t, src.URL, "https://")
}

func TestListAvailable(t *testing.T) {
	names := ListAvailable()

	assert.Contains(t, names, "ubuntu-24.04")
	assert.Contains(t, names, "ubuntu-22.04")
	assert.Contains(t, names, "debian-12")
}

func TestGetSource_Exists(t *testing.T) {
	src, err := GetSource("ubuntu-24.04", "amd64")

	assert.NoError(t, err)
	assert.Equal(t, "ubuntu-24.04", src.Name)
}

func TestGetSource_NotExists(t *testing.T) {
	_, err := GetSource("nonexistent", "amd64")

	assert.Error(t, err)
}

func TestResolveAlias_KnownAliases(t *testing.T) {
	tests := []struct {
		alias    string
		expected string
	}{
		{"alpine", "alpine-3.21"},
		{"ubuntu", "ubuntu-24.04"},
		{"debian", "debian-12"},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			resolved := ResolveAlias(tt.alias)
			assert.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveAlias_UnknownReturnsInput(t *testing.T) {
	// Non-alias names should be returned as-is
	result := ResolveAlias("ubuntu-24.04")
	assert.Equal(t, "ubuntu-24.04", result)

	result = ResolveAlias("custom-image")
	assert.Equal(t, "custom-image", result)
}

func TestGetSource_WithAlias(t *testing.T) {
	// Getting source with alias should work
	src, err := GetSource("alpine", "amd64")
	assert.NoError(t, err)
	assert.Equal(t, "alpine-3.21", src.Name)
}

func TestBaseImages_AllDistros(t *testing.T) {
	sources := BaseImages()

	// Alpine
	assert.Contains(t, sources, "alpine-3.21")
	assert.Contains(t, sources, "alpine-3.20")

	// Ubuntu
	assert.Contains(t, sources, "ubuntu-24.04")
	assert.Contains(t, sources, "ubuntu-22.04")
	assert.Contains(t, sources, "ubuntu-20.04")

	// Debian
	assert.Contains(t, sources, "debian-12")
	assert.Contains(t, sources, "debian-11")
}

func TestBaseImages_AllHaveValidURLs(t *testing.T) {
	sources := BaseImages()

	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, src.Description, "image %s should have description", name)
			// URL is per-arch; verify each supported arch resolves to an HTTPS URL.
			for _, arch := range SupportedArchitectures {
				got, err := GetSource(name, arch)
				assert.NoError(t, err)
				assert.NotEmpty(t, got.URL, "image %s should have URL for %s", name, arch)
				assert.Contains(t, got.URL, "https://", "image %s/%s URL should be HTTPS", name, arch)
			}
		})
	}
}

func TestBaseImages_AllHaveMatchingName(t *testing.T) {
	sources := BaseImages()

	for key, src := range sources {
		assert.Equal(t, key, src.Name, "image key %s should match source name", key)
	}
}

func TestListAvailable_IncludesAllImages(t *testing.T) {
	names := ListAvailable()
	sources := BaseImages()

	// Should have same count
	assert.Len(t, names, len(sources))

	// All keys should be present
	for key := range sources {
		assert.Contains(t, names, key)
	}
}

func TestImageSource_Structure(t *testing.T) {
	src := ImageSource{
		Name:        "test-image",
		URL:         "https://example.com/image.qcow2",
		Description: "Test image description",
	}

	assert.Equal(t, "test-image", src.Name)
	assert.Equal(t, "https://example.com/image.qcow2", src.URL)
	assert.Equal(t, "Test image description", src.Description)
}
