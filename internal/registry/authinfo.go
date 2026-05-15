package registry

import (
	"encoding/json"
	"net/http"
)

// AuthInfo is the parsed body of GET /api/v1/auth/info.
type AuthInfo struct {
	Issuer                      string   `json:"issuer"`
	DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
	TokenEndpoint               string   `json:"token_endpoint"`
	ClientID                    string   `json:"client_id"`
	Scopes                      []string `json:"scopes"`
	PATFormat                   string   `json:"pat_format"`
	PATConsoleURL               string   `json:"pat_console_url"`
	SupportedLayerMediaTypes    []string `json:"supported_layer_media_types"`
	SupportedManifestMediaTypes []string `json:"supported_manifest_media_types"`
	MaxManifestSize             int64    `json:"max_manifest_size"`
	MaxLayerSize                int64    `json:"max_layer_size"`
	MultipartPartSize           int64    `json:"multipart_part_size"`
}

// AuthInfo fetches /api/v1/auth/info from the registry. Anonymous (no token needed).
func (c *Client) AuthInfo() (*AuthInfo, error) {
	resp, err := c.do(http.MethodGet, "/api/v1/auth/info", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var out AuthInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
