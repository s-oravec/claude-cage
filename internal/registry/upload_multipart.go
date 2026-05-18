package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type multipartInitResp struct {
	UploadID         string `json:"upload_id"`
	PartSize         int64  `json:"part_size"`
	PartCount        int    `json:"part_count"`
	PartsURLTemplate string `json:"parts_url_template"`
	ExpiresAt        string `json:"expires_at"`
}

type partURLResp struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

type completedPart struct {
	N    int    `json:"n"`
	Etag string `json:"etag"`
}

// UploadBlobMultipart drives a per-part presigned-PUT multipart blob upload.
// Phase 1: POST /blobs/uploads?multipart=true {digest, size} -> upload_id + part metadata.
// Phase 2: For each part 1..N: GET per-part presigned URL, PUT chunk, collect ETag.
// Phase 3: POST .../complete {parts:[{n,etag}]}.
// Selected for layers above the multipart threshold; provides resumability and
// parallelism (though current implementation is serial).
func (c *Client) UploadBlobMultipart(owner, name, digest string, size int64, body io.Reader) error {
	// Init.
	initPath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads?multipart=true", owner, name)
	initBody, _ := json.Marshal(map[string]any{"digest": digest, "size": size})
	resp, err := c.do(http.MethodPost, initPath, initBody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	if resp.StatusCode != 202 {
		defer resp.Body.Close()
		return parseError(resp)
	}
	var init multipartInitResp
	if err := json.NewDecoder(resp.Body).Decode(&init); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	completed := make([]completedPart, 0, init.PartCount)
	buf := make([]byte, init.PartSize)
	for n := 1; n <= init.PartCount; n++ {
		// Read one part of bytes from body.
		readSize, err := io.ReadFull(body, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		chunk := buf[:readSize]

		// Get presigned URL for this part.
		urlPath := strings.Replace(init.PartsURLTemplate, "{n}", fmt.Sprintf("%d", n), 1)
		uresp, err := c.do(http.MethodGet, urlPath, nil, nil)
		if err != nil {
			return err
		}
		if uresp.StatusCode != 200 {
			defer uresp.Body.Close()
			return parseError(uresp)
		}
		var pu partURLResp
		if err := json.NewDecoder(uresp.Body).Decode(&pu); err != nil {
			uresp.Body.Close()
			return err
		}
		uresp.Body.Close()

		// PUT directly to presigned URL.
		req, err := http.NewRequest(http.MethodPut, c.resolveURL(pu.URL), bytes.NewReader(chunk))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		presp, err := c.transport(req)
		if err != nil {
			return err
		}
		etag := presp.Header.Get("ETag")
		presp.Body.Close()
		if presp.StatusCode != 200 && presp.StatusCode != 201 {
			return fmt.Errorf("part %d upload failed: HTTP %d", n, presp.StatusCode)
		}
		completed = append(completed, completedPart{N: n, Etag: etag})
	}

	// Complete.
	completePath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads/%s/complete", owner, name, init.UploadID)
	cbody, _ := json.Marshal(map[string]any{"parts": completed})
	cresp, err := c.do(http.MethodPost, completePath, cbody, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer cresp.Body.Close()
	if cresp.StatusCode != 200 && cresp.StatusCode != 201 {
		return parseError(cresp)
	}
	return nil
}
