package registry

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseError_PopulatesCode(t *testing.T) {
	body := `{"error":{"code":"CONFLICT_DIGEST_MISMATCH","message":"oops","details":{"want":"a","got":"b"}}}`
	err := parseError(&http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(body))})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "CONFLICT_DIGEST_MISMATCH", apiErr.Code)
	assert.Equal(t, 400, apiErr.HTTPStatus)
}

func TestParseError_NonJSONBody(t *testing.T) {
	err := parseError(&http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("oh no"))})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "UNEXPECTED", apiErr.Code)
}

func TestUserMessage_KnownCodes(t *testing.T) {
	cases := map[string]string{
		"UNAUTHORIZED":             "run `cage login",
		"FORBIDDEN":                "permission",
		"BLOB_MISSING":             "re-run push",
		"CONFLICT_DIGEST_MISMATCH": "digest",
		"UNKNOWN_BASE":             "base image",
		"UPLOAD_EXPIRED":           "expired",
		"UPLOAD_COMPLETED":         "already completed",
	}
	for code, frag := range cases {
		got := UserMessage(&APIError{Code: code})
		assert.Contains(t, got, frag)
	}
}

func TestClassifyTransport_Refused(t *testing.T) {
	wrapped := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	err := classifyTransport("localhost", wrapped)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "refused", te.Kind)
	assert.Contains(t, te.Error(), "Cannot connect to registry localhost")
	assert.Contains(t, te.Error(), "connection refused")
}

func TestClassifyTransport_DNS(t *testing.T) {
	err := classifyTransport("nope.example", &net.DNSError{Name: "nope.example"})
	var te *TransportError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "dns", te.Kind)
	assert.Contains(t, te.Error(), "host not found")
}

func TestClassifyTransport_Timeout(t *testing.T) {
	err := classifyTransport("h", timeoutErr{})
	var te *TransportError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "timeout", te.Kind)
	assert.Contains(t, te.Error(), "timed out")
}

func TestClassifyTransport_TLS(t *testing.T) {
	err := classifyTransport("h", errors.New("x509: certificate signed by unknown authority"))
	var te *TransportError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "tls", te.Kind)
	assert.Contains(t, te.Error(), "TLS handshake failed")
}

func TestClassifyTransport_NilPassThrough(t *testing.T) {
	assert.NoError(t, classifyTransport("h", nil))
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }
