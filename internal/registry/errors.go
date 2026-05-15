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
