package registry

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

// Options configure NewClient.
type Options struct {
	Token    string // Bearer token attached to every request (omit for anonymous)
	Insecure bool   // Use plain HTTP and skip TLS verification (local dev only)
}

// Client is the HTTP client for a single cage-hub registry host.
type Client struct {
	baseURL string
	token   string
	hc      *http.Client
}

// NewClient builds a Client for the given host. Insecure=true selects http://
// and disables TLS verification; otherwise https:// with full chain validation.
func NewClient(host string, opt Options) (*Client, error) {
	scheme := "https"
	tr := &http.Transport{}
	if opt.Insecure {
		scheme = "http"
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // localhost dev only
	}
	return &Client{
		baseURL: scheme + "://" + host,
		token:   opt.Token,
		hc: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
	}, nil
}

// do issues an HTTP request with bearer token (if set) and merges extra headers.
// Caller MUST close resp.Body.
func (c *Client) do(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	var br io.Reader
	if body != nil {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, br)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.hc.Do(req)
}

// SelectUploadMode returns "single" or "multipart" based on the C1 hybrid rule:
// layers below 4*partSize use single-PUT (no resume); larger use multipart.
func SelectUploadMode(size, partSize int64) string {
	if size < 4*partSize {
		return "single"
	}
	return "multipart"
}

// UploadBlob picks single or multipart based on size + partSize and uploads body.
func (c *Client) UploadBlob(owner, name, digest string, size, partSize int64, body io.Reader) error {
	if SelectUploadMode(size, partSize) == "multipart" {
		return c.UploadBlobMultipart(owner, name, digest, body)
	}
	return c.UploadBlobSinglePUT(owner, name, digest, body)
}
