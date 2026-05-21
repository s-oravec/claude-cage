package progress

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestGroupPlainMode(t *testing.T) {
	var buf bytes.Buffer
	g := NewGroupPlain(&buf) // plain mode forced (no TTY)

	b1 := g.AddBar(1000, "sha256:aaaa", "uploading")
	b1.SetCurrent(500)
	b1.SetCurrent(1000)
	b1.Done("")

	b2 := g.AddBar(2000, "sha256:bbbb", "uploading")
	b2.Done("exists")

	g.Wait()

	out := buf.String()
	if !strings.Contains(out, "sha256:aaaa: uploading done") {
		t.Errorf("missing finished line, got:\n%s", out)
	}
	if !strings.Contains(out, "sha256:bbbb: exists") {
		t.Errorf("missing skipped line, got:\n%s", out)
	}
}

func TestGroupPlainResetAfterRewind(t *testing.T) {
	var buf bytes.Buffer
	g := NewGroupPlain(&buf)
	b := g.AddBar(100, "sha256:cccc", "uploading")
	b.SetCurrent(80)
	b.SetCurrent(0) // 401 rewind
	b.SetCurrent(100)
	b.Done("")
	g.Wait()
	if !strings.Contains(buf.String(), "sha256:cccc: uploading done") {
		t.Errorf("missing done line, got:\n%s", buf.String())
	}
}

// TestGroupTTYRendersNote drives the mpb (TTY) path against an in-memory
// writer. mpb still renders to a non-terminal writer (just without cursor
// ANSI), so the label and the completion note should surface in the buffer.
func TestGroupTTYRendersNote(t *testing.T) {
	var buf bytes.Buffer
	g := newGroup(&buf, true)

	b := g.AddBar(100, "sha256:aaaa", "uploading")
	b.SetCurrent(100)
	b.Done("")

	g.Wait()

	out := buf.String()
	if out == "" {
		t.Fatal("expected non-empty TTY buffer, got empty")
	}
	if !strings.Contains(out, "sha256:aaaa") {
		t.Errorf("missing label in TTY output, got:\n%s", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("missing completion note %q in TTY output, got:\n%s", "done", out)
	}
}

// TestGroupTTYUnknownTotalDone exercises the unknown-total Done branch
// (SetTotal(-1, true)) on the mpb path, which the plain-mode tests never hit.
func TestGroupTTYUnknownTotalDone(t *testing.T) {
	var buf bytes.Buffer
	g := newGroup(&buf, true)

	bar := g.AddBar(0, "sha256:eeee", "downloading") // unknown total
	bar.Done("")

	g.Wait()

	if buf.Len() == 0 {
		t.Fatal("expected non-empty TTY buffer for unknown-total Done, got empty")
	}
}

// TestGroupConcurrentBars runs many bars concurrently on the mpb path. The
// real assertion is that it stays clean under -race; mpb bar ops are
// goroutine-safe and the note holder + plain mutex guard the rest.
func TestGroupConcurrentBars(t *testing.T) {
	var buf bytes.Buffer
	g := newGroup(&buf, true)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			bar := g.AddBar(1000, fmt.Sprintf("sha256:%04d", i), "uploading")
			bar.SetCurrent(250)
			bar.SetCurrent(500)
			bar.SetCurrent(1000)
			bar.Done("")
		}(i)
	}
	wg.Wait()

	g.Wait()
}
