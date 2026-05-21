package cmd

import (
	"io"

	"github.com/s-oravec/claude-cage/internal/progress"
)

// progressReadCloser counts bytes read from a layer download and reports the
// running total (seeded with a resume offset) to a progress bar.
type progressReadCloser struct {
	rc  io.ReadCloser
	n   int64
	bar *progress.LayerBar
}

func (p *progressReadCloser) Read(b []byte) (int, error) {
	m, err := p.rc.Read(b)
	if m > 0 {
		p.n += int64(m)
		p.bar.SetCurrent(p.n)
	}
	return m, err
}

func (p *progressReadCloser) Close() error { return p.rc.Close() }
