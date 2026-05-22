package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/s-oravec/cage/internal/registry"
)

// printAPIErrorHint inspects err for *registry.APIError and writes a friendly
// hint to stderr. Always returns err so callers can chain it from RunE.
func printAPIErrorHint(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *registry.APIError
	if errors.As(err, &apiErr) {
		fmt.Fprintln(os.Stderr, registry.UserMessage(apiErr))
	}
	return err
}
