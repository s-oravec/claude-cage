package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
