package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestBar(t *testing.T) {
	var buf bytes.Buffer
	bar := NewBar(1000, "test", &buf)

	bar.Update(0)
	bar.Update(250)
	bar.Update(500)
	bar.Update(750)
	bar.Finish()

	output := buf.String()

	// Should contain progress indicators
	if !strings.Contains(output, "100%") {
		t.Error("should show 100% at finish")
	}
	if !strings.Contains(output, "█") {
		t.Error("should contain filled blocks")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536 * 1024, "1.5 MB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestBarUnknownTotal(t *testing.T) {
	var buf bytes.Buffer
	bar := NewBar(0, "test", &buf)

	bar.Update(1000)
	bar.Update(2000)

	output := buf.String()

	// Should still show something without crashing
	if output == "" {
		t.Error("should produce some output even with unknown total")
	}
}
