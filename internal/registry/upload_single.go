package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// uploadInitResp is the JSON body of POST /blobs/uploads.
type uploadInitResp struct {
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"`
}

// UploadBlobSinglePUT performs the two-phase Docker V2 single-PUT blob upload.
// Phase 1: POST /blobs/uploads with {digest} -> {upload_id, upload_url, expires_at}.
// Phase 2: PUT upload_url?digest=<digest> with the body bytes as octet-stream.
// Used for layers below the multipart-size threshold; for larger layers use
// UploadBlobMultipart for resumable uploads.
func (c *Client) UploadBlobSinglePUT(owner, name, digest string, body io.Reader) error {
	// Phase 1: init.
	initPath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name)
	initBody, _ := json.Marshal(map[string]string{"digest": digest})
	resp, err := c.do(http.MethodPost, initPath, initBody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	if resp.StatusCode != 202 {
		defer resp.Body.Close()
		return parseError(resp)
	}
	var init uploadInitResp
	if err := json.NewDecoder(resp.Body).Decode(&init); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	// Phase 2: PUT body.
	url := init.UploadURL + "?digest=" + digest
	req, err := http.NewRequest(http.MethodPut, c.resolveURL(url), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp2, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		return parseError(resp2)
	}
	return nil
}
