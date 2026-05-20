package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexBody_RoundTrip(t *testing.T) {
	raw := []byte(`{"schemaVersion":1,"mediaType":"application/vnd.cage.index.v1+json","manifests":[
        {"digest":"sha256:a","platform":{"architecture":"amd64"}},
        {"digest":"sha256:b","platform":{"architecture":"arm64"}}
    ]}`)
	var idx IndexBody
	require.NoError(t, json.Unmarshal(raw, &idx))
	assert.Equal(t, 1, idx.SchemaVersion)
	assert.Equal(t, MediaTypeIndexV1, idx.MediaType)
	assert.Len(t, idx.Manifests, 2)
	assert.Equal(t, "amd64", idx.Manifests[0].Platform.Architecture)
}
