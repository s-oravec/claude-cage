package progress

import (
	"bytes"
	"strings"
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
