package imgstore

import (
	"os"
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
