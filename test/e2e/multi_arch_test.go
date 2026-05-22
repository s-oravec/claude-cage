package e2e

// Multi-arch e2e tests that exercise the LIVE cage-hub registry.
//
// These tests drive internal/registry directly (not the cage CLI binary)
// against a running cage-hub instance, verifying the server's index
// auto-composition contract: pushing two different architectures to the
// same tag composes a 2-arch index, while a single arch (or same-arch
// re-push) stays a plain manifest.
//
// Gating: the tests SKIP unless CAGE_HUB_URL and CAGE_HUB_TOKEN are set.
// Run with a fresh Keycloak token, e.g.:
//
//	CAGE_HUB_URL=localhost CAGE_HUB_TOKEN=<token> CAGE_HUB_OWNER=stiivo \
//	  go test ./test/e2e/ -run 'TestMultiArch' -count=1 -v
//
// Server contract relied upon (verified against cage-hub source):
//   - base.name must be a known alias; we use "alpine-3.21".
//   - base.type must be the literal "distro" (z.literal in the shared Zod
//     ManifestSchema); using anything else returns 400 MANIFEST_INVALID.
//   - every referenced layer digest must already be an uploaded blob, so we
//     upload the layer blob BEFORE pushing the manifest. The first blob
//     upload-init (POST /blobs/uploads) auto-creates the repo via
//     getOrCreateRepoForPush, so the natural "blob then manifest" order works.
//   - first push to a tag -> tag_target_kind "manifest"; a DIFFERENT arch
//     pushed to the same tag -> server composes a 2-arch index and returns
//     tag_target_kind "index".

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/s-oravec/cage/internal/manifest"
	"github.com/s-oravec/cage/internal/registry"
	"github.com/stretchr/testify/require"
)

// multiArchEnv carries the resolved test configuration read from CAGE_HUB_* env.
type multiArchEnv struct {
	url      string
	token    string
	owner    string
	insecure bool
}

// loadMultiArchEnv reads the CAGE_HUB_* environment and skips the test when the
// URL or token are absent, so the suite is a clean no-op without a live server.
func loadMultiArchEnv(t *testing.T) multiArchEnv {
	t.Helper()
	url := os.Getenv("CAGE_HUB_URL")
	token := os.Getenv("CAGE_HUB_TOKEN")
	if url == "" || token == "" {
		t.Skip("set CAGE_HUB_URL and CAGE_HUB_TOKEN to run the multi-arch e2e")
	}
	owner := os.Getenv("CAGE_HUB_OWNER")
	if owner == "" {
		owner = "stiivo"
	}
	// Insecure (plain http) is the default for local dev; only HTTPS when
	// explicitly opted out via CAGE_HUB_INSECURE=false.
	insecure := os.Getenv("CAGE_HUB_INSECURE") != "false"
	return multiArchEnv{url: url, token: token, owner: owner, insecure: insecure}
}

// uniqueRepoName returns a collision-resistant repo name per test run.
func uniqueRepoName(t *testing.T, prefix string) string {
	t.Helper()
	var rb [6]byte
	_, err := rand.Read(rb[:])
	require.NoError(t, err, "crypto/rand")
	return prefix + "-" + hex.EncodeToString(rb[:])
}

// buildManifest constructs a valid v1 manifest for the given arch that
// references the single uploaded layer. base.type is "distro" and base.name is
// the "alpine-3.21" alias, both required by the server's schema.
func buildManifest(arch, layerDigest string, layerSize int64) *manifest.Manifest {
	return &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Base: manifest.Base{
			Type:   "distro",
			Name:   "alpine-3.21",
			Digest: manifest.DigestBytes([]byte("base-" + arch)),
		},
		Layers: []manifest.Layer{{
			Digest:    layerDigest,
			Size:      layerSize,
			MediaType: manifest.MediaTypeLayerV1,
		}},
		Config: manifest.Config{OS: "linux", Arch: arch},
	}
}

// TestMultiArch_IndexComposition verifies the core multi-arch server behavior:
// pushing a second, different architecture to a tag composes an index, and the
// index entries point at per-arch manifests whose config.arch matches.
func TestMultiArch_IndexComposition(t *testing.T) {
	env := loadMultiArchEnv(t)

	rc, err := registry.NewClient(env.url, registry.Options{Token: env.token, Insecure: env.insecure})
	require.NoError(t, err, "NewClient")

	owner := env.owner
	name := uniqueRepoName(t, "e2e-multiarch")
	const tag = "1.0"

	// Build a tiny unique layer blob and upload it BEFORE any manifest push.
	// Uniqueness ensures the blob (and hence the manifest digests) differ
	// between runs and never collide with prior state.
	layer := []byte("cage-e2e-multiarch-layer-" + name)
	layerDigest := manifest.DigestBytes(layer)
	err = rc.UploadBlob(owner, name, layerDigest, int64(len(layer)), 8<<20, bytes.NewReader(layer), nil)
	require.NoError(t, err, "UploadBlob (must succeed before manifest push; upload-init auto-creates the repo)")

	// push builds the per-arch manifest, pushes it, and returns the PUT result
	// plus the canonical manifest digest the server should have computed.
	push := func(arch string) (*registry.PutManifestResult, string) {
		t.Helper()
		m := buildManifest(arch, layerDigest, int64(len(layer)))
		body, err := manifest.Canonical(m)
		require.NoError(t, err, "Canonical(%s)", arch)
		res, err := rc.PutManifest(owner, name, tag, body, false)
		require.NoError(t, err, "PutManifest(%s)", arch)
		return res, manifest.DigestBytes(body)
	}

	// Step 1: first push (amd64) -> single manifest target.
	resAmd, digAmd := push("amd64")
	require.Equal(t, "manifest", resAmd.TagTargetKind, "first push should be a single manifest")
	require.Equal(t, digAmd, resAmd.ManifestDigest, "server manifest_digest must match canonical digest")

	// Step 2: push a DIFFERENT arch (arm64) to the same tag -> index composed.
	// This is THE key multi-arch server behavior under test.
	resArm, digArm := push("arm64")
	require.Equal(t, "index", resArm.TagTargetKind, "pushing a second arch must compose an index")
	require.Equal(t, digArm, resArm.ManifestDigest, "server manifest_digest must match canonical digest")

	// Step 3: GET the tag -> the server must serve an index with both arches.
	body, contentType, _, err := rc.GetManifest(owner, name, tag)
	require.NoError(t, err, "GetManifest(tag)")
	require.Equal(t, manifest.MediaTypeIndexV1, contentType, "tag should resolve to an index")

	var idx manifest.IndexBody
	require.NoError(t, json.Unmarshal(body, &idx), "unmarshal IndexBody")

	// Map arch -> entry digest and assert both arches are present with the
	// per-arch manifest digests the PUTs returned.
	byArch := map[string]string{}
	for _, e := range idx.Manifests {
		byArch[e.Platform.Architecture] = e.Digest
	}
	require.Contains(t, byArch, "amd64", "index must contain amd64 entry")
	require.Contains(t, byArch, "arm64", "index must contain arm64 entry")
	require.Equal(t, digAmd, byArch["amd64"], "amd64 entry digest must match the amd64 manifest")
	require.Equal(t, digArm, byArch["arm64"], "arm64 entry digest must match the arm64 manifest")

	// Step 4: for each index entry, fetch the manifest by digest and assert the
	// fetched manifest's config.arch matches the entry's platform.architecture.
	// This arch-consistency is what the CLI's pull dispatch relies on.
	for _, e := range idx.Manifests {
		entryBody, entryCT, _, err := rc.GetManifest(owner, name, e.Digest)
		require.NoError(t, err, "GetManifest(entry %s)", e.Digest)
		require.Equal(t, manifest.MediaTypeManifestV1, entryCT, "index entry must resolve to a plain manifest")

		var em manifest.Manifest
		require.NoError(t, json.Unmarshal(entryBody, &em), "unmarshal entry manifest")
		require.Equal(t, e.Platform.Architecture, em.Config.Arch,
			"fetched manifest arch must match index entry platform.architecture")
	}
}

// TestMultiArch_SingleManifestContentType is a regression guard: a tag with only
// one arch pushed to it must resolve to a plain manifest (not an index).
func TestMultiArch_SingleManifestContentType(t *testing.T) {
	env := loadMultiArchEnv(t)

	rc, err := registry.NewClient(env.url, registry.Options{Token: env.token, Insecure: env.insecure})
	require.NoError(t, err, "NewClient")

	owner := env.owner
	name := uniqueRepoName(t, "e2e-single")
	const tag = "1.0"

	layer := []byte("cage-e2e-single-layer-" + name)
	layerDigest := manifest.DigestBytes(layer)
	err = rc.UploadBlob(owner, name, layerDigest, int64(len(layer)), 8<<20, bytes.NewReader(layer), nil)
	require.NoError(t, err, "UploadBlob")

	m := buildManifest("amd64", layerDigest, int64(len(layer)))
	body, err := manifest.Canonical(m)
	require.NoError(t, err, "Canonical")
	res, err := rc.PutManifest(owner, name, tag, body, false)
	require.NoError(t, err, "PutManifest")
	require.Equal(t, "manifest", res.TagTargetKind, "single arch push must stay a manifest")

	_, contentType, _, err := rc.GetManifest(owner, name, tag)
	require.NoError(t, err, "GetManifest(tag)")
	require.Equal(t, manifest.MediaTypeManifestV1, contentType, "single-arch tag must serve a plain manifest")
}
