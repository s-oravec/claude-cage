package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest_RoundTripJSON(t *testing.T) {
	m := Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base: Base{
			Type:   "distro",
			Name:   "ubuntu-24.04",
			Digest: "sha256:abc",
		},
		Layers: []Layer{
			{Digest: "sha256:def", Size: 209715200, MediaType: MediaTypeLayerV1},
		},
		Config: Config{
			OS:      "linux",
			Arch:    "amd64",
			User:    "cage",
			Workdir: "/home/cage",
			Env:     []string{"K=V"},
		},
	}

	data, err := json.Marshal(&m)
	require.NoError(t, err)

	var got Manifest
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, m, got)
}

func TestDigestBytes_KnownVector(t *testing.T) {
	got := DigestBytes([]byte("hello"))
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", got)
}

func TestManifest_Canonical_Deterministic(t *testing.T) {
	m := Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "amd64"},
	}

	a, err := Canonical(&m)
	require.NoError(t, err)
	b, err := Canonical(&m)
	require.NoError(t, err)
	assert.Equal(t, a, b, "canonical encoding must be byte-identical across calls")

	d1, err := Digest(&m)
	require.NoError(t, err)
	d2, err := Digest(&m)
	require.NoError(t, err)
	assert.Equal(t, d1, d2)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, d1)
}

func TestManifest_Validate_AcceptsCanonical(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "amd64"},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_RejectsOffWhitelistOS(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "Linux", Arch: "amd64"}, // capitalized
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.os")
}

func TestManifest_Validate_RejectsOffWhitelistArch(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{{Digest: "sha256:def", Size: 1, MediaType: MediaTypeLayerV1}},
		Config:        Config{OS: "linux", Arch: "x86_64"},
	}
	err := m.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.arch")
}

func TestManifest_Validate_RejectsMissingLayer(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		MediaType:     MediaTypeManifestV1,
		Base:          Base{Type: "distro", Name: "ubuntu-24.04", Digest: "sha256:abc"},
		Layers:        []Layer{}, // empty
		Config:        Config{OS: "linux", Arch: "amd64"},
	}
	err := m.Validate()
	require.Error(t, err)
}
