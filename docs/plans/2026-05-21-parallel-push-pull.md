# Parallel push/pull with multi-bar progress + ETA - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Parallelize layer transfer in `cage push` and registry `cage pull`, with a docker-push-style multi-bar live progress display showing bytes, speed, and ETA per layer.

**Architecture:** A bounded `errgroup` worker pool replaces the serial per-layer loops in `runPush` and `runRegistryPull`. A rewritten `internal/progress` package wraps `vbauerster/mpb/v8` (one bar per layer) with a plain line-based fallback when stdout is not a TTY. The registry upload path gains an optional cumulative-bytes `ProgressFunc`; the pull path wraps the `GetBlob` reader at the cmd layer.

**Tech Stack:** Go, cobra, `github.com/vbauerster/mpb/v8`, `golang.org/x/sync/errgroup`, `golang.org/x/term`.

**Design doc:** `docs/plans/2026-05-21-parallel-push-pull-design.md`

**Conventions for the executor:**
- Run `go build ./...` and `go test ./internal/...` after each task; both MUST stay green. (The `test/e2e` suite needs pulled base images / VMs and fails in this environment - ignore it.)
- TDD: write the failing test first, watch it fail, implement, watch it pass, commit.
- Every commit message ends with the Co-Authored-By trailer used in this repo.
- ASCII only in code comments and docs (no em-dash, no arrow glyphs).

---

## Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Fetch the modules**

```bash
go get github.com/vbauerster/mpb/v8@latest
go get golang.org/x/sync/errgroup@latest
go get golang.org/x/term@latest
go mod tidy
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: success, `go.mod` now lists `vbauerster/mpb/v8`, `golang.org/x/sync`, `golang.org/x/term`.

**Step 3: Note the real decorator API**

Run: `go doc github.com/vbauerster/mpb/v8/decor | head -60`
Expected: confirms identifier names used in Task 2 (`CountersKibiByte`, `AverageSpeed`, `AverageETA`, `UnitKiB`, `ET_STYLE_GO`, `Any`, `OnComplete`). If a name differs in the resolved version, adjust Task 2 accordingly.

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add mpb, errgroup, x/term deps for parallel transfer"
```

---

## Task 2: New progress Group/Bar API (added alongside the old Bar)

Build the new API in a new file so the existing `Bar` (still used by `pull.go`) keeps the build green. The old `Bar` is removed in Task 6.

**Files:**
- Create: `internal/progress/group.go`
- Create: `internal/progress/group_test.go`

**Step 1: Write the failing test (plain / non-TTY mode is deterministic)**

`internal/progress/group_test.go`:

```go
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
	// Plain mode only prints terminal lines; just assert no panic and the done line.
	if !strings.Contains(buf.String(), "sha256:cccc: uploading done") {
		t.Errorf("missing done line, got:\n%s", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/progress/ -run TestGroup -v`
Expected: FAIL - `NewGroupPlain` / `AddBar` undefined.

**Step 3: Implement `internal/progress/group.go`**

```go
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
// across worker goroutines (mpb bars are goroutine-safe; plain mode guards
// writes with a mutex).
type Group struct {
	p     *mpb.Progress // nil in plain mode
	out   io.Writer
	plain bool
	mu    sync.Mutex // serializes plain-mode writes
}

// NewGroup returns a Group for out. If out is not a terminal it runs in plain
// mode. Detection: out must be an *os.File whose fd is a terminal.
func NewGroup(out io.Writer) *Group {
	if isTerminal(out) {
		return &Group{p: mpb.New(mpb.WithOutput(out)), out: out}
	}
	return NewGroupPlain(out)
}

// NewGroupPlain forces plain mode regardless of out (used by tests).
func NewGroupPlain(out io.Writer) *Group { return &Group{out: out, plain: true} }

func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// AddBar registers a bar. total<=0 means unknown size. label is the short
// digest; verb is "uploading" or "downloading".
func (g *Group) AddBar(total int64, label, verb string) *Bar {
	b := &Bar{group: g, total: total, label: label, verb: verb}
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
			decor.OnComplete(decor.AverageSpeed(decor.UnitKiB, "% .1f"), ""),
			decor.OnComplete(decor.AverageETA(decor.ET_STYLE_GO), ""),
			// Trailing dynamic note ("", "done", "exists", "cached").
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

func (n *noteHolder) get() string   { n.mu.Lock(); defer n.mu.Unlock(); return n.s }
func (n *noteHolder) set(s string)  { n.mu.Lock(); n.s = s; n.mu.Unlock() }

// Bar is a single layer's progress handle.
type Bar struct {
	group *Group
	bar   *mpb.Bar // nil in plain mode
	note  *noteHolder
	total int64
	label string
	verb  string
}

// SetCurrent sets the absolute number of bytes transferred so far. A smaller
// value than before (e.g. a 401 rewind to 0) is honored.
func (b *Bar) SetCurrent(n int64) {
	if b.bar != nil {
		b.bar.SetCurrent(n)
	}
}

// IncrBy advances by n bytes.
func (b *Bar) IncrBy(n int) {
	if b.bar != nil {
		b.bar.IncrBy(n)
	}
}

// Done completes the bar. note is an optional trailing word ("exists",
// "cached"); empty means a normal finish ("done").
func (b *Bar) Done(note string) {
	if b.bar != nil {
		shown := note
		if shown == "" {
			shown = "done"
		}
		b.note.set(" " + shown)
		// Complete the bar (fills it; triggers OnComplete decorators).
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/progress/ -run TestGroup -v`
Expected: PASS.

**Step 5: Verify whole package + build**

Run: `go build ./... && go test ./internal/progress/`
Expected: PASS (old `Bar` tests still pass too).

**Step 6: Commit**

```bash
git add internal/progress/group.go internal/progress/group_test.go
git commit -m "feat(progress): mpb-backed Group/Bar with non-TTY plain fallback"
```

---

## Task 3: ProgressFunc callback in the registry upload path

**Files:**
- Modify: `internal/registry/client.go:141-147` (`UploadBlob`)
- Modify: `internal/registry/upload_single.go:23` (`UploadBlobSinglePUT`)
- Modify: `internal/registry/upload_multipart.go:36` (`UploadBlobMultipart`)
- Modify (pass `nil`): `internal/cmd/push.go:110`
- Modify (add `, nil`): `internal/registry/upload_single_test.go` (4 calls), `internal/registry/upload_multipart_test.go` (1 call), `internal/registry/upload_dispatcher_test.go` (all `UploadBlob` calls)
- Test: `internal/registry/upload_progress_test.go` (new)

**Step 1: Write the failing test**

`internal/registry/upload_progress_test.go` - assert monotonic cumulative progress for both modes and that the final value equals the blob size. Model it on the existing stub-server tests in `upload_single_test.go` / `upload_multipart_test.go` (reuse their `httptest.Server` setup). Skeleton:

```go
package registry

import (
	"strings"
	"testing"
)

func TestUploadBlobSinglePUT_ReportsProgress(t *testing.T) {
	// ... same stub server as TestUploadBlobSinglePUT_TwoPhase ...
	var last int64
	var calls int
	cb := func(uploaded int64) {
		if uploaded < last {
			t.Fatalf("progress went backwards: %d < %d", uploaded, last)
		}
		last = uploaded
		calls++
	}
	err := c.UploadBlobSinglePUT("s", "d", "sha256:abc", 5, strings.NewReader("layer"), cb)
	if err != nil { t.Fatal(err) }
	if last != 5 { t.Errorf("final progress = %d, want 5", last) }
	if calls == 0 { t.Error("callback never invoked") }
}

func TestUploadBlobMultipart_ReportsProgress(t *testing.T) {
	// ... same stub server as TestUploadBlobMultipart_HappyPath, 10-byte blob ...
	var last int64
	cb := func(uploaded int64) {
		if uploaded < last { t.Fatalf("backwards: %d<%d", uploaded, last) }
		last = uploaded
	}
	err := c.UploadBlobMultipart("s", "d", "sha256:abc", 10, strings.NewReader("abcdefghij"), cb)
	if err != nil { t.Fatal(err) }
	if last != 10 { t.Errorf("final progress = %d, want 10", last) }
}

func TestUploadBlob_NilProgressIsNoop(t *testing.T) {
	// ... single-PUT stub ...; passing nil must not panic ...
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/ -run ReportsProgress -v`
Expected: FAIL - too many arguments / undefined `ProgressFunc`.

**Step 3: Implement the callback**

In `internal/registry/client.go`, add the type and thread it through the dispatcher:

```go
// ProgressFunc reports cumulative bytes transferred for a single blob upload.
// It may be nil.
type ProgressFunc func(uploaded int64)

func (c *Client) UploadBlob(owner, name, digest string, size, partSize int64, body io.Reader, onProgress ProgressFunc) error {
	if SelectUploadMode(size, partSize) == "multipart" {
		return c.UploadBlobMultipart(owner, name, digest, size, body, onProgress)
	}
	return c.UploadBlobSinglePUT(owner, name, digest, size, body, onProgress)
}
```

In `internal/registry/upload_single.go`, add `onProgress ProgressFunc` to the
signature and wrap the body in a counting+seeking reader before the PUT:

```go
func (c *Client) UploadBlobSinglePUT(owner, name, digest string, size int64, body io.Reader, onProgress ProgressFunc) error {
	// ... phase 1 unchanged ...

	// Phase 2: wrap body so reads report cumulative progress; preserve Seeker
	// (the 401 refresh-retry rewinds to 0) and reset the counter on seek.
	var rdr io.Reader = body
	if onProgress != nil {
		rdr = &progressReader{r: body, seeker: asSeeker(body), cb: onProgress}
	}
	seeker, _ := rdr.(io.Seeker)
	resp2, err := c.putWithRefresh(c.resolveURL(uploadURL), rdr, seeker)
	// ... rest unchanged ...
}

func asSeeker(r io.Reader) io.Seeker { s, _ := r.(io.Seeker); return s }

// progressReader counts bytes read and reports the running total. It forwards
// Seek to the underlying seeker and resets the counter to the new offset, so a
// rewind-and-retry replays progress from that point.
type progressReader struct {
	r      io.Reader
	seeker io.Seeker
	cb     ProgressFunc
	n      int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	m, err := p.r.Read(b)
	if m > 0 {
		p.n += int64(m)
		p.cb(p.n)
	}
	return m, err
}

func (p *progressReader) Seek(offset int64, whence int) (int64, error) {
	if p.seeker == nil {
		return 0, fmt.Errorf("progressReader: underlying body is not seekable")
	}
	pos, err := p.seeker.Seek(offset, whence)
	if err == nil {
		p.n = pos
		p.cb(p.n)
	}
	return pos, err
}
```

Note: `putWithRefresh`'s seek path keys off whether the body is an `io.Seeker`.
When the original body is seekable, `progressReader` is too (it forwards Seek),
so the retry behavior is preserved; when it is not, `progressReader` reports a
non-seekable error only if a retry is actually attempted, which matches the
prior "non-seekable cannot retry" behavior. Confirm
`TestUploadBlobSinglePUT_RefreshAndRetryOn401` still passes.

In `internal/registry/upload_multipart.go`, add `onProgress ProgressFunc` and,
after each successful part PUT (right after the 200/201 check, before/with the
`completed = append(...)`), advance and report:

```go
		completed = append(completed, completedPart{N: n, Etag: etag})
		uploaded += int64(len(chunk))
		if onProgress != nil {
			onProgress(uploaded)
		}
```

(declare `var uploaded int64` before the loop.)

**Step 4: Update existing call sites to pass `nil`**

- `internal/cmd/push.go:110`: append `, nil` for now (real callback wired in Task 4).
- `internal/registry/upload_single_test.go`: 4 `UploadBlobSinglePUT(...)` calls -> add `, nil`.
- `internal/registry/upload_multipart_test.go`: 1 `UploadBlobMultipart(...)` call -> add `, nil`.
- `internal/registry/upload_dispatcher_test.go`: every `UploadBlob(...)` call -> add `, nil`.

**Step 5: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS, including the new progress tests and the existing 401-retry test.

**Step 6: Build all**

Run: `go build ./...`
Expected: success (push.go compiles with the `, nil`).

**Step 7: Commit**

```bash
git add internal/registry/ internal/cmd/push.go
git commit -m "feat(registry): optional ProgressFunc on blob upload (single + multipart)"
```

---

## Task 4: Parallelize push with progress + -j flag

**Files:**
- Modify: `internal/cmd/push.go` (`NewPushCmd`, `runPush`)
- Test: `internal/cmd/push_test.go`

**Step 1: Write the failing test**

Add a test that runs `runPush` against an `httptest.Server` fake registry with
several layers (one already-present -> HEAD 200; the rest -> upload), output to
a `bytes.Buffer` (non-TTY -> plain lines), `concurrency=3`, and asserts: all
missing layers were PUT, the present one shows `exists`, and a forced error on
one layer makes `runPush` return non-nil. Reuse manifest/imgstore test helpers
already used by other cmd tests. (If `runPush` is hard to drive directly,
factor the layer-transfer loop into a helper `pushLayers(out, rc, ref, layers, concurrency) error`
and test that.)

```go
func TestPushLayers_ParallelTransfersAllAndReportsExists(t *testing.T) { ... }
func TestPushLayers_FirstErrorPropagates(t *testing.T) { ... }
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cmd/ -run TestPushLayers -v`
Expected: FAIL - helper undefined / serial signature.

**Step 3: Implement**

Add the flag in `NewPushCmd`:

```go
var concurrency int
// ...
c.Flags().IntVarP(&concurrency, "concurrency", "j", 3, "Max layers to upload in parallel")
// pass concurrency into runPush(...)
```

Rewrite the transfer loop in `runPush` (replacing `push.go:95-115`) with an
errgroup pool driving a `progress.Group`:

```go
import (
	"context"
	"golang.org/x/sync/errgroup"
	"github.com/s-oravec/claude-cage/internal/progress"
)

if concurrency < 1 {
	concurrency = 1
}
pg := progress.NewGroup(out)
g, _ := errgroup.WithContext(context.Background())
g.SetLimit(concurrency)
for _, l := range m.Layers {
	l := l
	g.Go(func() error {
		exists, err := rc.HeadBlob(ref.Owner, ref.Name, l.Digest)
		if err != nil {
			return err
		}
		bar := pg.AddBar(l.Size, shortDigest(l.Digest), "uploading")
		if exists {
			bar.Done("exists")
			return nil
		}
		f, err := os.Open(imgstore.LayerPath(l.Digest))
		if err != nil {
			return err
		}
		defer f.Close()
		if err := rc.UploadBlob(ref.Owner, ref.Name, l.Digest, l.Size,
			info.MultipartPartSize, f, func(u int64) { bar.SetCurrent(u) }); err != nil {
			return err
		}
		bar.Done("")
		return nil
	})
}
err = g.Wait()
pg.Wait() // shut down the live region before printing the manifest result
if err != nil {
	return err
}
```

The manifest PUT + `printPushResult` stay after this block, unchanged.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cmd/ -run TestPushLayers -v`
Expected: PASS.

**Step 5: Build + full cmd tests + lint the flag**

Run: `go build ./... && go test ./internal/cmd/`
Run: `go run . push --help` (sanity-check `-j, --concurrency` shows up)
Expected: PASS; flag visible.

**Step 6: Commit**

```bash
git add internal/cmd/push.go internal/cmd/push_test.go
git commit -m "feat(push): parallel layer upload with per-layer progress and -j flag"
```

---

## Task 5: Parallelize registry pull with progress + -j flag

**Files:**
- Modify: `internal/cmd/pull.go` (`NewPullCmd`, `runRegistryPull`, layer loop at `pull.go:249-265`)
- Test: `internal/cmd/pull_test.go`

**Step 1: Write the failing test**

Mirror Task 4: a fake registry serving a multi-layer manifest + blobs; assert
all missing layers land in the local store, `cached` layers are reported, output
is plain (buffer), and an error on one layer aborts with a non-nil error.
Factor a helper `pullLayers(out, rc, ref, layers, concurrency) error` if direct
`runRegistryPull` driving is awkward.

**Step 2: Run to verify failure**

Run: `go test ./internal/cmd/ -run TestPullLayers -v`
Expected: FAIL - helper/flag undefined.

**Step 3: Implement**

Add the flag to `NewPullCmd` (`-j` / `--concurrency`, default 3), thread into
`runRegistryPull`. Replace the serial layer loop (`pull.go:249-265`) with:

```go
if concurrency < 1 {
	concurrency = 1
}
pg := progress.NewGroup(cmd.OutOrStdout())
g, _ := errgroup.WithContext(context.Background())
g.SetLimit(concurrency)
for _, l := range m.Layers {
	l := l
	g.Go(func() error {
		bar := pg.AddBar(l.Size, shortDigest(l.Digest), "downloading")
		if imgstore.HasLayer(l.Digest) {
			bar.Done("cached")
			return nil
		}
		err := imgstore.PutLayerStreamed(l.Digest, func(offset int64) (io.ReadCloser, error) {
			if offset > 0 {
				bar.SetCurrent(offset)
			}
			rc2, err := rc.GetBlob(ref.Owner, ref.Name, l.Digest, offset)
			if err != nil {
				return nil, err
			}
			return &progressReadCloser{rc: rc2, n: offset, bar: bar}, nil
		})
		if err != nil {
			return err
		}
		bar.Done("")
		return nil
	})
}
err := g.Wait()
pg.Wait()
if err != nil {
	return err
}
```

Add the small helper (new file `internal/cmd/progress_reader.go` or inside
pull.go):

```go
// progressReadCloser counts bytes read from a layer download and reports the
// running total (seeded with the resume offset) to a progress bar.
type progressReadCloser struct {
	rc  io.ReadCloser
	n   int64
	bar *progress.Bar
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
```

The base-image check + `WriteRef` + final "Pulled" line stay serial after
`g.Wait()`.

**Step 4: Run to verify pass**

Run: `go test ./internal/cmd/ -run TestPullLayers -v`
Expected: PASS.

**Step 5: Build + cmd tests**

Run: `go build ./... && go test ./internal/cmd/`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/cmd/pull.go internal/cmd/pull_test.go internal/cmd/progress_reader.go
git commit -m "feat(pull): parallel registry layer download with progress and -j flag"
```

---

## Task 6: Migrate base-image pull to Group, delete old Bar

**Files:**
- Modify: `internal/cmd/pull.go` (`pullImage`, `pull.go:121-140`)
- Delete: `internal/progress/bar.go`, `internal/progress/bar_test.go`

**Step 1: Migrate `pullImage`**

Replace the `progress.NewBar` usage with a one-bar Group:

```go
pg := progress.NewGroup(cmd.OutOrStdout())
var bar *progress.Bar
progressFn := func(written, total int64) {
	if bar == nil && total > 0 {
		bar = pg.AddBar(total, name, "downloading")
	}
	if bar != nil {
		bar.SetCurrent(written)
	}
}
status := func(msg string) {
	if bar != nil {
		bar.Done("")
	}
	pg.Wait()
	fmt.Fprintln(cmd.OutOrStdout(), msg)
}
return images.Setup(name, arch, progressFn, status)
```

(Confirm against the exact current closure shape at `pull.go:121-140`.)

**Step 2: Delete the old Bar**

```bash
git rm internal/progress/bar.go internal/progress/bar_test.go
```

**Step 3: Verify nothing else references the old API**

Run: `grep -rn "progress.NewBar\|\.Update(\|\.Finish()" internal/ --include=*.go`
Expected: no matches (all migrated to `NewGroup`/`AddBar`/`SetCurrent`/`Done`).

**Step 4: Build + full unit tests**

Run: `go build ./... && go test ./internal/...`
Expected: PASS across all internal packages.

**Step 5: Commit**

```bash
git add internal/cmd/pull.go internal/progress/
git commit -m "refactor(progress): migrate base-image pull to Group, drop old Bar"
```

---

## Task 7: Concurrency-stress test for the shared token provider

Guards the design assumption that `tokensrc.Refreshing` is safe to share across
parallel uploads.

**Files:**
- Test: `internal/tokensrc/tokensrc_test.go` (add a test)

**Step 1: Write the test**

```go
func TestRefreshing_ConcurrentTokenAccess(t *testing.T) {
	// Construct a Refreshing pointing at an auth entry that does NOT need
	// refresh (far-future ExpiresAt) so Token() just reads. Hammer Token()
	// from many goroutines; -race must stay clean.
	r := NewRefreshing("host", "client", "https://example/token")
	// ... seed a valid non-expiring auth entry via the same mechanism the
	//     existing tests use ...
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = r.Token() }()
	}
	wg.Wait()
}
```

(Match the existing test setup in `tokensrc_test.go` for seeding auth state.)

**Step 2: Run with the race detector**

Run: `go test ./internal/tokensrc/ -race -run Concurrent -v`
Expected: PASS, no race.

**Step 3: Commit**

```bash
git add internal/tokensrc/tokensrc_test.go
git commit -m "test(tokensrc): concurrent Token() access is race-free"
```

---

## Final verification

Run:
```bash
go build ./...
go test ./internal/... -race
go vet ./...
go run . push --help   # shows -j, --concurrency
go run . pull --help    # shows -j, --concurrency
```
Expected: all green; both commands document `-j`.

Manual smoke (optional, needs a real cage-hub login + a multi-layer image):
`cage push host/owner/name:tag` shows one live bar per uploading layer with
bytes / speed / ETA, already-present layers flash `exists`, and up to 3 upload
at once.

## Done criteria

- Push and registry pull transfer layers concurrently (default 3, `-j` to change).
- TTY: docker-style multi-bar with bytes + speed + ETA per layer.
- Non-TTY: stable plain lines (existing test style preserved).
- First error aborts the batch and is returned.
- `internal/progress` exposes only the new Group/Bar API; old Bar deleted.
- All `internal/...` tests pass under `-race`.
