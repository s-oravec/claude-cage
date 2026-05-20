package cmd

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestBody builds a JSON-encoded single-arch manifest with the given arch.
func manifestBody(t *testing.T, arch string) []byte {
	t.Helper()
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Config:        manifest.Config{OS: "linux", Arch: arch},
	}
	b, err := json.Marshal(&m)
	require.NoError(t, err)
	return b
}

// indexBody builds a JSON-encoded multi-arch index over the given arches.
func indexBody(t *testing.T, arches ...string) []byte {
	t.Helper()
	idx := manifest.IndexBody{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeIndexV1,
	}
	for _, a := range arches {
		idx.Manifests = append(idx.Manifests, manifest.IndexEntry{
			Digest:   "sha256:" + a,
			Platform: manifest.Platform{Architecture: a},
		})
	}
	b, err := json.Marshal(&idx)
	require.NoError(t, err)
	return b
}

func TestSelectArchManifest_SingleManifest_ArchMatch(t *testing.T) {
	body := manifestBody(t, "amd64")
	fetchCalled := false
	fetch := func(reference string) ([]byte, string, error) {
		fetchCalled = true
		return nil, "", errors.New("fetch should not be called")
	}

	selBody, selDigest, m, err := selectArchManifest(
		"amd64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.False(t, fetchCalled, "fetch must not be called for a single manifest")
	assert.Equal(t, body, selBody)
	assert.Equal(t, "sha256:idx", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "amd64", m.Config.Arch)
}

func TestSelectArchManifest_SingleManifest_ArchMismatch_NoOverride_Errors(t *testing.T) {
	body := manifestBody(t, "arm64")
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"amd64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--platform arm64")
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_SingleManifest_ArchMismatch_WithPlatform_Proceeds(t *testing.T) {
	body := manifestBody(t, "arm64")
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	selBody, selDigest, m, err := selectArchManifest(
		"arm64", manifest.MediaTypeManifestV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.Equal(t, body, selBody)
	assert.Equal(t, "sha256:idx", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "arm64", m.Config.Arch)
}

func TestSelectArchManifest_Index_PicksMatchingArch(t *testing.T) {
	body := indexBody(t, "amd64", "arm64")
	arm64Manifest := manifestBody(t, "arm64")

	var fetchedRef string
	fetch := func(reference string) ([]byte, string, error) {
		fetchedRef = reference
		return arm64Manifest, "sha256:arm64-docker-digest", nil
	}

	selBody, selDigest, m, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.NoError(t, err)
	assert.Equal(t, "sha256:arm64", fetchedRef, "fetch must be called with the arm64 entry digest")
	assert.Equal(t, arm64Manifest, selBody)
	assert.Equal(t, "sha256:arm64-docker-digest", selDigest)
	require.NotNil(t, m)
	assert.Equal(t, "arm64", m.Config.Arch)
}

func TestSelectArchManifest_Index_NoMatchingArch_Errors(t *testing.T) {
	body := indexBody(t, "amd64")
	fetchCalled := false
	fetch := func(reference string) ([]byte, string, error) {
		fetchCalled = true
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.False(t, fetchCalled, "fetch must not be called when no arch matches")
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_Index_EntryArchMismatch_Errors(t *testing.T) {
	body := indexBody(t, "arm64")
	// The index entry claims arm64, but the fetched manifest is actually amd64.
	mismatchedManifest := manifestBody(t, "amd64")
	fetch := func(reference string) ([]byte, string, error) {
		return mismatchedManifest, "sha256:amd64-docker-digest", nil
	}

	_, _, _, err := selectArchManifest(
		"arm64", manifest.MediaTypeIndexV1, body, "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "arm64")
	assert.Contains(t, err.Error(), "amd64")
}

func TestSelectArchManifest_UnexpectedContentType_Errors(t *testing.T) {
	fetch := func(reference string) ([]byte, string, error) {
		return nil, "", errors.New("fetch should not be called")
	}

	_, _, _, err := selectArchManifest(
		"amd64", "text/plain", []byte("{}"), "sha256:idx", fetch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "text/plain")
}

func TestPullCmd_Exists(t *testing.T) {
	cmd := NewPullCmd()

	assert.Equal(t, "pull", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestPullCmd_HasBaseFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("base")
	assert.NotNil(t, flag)
}

func TestPullCmd_HasListFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("list")
	assert.NotNil(t, flag)
}

func TestPullCmd_LongMentionsRegistry(t *testing.T) {
	c := NewPullCmd()
	assert.Contains(t, c.Long, "registry")
}

func TestPullCmd_HasPlatformFlag(t *testing.T) {
	cmd := NewPullCmd()

	flag := cmd.Flag("platform")
	assert.NotNil(t, flag)
}
