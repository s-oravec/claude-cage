package progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Bar represents a progress bar
type Bar struct {
	Total       int64
	Width       int
	Description string
	Output      io.Writer

	current   int64
	startTime time.Time
	lastPrint time.Time
}

// NewBar creates a new progress bar
func NewBar(total int64, description string, output io.Writer) *Bar {
	return &Bar{
		Total:       total,
		Width:       40,
		Description: description,
		Output:      output,
		startTime:   time.Now(),
	}
}

// Update updates the progress bar
func (b *Bar) Update(current int64) {
	b.current = current

	// Throttle updates to max 10 per second
	if time.Since(b.lastPrint) < 100*time.Millisecond && current < b.Total {
		return
	}
	b.lastPrint = time.Now()

	b.render()
}

// Finish completes the progress bar
func (b *Bar) Finish() {
	b.current = b.Total
	b.render()
	fmt.Fprintln(b.Output)
}

func (b *Bar) render() {
	if b.Total <= 0 {
		fmt.Fprintf(b.Output, "\r  %s... %s", b.Description, formatBytes(b.current))
		return
	}

	percent := float64(b.current) / float64(b.Total)
	if percent > 1 {
		percent = 1
	}

	// Calculate speed
	elapsed := time.Since(b.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(b.current) / elapsed
	}

	// Build progress bar
	filled := int(percent * float64(b.Width))
	empty := b.Width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Format output
	fmt.Fprintf(b.Output, "\r  [%s] %3.0f%% %s / %s  %s/s    ",
		bar,
		percent*100,
		formatBytes(b.current),
		formatBytes(b.Total),
		formatBytes(int64(speed)),
	)
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
