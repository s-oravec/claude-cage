package registry

import (
	"io"
	"net/http"
	"strings"
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
