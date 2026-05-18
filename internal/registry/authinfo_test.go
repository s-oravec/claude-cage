package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthInfo_ReturnsParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/info", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                         "https://kc.example/realms/cage-hub",
			"device_authorization_endpoint":  "https://kc.example/.../auth/device",
			"token_endpoint":                 "https://kc.example/.../token",
			"client_id":                      "cage-cli",
			"scopes":                         []string{"openid", "profile"},
			"pat_format":                     "cgh_<base64url>",
			"pat_console_url":                "https://h/settings/tokens",
			"supported_layer_media_types":    []string{"application/vnd.cage.layer.v1.qcow2"},
			"supported_manifest_media_types": []string{"application/vnd.cage.manifest.v1+json"},
			"max_manifest_size":              65536,
			"max_layer_size":                 21474836480,
			"multipart_part_size":            67108864,
		})
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL[len("http://"):], Options{Insecure: true})
	got, err := c.AuthInfo()
	require.NoError(t, err)
	assert.Equal(t, "cage-cli", got.ClientID)
	assert.Equal(t, int64(67108864), got.MultipartPartSize)
}
