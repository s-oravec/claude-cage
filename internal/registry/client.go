package registry

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"
)

// TokenProvider supplies bearer tokens for registry requests. Token() returns a
// currently-valid token (refreshing proactively if near expiry); Refresh()
// forces a refresh and is called after a 401.
type TokenProvider interface {
	Token() (string, error)
	Refresh() (string, error)
}

// staticToken is the fallback provider for a fixed token (anonymous or a caller
// that passed Options.Token). It cannot refresh, so Refresh returns the same
// token: a 401 then surfaces unchanged after a single harmless retry.
type staticToken string

func (s staticToken) Token() (string, error)   { return string(s), nil }
func (s staticToken) Refresh() (string, error) { return string(s), nil }

// Options configure NewClient.
type Options struct {
	Token         string        // Bearer token attached to every request (omit for anonymous)
	TokenProvider TokenProvider // optional; overrides Token when set
	Insecure      bool          // Use plain HTTP and skip TLS verification (local dev only)
}

// Client is the HTTP client for a single cage-hub registry host.
type Client struct {
	baseURL  string
	host     string
	provider TokenProvider
	hc       *http.Client
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
	provider := opt.TokenProvider
	if provider == nil {
		provider = staticToken(opt.Token)
	}
	return &Client{
		baseURL:  scheme + "://" + host,
		host:     host,
		provider: provider,
		hc: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
	}, nil
}

// SetTokenProvider swaps the token provider after construction. Used by callers
// that learn the OAuth endpoints from /auth/info before building a refreshing
// provider.
func (c *Client) SetTokenProvider(p TokenProvider) { c.provider = p }

// transport executes req and converts any transport-level failure into a
// *TransportError so callers see a human-friendly message instead of a raw
// dial/TLS/DNS error string.
func (c *Client) transport(req *http.Request) (*http.Response, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, classifyTransport(c.host, err)
	}
	return resp, nil
}

// resolveURL returns target as-is if it is already absolute (starts with http://
// or https://), otherwise prepends c.baseURL. Used for upload/redirect URLs that
// the server may return either as paths or as full URLs (e.g. presigned storage
// URLs).
func (c *Client) resolveURL(target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return c.baseURL + target
}

// do issues an HTTP request with bearer token (if set) and merges extra headers.
// On a 401 it forces a token refresh and retries the request exactly once.
// Caller MUST close resp.Body.
func (c *Client) do(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	resp, err := c.doOnce(method, path, body, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if _, rerr := c.provider.Refresh(); rerr != nil {
			return nil, rerr
		}
		return c.doOnce(method, path, body, headers)
	}
	return resp, nil
}

// doOnce sends a single request with the current bearer token. Body is []byte,
// so it can be replayed across a retry.
func (c *Client) doOnce(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	var br io.Reader
	if body != nil {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.resolveURL(path), br)
	if err != nil {
		return nil, err
	}
	if tok, _ := c.provider.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.transport(req)
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
		return c.UploadBlobMultipart(owner, name, digest, size, body)
	}
	return c.UploadBlobSinglePUT(owner, name, digest, size, body)
}
