package imgstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGetLayer_RoundTrip(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"
	require.NoError(t, PutLayerBytes(digest, []byte("hello")))

	got, err := GetLayerBytes(digest)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)

	assert.True(t, HasLayer(digest))
	assert.False(t, HasLayer("sha256:ffff"+
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
}

func TestPutGetManifest_RoundTrip(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	body := []byte(`{"schemaVersion":1}`)
	require.NoError(t, PutManifestBytes(digest, body))

	got, err := GetManifestBytes(digest)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestPutRef_OverwritesIdempotently(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	r := Ref{Name: "myimage", Tag: "latest"}
	require.NoError(t, WriteRef(r, "sha256:aaaa"))
	require.NoError(t, WriteRef(r, "sha256:bbbb")) // overwrite OK

	got, err := ReadRef(r)
	require.NoError(t, err)
	assert.Equal(t, "sha256:bbbb", got)
}

func TestReadRef_NotFound(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	_, err := ReadRef(Ref{Name: "ghost", Tag: "latest"})
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteRef(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	r := Ref{Name: "myimage", Tag: "latest"}
	require.NoError(t, WriteRef(r, "sha256:aaaa"))
	require.NoError(t, DeleteRef(r))
	_, err := ReadRef(r)
	require.Error(t, err)
}

func TestPutLayer_FsyncedAtomicRename(t *testing.T) {
	// White-box: PutLayerBytes must not leave a tmp file alongside the final.
	SetRoot(t.TempDir())
	defer SetRoot("")

	const digest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	require.NoError(t, PutLayerBytes(digest, []byte("x")))

	dir := filepath.Dir(LayerPath(digest))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp")
	}
}

func TestHashFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "x")
	require.NoError(t, err)
	_, err = f.WriteString("hello")
	require.NoError(t, err)
	f.Close()

	got, err := HashFile(f.Name())
	require.NoError(t, err)
	// sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", got)
}

func TestMaterializeChain_SingleLayer(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed")
	}
	root := t.TempDir()
	SetRoot(root)
	defer SetRoot("")

	// Create a fake qcow2 layer.
	layerSrc := filepath.Join(t.TempDir(), "src.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", layerSrc, "1M").Run())

	// Compute its digest and store it under that digest.
	digest := layerDigestForFile(t, layerSrc)
	require.NoError(t, CopyFromFile(layerSrc, digest))

	// Construct a minimal manifest referencing one layer (use actual digest).
	manifest := []byte(`{"layers":[{"digest":"` + digest + `"}]}`)
	mdigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	require.NoError(t, PutManifestBytes(mdigest, manifest))

	// Create a fake base image.
	base := filepath.Join(t.TempDir(), "base.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	dst := filepath.Join(t.TempDir(), "materialized.qcow2")
	require.NoError(t, MaterializeChain(mdigest, base, dst))

	// Verify the materialized file exists and has correct backing file.
	out, err := exec.Command("qemu-img", "info", "--output=json", dst).Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), base, "rebased backing path should be the base image")
}

// helper
func layerDigestForFile(t *testing.T, path string) string {
	t.Helper()
	d, err := HashFile(path)
	require.NoError(t, err)
	return d
}

func TestPutLayerStreamed_FreshDownload(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	payload := []byte("hello-layer-bytes")
	digest := "sha256:" + sha256Hex(payload)

	err := PutLayerStreamed(digest, func(offset int64) (io.ReadCloser, error) {
		require.Equal(t, int64(0), offset)
		return io.NopCloser(bytes.NewReader(payload)), nil
	})
	require.NoError(t, err)
	assert.True(t, HasLayer(digest))

	got, err := GetLayerBytes(digest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestPutLayerStreamed_ResumesFromPartial(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")

	payload := []byte("hello-layer-bytes")
	digest := "sha256:" + sha256Hex(payload)

	// Simulate a previous failed download that wrote 6 bytes.
	if err := ensureDir(LayerPath(digest)); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, os.WriteFile(LayerPath(digest)+".partial", payload[:6], 0o644))

	var seenOffset int64
	err := PutLayerStreamed(digest, func(offset int64) (io.ReadCloser, error) {
		seenOffset = offset
		return io.NopCloser(bytes.NewReader(payload[offset:])), nil
	})
	require.NoError(t, err)
	assert.Equal(t, int64(6), seenOffset, "fetch should have been asked for offset 6")
	assert.True(t, HasLayer(digest))
	got, _ := GetLayerBytes(digest)
	assert.Equal(t, payload, got)
}

func TestPutLayerStreamed_ExistingLayerNoop(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	digest := "sha256:0000000000000000000000000000000000000000000000000000000000000001"
	require.NoError(t, PutLayerBytes(digest, []byte("x")))

	called := false
	err := PutLayerStreamed(digest, func(int64) (io.ReadCloser, error) {
		called = true
		return nil, fmt.Errorf("should not be called")
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestPutLayerStreamed_DigestMismatch_DeletesPartial(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	badDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000099"

	err := PutLayerStreamed(badDigest, func(int64) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("not the real bytes"))), nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "digest mismatch")

	_, statErr := os.Stat(LayerPath(badDigest) + ".partial")
	assert.True(t, os.IsNotExist(statErr), "partial should be deleted on digest mismatch")
}

// helper - sha256 hex of bytes
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
