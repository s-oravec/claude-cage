package registry

import (
	"fmt"
	"io"
	"net/http"
)

// HeadBlob reports whether the named blob exists on the server.
// 200 -> true, 404 -> false, anything else -> error.
func (c *Client) HeadBlob(owner, name, digest string) (bool, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/%s", owner, name, digest)
	resp, err := c.do(http.MethodHead, path, nil, nil)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, parseError(resp)
	}
}

// GetBlob streams the named blob's body. Caller MUST Close the reader.
// If offset > 0 a Range header (bytes=<offset>-) is sent for resume.
func (c *Client) GetBlob(owner, name, digest string, offset int64) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/%s", owner, name, digest)
	headers := map[string]string{}
	if offset > 0 {
		headers["Range"] = fmt.Sprintf("bytes=%d-", offset)
	}
	resp, err := c.do(http.MethodGet, path, nil, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		defer resp.Body.Close()
		return nil, parseError(resp)
	}
	return resp.Body, nil
}
