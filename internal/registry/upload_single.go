package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
)

// uploadInitResp is the JSON body of POST /blobs/uploads.
type uploadInitResp struct {
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"`
}

// UploadBlobSinglePUT performs the two-phase Docker V2 single-PUT blob upload.
// Phase 1: POST /blobs/uploads with {digest, size} -> {upload_id, upload_url, expires_at}.
// Phase 2: PUT upload_url?digest=<digest> with the body bytes as octet-stream.
// Used for layers below the multipart-size threshold; for larger layers use
// UploadBlobMultipart for resumable uploads.
func (c *Client) UploadBlobSinglePUT(owner, name, digest string, size int64, body io.Reader, onProgress ProgressFunc) error {
	// Phase 1: init.
	initPath := fmt.Sprintf("/api/v1/repos/%s/%s/blobs/uploads", owner, name)
	initBody, _ := json.Marshal(map[string]any{"digest": digest, "size": size})
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

	// Phase 2: PUT body. Parse upload_url and SET the digest query param
	// explicitly. The cage-hub server already bakes "?digest=<digest>" into
	// upload_url, but setting it ensures exactly one digest param regardless of
	// what the server returned: it overwrites a pre-baked value rather than
	// appending a second one ("...?digest=X?digest=X", which the server rejects
	// as querystring/digest Invalid) and adds it when the server omits it.
	// upload_url may be relative or absolute; neturl.Parse handles both, and
	// resolveURL still prepends the base for relative URLs.
	u, err := neturl.Parse(init.UploadURL)
	if err != nil {
		return fmt.Errorf("invalid upload_url from server: %w", err)
	}
	q := u.Query()
	q.Set("digest", digest) // exactly one digest param, regardless of what the server returned
	u.RawQuery = q.Encode()
	uploadURL := u.String()

	var rdr io.Reader = body
	if onProgress != nil {
		var sk io.Seeker
		if s, ok := body.(io.Seeker); ok {
			sk = s
		}
		rdr = &progressReader{r: body, seeker: sk, cb: onProgress}
	}
	seeker, _ := rdr.(io.Seeker)
	resp2, err := c.putWithRefresh(c.resolveURL(uploadURL), rdr, seeker)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		return parseError(resp2)
	}
	return nil
}

// putWithRefresh PUTs body as octet-stream with the bearer token. On a 401 it
// force-refreshes the token and, if the body is seekable, rewinds and retries
// once. A non-seekable body cannot be replayed, so the 401 is returned as-is
// (proactive refresh in Token() makes this path unlikely).
func (c *Client) putWithRefresh(url string, body io.Reader, seeker io.Seeker) (*http.Response, error) {
	resp, err := c.putOnce(url, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && seeker != nil {
		resp.Body.Close()
		if _, rerr := c.provider.Refresh(); rerr != nil {
			return nil, rerr
		}
		if _, serr := seeker.Seek(0, io.SeekStart); serr != nil {
			return nil, serr
		}
		return c.putOnce(url, body)
	}
	return resp, nil
}

// progressReader counts bytes read and reports the running total. It forwards
// Seek to the underlying seeker and resets the counter to the new offset, so a
// rewind-and-retry replays progress from that point.
type progressReader struct {
	r      io.Reader
	seeker io.Seeker
	cb     ProgressFunc
	n      int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	m, err := p.r.Read(b)
	if m > 0 {
		p.n += int64(m)
		p.cb(p.n)
	}
	return m, err
}

func (p *progressReader) Seek(offset int64, whence int) (int64, error) {
	if p.seeker == nil {
		return 0, fmt.Errorf("progressReader: underlying body is not seekable")
	}
	pos, err := p.seeker.Seek(offset, whence)
	if err == nil {
		p.n = pos
		p.cb(p.n)
	}
	return pos, err
}

func (c *Client) putOnce(url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if tok, _ := c.provider.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return c.transport(req)
}
