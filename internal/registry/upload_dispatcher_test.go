package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectUploadMode(t *testing.T) {
	const part = int64(64 * 1024 * 1024) // 64 MiB
	tests := []struct {
		size int64
		want string
	}{
		{1, "single"},
		{4*part - 1, "single"},
		{4 * part, "multipart"},
		{10 * part, "multipart"},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tc.want, SelectUploadMode(tc.size, part))
		})
	}
}
