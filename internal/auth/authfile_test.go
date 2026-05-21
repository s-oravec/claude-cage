package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_AddHostFull_RoundTrip(t *testing.T) {
	setRootForTest(t)
	exp := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	require.NoError(t, AddHostFull("cage-hub.io", "acc", "ref", "stiivo", exp))

	got, err := Load()
	require.NoError(t, err)
	e := got.Registries["cage-hub.io"]
	assert.Equal(t, "acc", e.Token)
	assert.Equal(t, "ref", e.RefreshToken)
	assert.Equal(t, "2026-05-21T12:00:00Z", e.ExpiresAt)
}

func TestAuth_AddHost_NoRefreshFields(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("h", "t", "u"))
	got, _ := Load()
	assert.Empty(t, got.Registries["h"].RefreshToken)
	assert.Empty(t, got.Registries["h"].ExpiresAt)
}

func setRootForTest(t *testing.T) string {
	d := t.TempDir()
	SetDir(d)
	t.Cleanup(func() { SetDir("") })
	return d
}

func TestAuth_NoFile_LoadEmpty(t *testing.T) {
	setRootForTest(t)
	auth, err := Load()
	require.NoError(t, err)
	assert.Empty(t, auth.Registries)
}

func TestAuth_SaveLoad_RoundTrip(t *testing.T) {
	setRootForTest(t)

	a := &Auth{Registries: map[string]Entry{
		"cage-hub.io": {Token: "ey...", Username: "stiivo", ObtainedAt: "2026-05-15T10:00:00Z"},
	}}
	require.NoError(t, Save(a))

	got, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "ey...", got.Registries["cage-hub.io"].Token)
}

func TestAuth_SaveIs0600(t *testing.T) {
	dir := setRootForTest(t)
	a := &Auth{Registries: map[string]Entry{"h": {Token: "t"}}}
	require.NoError(t, Save(a))

	info, err := os.Stat(filepath.Join(dir, "auth.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAuth_LoadWarnsOnLoosePerms(t *testing.T) {
	dir := setRootForTest(t)
	path := filepath.Join(dir, "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte("registries: {}\n"), 0o644))

	_, err := Load()
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "Load fixes permissions")
}

func TestAuth_AddRemoveHost(t *testing.T) {
	setRootForTest(t)

	require.NoError(t, AddHost("cage-hub.io", "ey...", "stiivo"))
	require.NoError(t, AddHost("cage-hub.local", "pat_...", "stiivo"))

	got, err := Load()
	require.NoError(t, err)
	assert.Len(t, got.Registries, 2)

	require.NoError(t, RemoveHost("cage-hub.io"))
	got, err = Load()
	require.NoError(t, err)
	assert.Len(t, got.Registries, 1)
}

func TestAuth_RemoveHost_Missing_NoError(t *testing.T) {
	setRootForTest(t)
	assert.NoError(t, RemoveHost("ghost.example"))
}

func TestAuth_Token_Helper(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("cage-hub.io", "ey...", "stiivo"))
	tok, ok := Token("cage-hub.io")
	assert.True(t, ok)
	assert.Equal(t, "ey...", tok)
}

func TestAuth_Logout_All(t *testing.T) {
	setRootForTest(t)
	require.NoError(t, AddHost("a", "t1", "u"))
	require.NoError(t, AddHost("b", "t2", "u"))
	require.NoError(t, RemoveAll())

	got, err := Load()
	require.NoError(t, err)
	assert.Empty(t, got.Registries)
}
