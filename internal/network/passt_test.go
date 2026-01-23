package network

import (
	"testing"
)

func TestHasPasst(t *testing.T) {
	// Just verify it doesn't panic
	// Result depends on system having passt installed
	_ = HasPasst()
}
