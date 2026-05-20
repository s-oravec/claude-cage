package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s-oravec/claude-cage/internal/registry"
)

func TestPushCmd_Args(t *testing.T) {
	c := NewPushCmd()
	assert.NotNil(t, c.Flag("latest"))
}

func TestPrintPushResult(t *testing.T) {
	const (
		manifestDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		targetDigest   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	)
	tests := []struct {
		name     string
		tagLabel string
		arch     string
		res      *registry.PutManifestResult
		want     string
	}{
		{
			name:     "manifest kind, latest not updated",
			tagLabel: "acme/widget:v1",
			arch:     "amd64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "manifest",
				TagTargetDigest: targetDigest,
				LatestUpdated:   false,
			},
			want: "Pushed: sha256:111111111111 (amd64)\n" +
				"Tag acme/widget:v1 -> manifest sha256:222222222222\n",
		},
		{
			name:     "index kind, latest not updated",
			tagLabel: "acme/widget:v1",
			arch:     "arm64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "index",
				TagTargetDigest: targetDigest,
				LatestUpdated:   false,
			},
			want: "Pushed: sha256:111111111111 (arm64)\n" +
				"Tag acme/widget:v1 -> index sha256:222222222222 (auto-composed by server)\n",
		},
		{
			name:     "manifest kind, latest updated",
			tagLabel: "acme/widget:v1",
			arch:     "amd64",
			res: &registry.PutManifestResult{
				ManifestDigest:  manifestDigest,
				TagTargetKind:   "manifest",
				TagTargetDigest: targetDigest,
				LatestUpdated:   true,
			},
			want: "Pushed: sha256:111111111111 (amd64)\n" +
				"Tag acme/widget:v1 -> manifest sha256:222222222222\n" +
				"Updated latest -> sha256:222222222222\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printPushResult(&buf, tt.tagLabel, tt.arch, tt.res)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
