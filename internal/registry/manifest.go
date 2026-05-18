package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PutManifestResult is the JSON response body for a successful manifest PUT.
type PutManifestResult struct {
	Tag            string `json:"tag"`
	ManifestDigest string `json:"manifest_digest"`
	LatestUpdated  bool   `json:"latest_updated"`
}

// GetManifest fetches the canonical manifest JSON for repos/<owner>/<name>:<tag>.
// Returns the body bytes and the server-reported Docker-Content-Digest.
func (c *Client) GetManifest(owner, name, tag string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/manifests/%s", owner, name, tag)
	resp, err := c.do(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", parseError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Docker-Content-Digest"), nil
}

// PutManifest uploads a canonical manifest JSON for repos/<owner>/<name>:<tag>.
// If asLatest is true, sets X-As-Latest: true so the server upserts the latest
// tag pointer in addition to <tag>.
func (c *Client) PutManifest(owner, name, tag string, body []byte, asLatest bool) (*PutManifestResult, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/manifests/%s", owner, name, tag)
	headers := map[string]string{"Content-Type": "application/vnd.cage.manifest.v1+json"}
	if asLatest {
		headers["X-As-Latest"] = "true"
	}
	resp, err := c.do(http.MethodPut, path, body, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, parseError(resp)
	}
	var out PutManifestResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
