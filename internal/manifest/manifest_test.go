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
