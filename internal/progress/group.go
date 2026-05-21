package progress

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
)

// Group manages a set of per-layer progress bars. In TTY mode it drives an
// mpb live region; in plain mode (non-terminal: CI, pipes, tests) it prints
// one terminal line per bar on completion. A Group is safe for concurrent use
// across worker goroutines.
type Group struct {
	p     *mpb.Progress // nil in plain mode
	out   io.Writer
	plain bool
	mu    sync.Mutex // serializes plain-mode writes
}

// newGroup builds a Group; tty selects the mpb live-region path vs plain lines.
//
// WithAutoRefresh forces the render loop on regardless of whether out is a
// terminal. On a real TTY mpb already enables auto refresh, so this is a no-op
// there; it only matters for tests that drive the mpb path with an in-memory
// writer, where mpb would otherwise never flush a frame.
func newGroup(out io.Writer, tty bool) *Group {
	if tty {
		return &Group{p: mpb.New(mpb.WithOutput(out), mpb.WithAutoRefresh()), out: out}
	}
	return &Group{out: out, plain: true}
}

// NewGroup returns a Group for out. If out is not a terminal it runs in plain
// mode (detection: out is an *os.File whose fd is a terminal).
func NewGroup(out io.Writer) *Group { return newGroup(out, isTerminal(out)) }

// NewGroupPlain forces plain mode regardless of out (used by tests).
func NewGroupPlain(out io.Writer) *Group { return newGroup(out, false) }

func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// AddBar registers a bar. total<=0 means unknown size. label is the short
// digest; verb is "uploading" or "downloading".
func (g *Group) AddBar(total int64, label, verb string) *LayerBar {
	b := &LayerBar{group: g, total: total, label: label, verb: verb}
	if g.plain {
		return b
	}
	b.note = &noteHolder{}
	b.bar = g.p.AddBar(total,
		mpb.PrependDecorators(
			decor.Name(label+": "+verb+" "),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.CountersKibiByte("% .1f / % .1f"), ""),
			decor.OnComplete(decor.Name("  "), ""),
			decor.OnComplete(decor.AverageSpeed(decor.SizeB1024(0), "% .1f"), ""),
			decor.OnComplete(decor.AverageETA(decor.ET_STYLE_GO), ""),
			decor.Any(func(decor.Statistics) string { return b.note.get() }),
		),
	)
	return b
}

// Wait blocks until all TTY bars finish rendering. No-op in plain mode.
func (g *Group) Wait() {
	if g.p != nil {
		g.p.Wait()
	}
}

type noteHolder struct {
	mu sync.Mutex
	s  string
}

func (n *noteHolder) get() string  { n.mu.Lock(); defer n.mu.Unlock(); return n.s }
func (n *noteHolder) set(s string) { n.mu.Lock(); n.s = s; n.mu.Unlock() }

// LayerBar is a single layer's progress handle.
type LayerBar struct {
	group *Group
	bar   *mpb.Bar // nil in plain mode
	note  *noteHolder
	total int64
	label string
	verb  string
}

// SetCurrent sets the absolute number of bytes transferred so far. A smaller
// value than before (e.g. a 401 rewind to 0) is honored.
func (b *LayerBar) SetCurrent(n int64) {
	if b.bar != nil {
		b.bar.SetCurrent(n)
	}
}

// IncrBy advances by n bytes.
func (b *LayerBar) IncrBy(n int) {
	if b.bar != nil {
		b.bar.IncrBy(n)
	}
}

// Done completes the bar. note is an optional trailing word ("exists",
// "cached"); empty means a normal finish ("done").
func (b *LayerBar) Done(note string) {
	if b.bar != nil {
		shown := note
		if shown == "" {
			shown = "done"
		}
		b.note.set(" " + shown)
		if b.total > 0 {
			b.bar.SetCurrent(b.total)
		} else {
			b.bar.SetTotal(-1, true)
		}
		return
	}
	// Plain mode: one terminal line.
	shown := b.verb + " done"
	if note != "" {
		shown = note
	}
	b.group.mu.Lock()
	fmt.Fprintf(b.group.out, "  %s: %s\n", b.label, shown)
	b.group.mu.Unlock()
}
