package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
)

// APIError is the typed error returned by registry client methods when the
// server responds with a non-success status. Wraps the standard cage-hub
// {error: {code, message, details}} envelope plus the HTTP status code.
type APIError struct {
	HTTPStatus int            `json:"-"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d): %s", e.Code, e.HTTPStatus, e.Message)
}

// parseError builds an APIError from a non-2xx HTTP response, then closes the body.
// Non-JSON bodies become APIError{Code: "UNEXPECTED", Message: <raw body>}.
func parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var env struct {
		Error APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code == "" {
		return &APIError{HTTPStatus: resp.StatusCode, Code: "UNEXPECTED", Message: string(body)}
	}
	env.Error.HTTPStatus = resp.StatusCode
	return &env.Error
}

// TransportError is returned when the HTTP transport fails before the server
// can respond (DNS failure, connection refused, TLS handshake error, timeout).
// Distinct from APIError, which represents a server-returned non-2xx status.
type TransportError struct {
	Host string
	Kind string // "refused", "dns", "tls", "timeout", "other"
	Err  error  // underlying transport error, preserved for debugging
}

func (e *TransportError) Error() string {
	switch e.Kind {
	case "refused":
		return fmt.Sprintf("Cannot connect to registry %s: connection refused (is cage-hub running?)", e.Host)
	case "dns":
		return fmt.Sprintf("Cannot connect to registry %s: host not found", e.Host)
	case "tls":
		return fmt.Sprintf("Cannot connect to registry %s: TLS handshake failed: %v", e.Host, e.Err)
	case "timeout":
		return fmt.Sprintf("Cannot connect to registry %s: connection timed out", e.Host)
	default:
		return fmt.Sprintf("Cannot connect to registry %s: %v", e.Host, e.Err)
	}
}

func (e *TransportError) Unwrap() error { return e.Err }

// classifyTransport wraps a transport-level error (returned by http.Client.Do)
// into a TransportError with a kind suitable for human-friendly display.
// Returns nil if err is nil.
func classifyTransport(host string, err error) error {
	if err == nil {
		return nil
	}
	kind := "other"
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		kind = "refused"
	case isDNSError(err):
		kind = "dns"
	case isTimeout(err):
		kind = "timeout"
	case isTLSError(err):
		kind = "tls"
	}
	return &TransportError{Host: host, Kind: kind, Err: err}
}

func isDNSError(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func isTLSError(err error) bool {
	// Standard library returns a *tls.RecordHeaderError or wraps an
	// x509.UnknownAuthorityError; both surface "tls:" or "x509:" in their
	// rendered message. Cheap substring check avoids importing crypto/tls
	// just for type identity.
	msg := err.Error()
	return strings.Contains(msg, "tls:") || strings.Contains(msg, "x509:")
}

// UserMessage maps a server error code to an actionable hint suitable for
// printing to a human. Falls back to the server-supplied Message if the code
// is unknown.
func UserMessage(e *APIError) string {
	switch e.Code {
	case "UNAUTHORIZED":
		return "not authorized - run `cage login <host>` and try again"
	case "FORBIDDEN":
		return "no permission for this operation - check namespace ownership or collaborator role"
	case "BLOB_MISSING":
		return "server lost or never received a layer; re-run push"
	case "CONFLICT_DIGEST_MISMATCH":
		return "uploaded blob digest did not match the server's computation; this is a client or transport bug"
	case "UNKNOWN_BASE":
		return "the base image in the manifest is not on the server's whitelist"
	case "UPLOAD_EXPIRED":
		return "upload session expired (24h TTL); re-run push"
	case "UPLOAD_COMPLETED":
		return "upload already completed; nothing to abort"
	case "UPLOAD_ABORTED":
		return "upload was aborted; start a fresh push"
	default:
		return e.Message
	}
}
