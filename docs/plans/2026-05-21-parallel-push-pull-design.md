# Parallel push/pull with multi-bar progress + ETA - design

Date: 2026-05-21

## Problem

`cage push` and registry `cage pull` transfer layers one at a time in a serial
loop, with no live progress:

- Push (`internal/cmd/push.go:96`) loops over `m.Layers`, HEADs each, and on a
  miss calls `rc.UploadBlob`. The only feedback is a single
  `uploading <N> bytes` line printed before the call blocks for the whole
  upload.
- Registry pull (`internal/cmd/pull.go:250`) loops over `m.Layers` and downloads
  each via `imgstore.PutLayerStreamed` + `rc.GetBlob`, printing only
  `downloading` / `cached` lines.

The user wants a standard CLI progress indicator with ETA, and - because an
image is a chain of layers (image N from N-1 from ... from a base) - wants all
layers visible at once (docker-push style) and the transfers parallelized for
real speedup.

There is an existing hand-rolled `internal/progress.Bar` (single bar, shows
bytes + speed, no ETA), wired only into base-image `pullImage`. It is being
retired in favor of a library.

## Decisions

- Library: `github.com/vbauerster/mpb/v8` (+ `/decor`). Multi-bar live region,
  one bar per layer, with bytes / speed / ETA decorators out of the box.
- Concurrency: bounded worker pool via `golang.org/x/sync/errgroup`
  (`g.SetLimit(N)`), default N=3, overridable with a `-j` / `--concurrency`
  flag on both `push` and `pull`.
- TTY detection: `golang.org/x/term`. When stdout is not a terminal
  (CI / pipe / tests), skip the live bars and print plain per-layer lines
  (today's behavior). Keeps logs and tests deterministic.
- Replace `internal/progress` everywhere (push, registry pull, and base-image
  `pullImage`), then delete the package (its only user is pull).

## Design

### 1. Progress abstraction (`internal/progress`, rewritten)

Rewrite the package as a thin wrapper over mpb so the cmd layer does not deal
with mpb/decor/tty details directly, and so the non-TTY fallback lives in one
place.

```go
// Group owns one mpb.Progress (or nil when not a TTY).
type Group struct { ... }

// NewGroup returns a Group writing to out. If out is not a terminal it runs in
// "plain" mode: AddBar returns a Bar that prints lines instead of drawing.
func NewGroup(out io.Writer) *Group

// AddBar registers a layer bar. total<=0 means unknown size (spinner-ish).
// label is the short digest. verb is "uploading"/"downloading".
func (g *Group) AddBar(total int64, label, verb string) *Bar

// Wait blocks until all bars finish (mpb shutdown) - no-op in plain mode.
func (g *Group) Wait()

type Bar interface {
    SetCurrent(n int64) // absolute bytes transferred so far
    IncrBy(n int)       // delta
    Done(note string)   // mark complete with a trailing note e.g. "exists"
}
```

Decorators (TTY mode): `decor.Name(label)`, `decor.CountersKiloByte`,
`decor.AverageSpeed(decor.UnitKiB, ...)`, and `decor.AverageETA(decor.ET_STYLE_GO)`
replaced by elapsed on completion. Plain mode prints
`  <label>: <verb> done` (and `: <note>` for skipped) on `Done`.

Why absolute `SetCurrent`: the single-PUT 401-retry rewinds the body to 0; an
absolute API lets the bar follow the rewind (back to 0, then climb) without a
fragile negative-delta path.

### 2. Push parallelization (`internal/cmd/push.go`)

Replace the serial loop with an errgroup pool:

```go
g, _ := errgroup.WithContext(ctx)
g.SetLimit(concurrency)
pg := progress.NewGroup(out)
for _, l := range m.Layers {
    l := l
    g.Go(func() error {
        exists, err := rc.HeadBlob(...)
        if err != nil { return err }
        bar := pg.AddBar(l.Size, shortDigest(l.Digest), "uploading")
        if exists { bar.Done("exists"); return nil }
        f, err := os.Open(imgstore.LayerPath(l.Digest))
        if err != nil { return err }
        defer f.Close()
        err = rc.UploadBlob(ref.Owner, ref.Name, l.Digest, l.Size,
            info.MultipartPartSize, f, func(uploaded int64) { bar.SetCurrent(uploaded) })
        if err != nil { return err }
        bar.Done("")
        return nil
    })
}
err := g.Wait()
pg.Wait()
```

The first error cancels the context; mpb is shut down via `pg.Wait()` after the
group returns so the terminal is left clean even on error.

### 3. Registry upload progress callback (`internal/registry`)

Add a `ProgressFunc` parameter threaded through the upload entry points. It
reports cumulative bytes transferred for this blob:

```go
type ProgressFunc func(uploaded int64) // cumulative; may be nil

func (c *Client) UploadBlob(owner, name, digest string, size, partSize int64,
    body io.Reader, onProgress ProgressFunc) error
```

- single-PUT: wrap `body` in a counting reader that calls `onProgress(total)`
  on each Read. The wrapper also implements `io.Seeker` (delegating to the
  underlying file) and resets its counter to the seek offset, so the existing
  401 refresh-and-retry rewind keeps working and the bar follows it.
- multipart: after each part PUT returns 2xx, add the chunk length to a running
  total and call `onProgress(total)`. Part-granular but accurate to the network
  (no jump-ahead from reading a part into the buffer before it is sent).

`onProgress == nil` preserves current behavior for callers/tests that do not
want progress.

### 4. Registry pull parallelization (`internal/cmd/pull.go`)

Same errgroup pool over `m.Layers`. No `registry` or `imgstore` signature
change: wrap the `GetBlob` ReadCloser in a counting reader inside the
`PutLayerStreamed` open closure, and on a resume (`offset > 0`) seed the bar
with `bar.SetCurrent(offset)`.

```go
g.Go(func() error {
    bar := pg.AddBar(l.Size, shortDigest(l.Digest), "downloading")
    if imgstore.HasLayer(l.Digest) { bar.Done("cached"); return nil }
    err := imgstore.PutLayerStreamed(l.Digest, func(offset int64) (io.ReadCloser, error) {
        if offset > 0 { bar.SetCurrent(offset) }
        rc2, err := rc.GetBlob(ref.Owner, ref.Name, l.Digest, offset)
        if err != nil { return nil, err }
        return countingReadCloser(rc2, offset, bar), nil
    })
    if err != nil { return err }
    bar.Done(""); return nil
})
```

The base-image check and ref write stay serial after `g.Wait()`.

### 5. Base-image pull (`internal/cmd/pull.go` pullImage)

`pullImage` keeps its single-file download but switches from
`internal/progress.Bar` to a one-bar `progress.Group` so the old package can be
deleted. The `images.Setup` callback `progress(written, total)` drives
`bar.SetCurrent(written)`.

### 6. Concurrency flag

Add `-j` / `--concurrency` (default 3) to both `NewPushCmd` and `NewPullCmd`,
plumbed into `runPush` / `runRegistryPull`. A value < 1 is clamped to 1.

### 7. Token provider under concurrency

`tokensrc.Refreshing` is already mutex-guarded (`Token`/`Refresh`), so
concurrent uploads sharing one `Client` serialize correctly on refresh. No
change needed; covered by a concurrency-stress test.

## Out of scope

- Parallelizing the parts within a single multipart upload (still serial).
- An aggregate "total across all layers" bar; per-layer bars are the
  docker-push convention and enough here.

## Testing

- `internal/progress`: plain-mode output (non-TTY) is deterministic; `Done`
  notes render; absolute `SetCurrent` after a reset works.
- `internal/registry`: `UploadBlob` with a stub server reports monotonic
  cumulative progress for both single and multipart; nil callback is a no-op;
  single-PUT 401 rewind re-reports from 0.
- `internal/cmd`: push/pull with concurrency>1 over a fake registry transfers
  all layers, surfaces the first error and cancels the rest, and respects the
  `-j` flag; non-TTY output stays line-based and stable.
- Concurrency-stress test that many goroutines sharing a `Refreshing` provider
  do not race (run with `-race`).
