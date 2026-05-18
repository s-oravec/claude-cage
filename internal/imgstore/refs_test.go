package imgstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef_FullyQualified(t *testing.T) {
	r, err := ParseRef("cage-hub.io/stiivo/devbox:v1")
	require.NoError(t, err)
	assert.True(t, r.IsRegistry())
	assert.Equal(t, "cage-hub.io", r.Host)
	assert.Equal(t, "stiivo", r.Owner)
	assert.Equal(t, "devbox", r.Name)
	assert.Equal(t, "v1", r.Tag)
}

func TestParseRef_DefaultsTagToLatest(t *testing.T) {
	r, err := ParseRef("cage-hub.io/stiivo/devbox")
	require.NoError(t, err)
	assert.Equal(t, "latest", r.Tag)
}

func TestParseRef_LocalName(t *testing.T) {
	r, err := ParseRef("myimage:v2")
	require.NoError(t, err)
	assert.False(t, r.IsRegistry())
	assert.Equal(t, "myimage", r.Name)
	assert.Equal(t, "v2", r.Tag)
}

func TestParseRef_LocalNameDefaultLatest(t *testing.T) {
	r, err := ParseRef("myimage")
	require.NoError(t, err)
	assert.False(t, r.IsRegistry())
	assert.Equal(t, "myimage", r.Name)
	assert.Equal(t, "latest", r.Tag)
}

func TestParseRef_RejectsTwoSegments(t *testing.T) {
	_, err := ParseRef("stiivo/devbox:v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host/owner/name:tag")
}

func TestParseRef_RejectsEmpty(t *testing.T) {
	_, err := ParseRef("")
	require.Error(t, err)
}

func TestRef_RefPath_Registry(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	r := Ref{Host: "cage-hub.io", Owner: "s", Name: "d", Tag: "v1"}
	assert.Equal(t, "/tmp/cc/refs/cage-hub.io/s/d/v1", r.RefPath())
}

func TestRef_RefPath_Local(t *testing.T) {
	SetRoot("/tmp/cc")
	defer SetRoot("")
	r := Ref{Name: "myimage", Tag: "v1"}
	assert.Equal(t, "/tmp/cc/refs/_local/myimage/v1", r.RefPath())
}
