# 114 — Remote Load stream drops to batch shortly after connecting: stdin-EOF false-triggers shutdown

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [[110-remote-load-batch-vs-streaming-collection-modes]], `internal/usage/loadstream.go`

## Problem

Observed live by the user running `harnez usage --watch` (with `load.watch_host`
configured) after the issue-110 streaming label landed: the `[R] Remote Load` box
shows `streaming` briefly, then drops to `batch` — the fallback-and-retry path
(`runRemoteLoadManager`, `watch.go`) is triggering almost immediately after a
stream connects, not because of a real network/host problem.

## Root cause (file:line)

`StartRemoteLoadStream` (`internal/usage/loadstream.go:188-244`) builds the local
`ssh -S <ctl> host "harnez load-stream"` child command and wires only its stdout:

```go
cmd := sshLoadStreamChildCmd(ctx, cm.ctlPath, host)
stdout, err := cmd.StdoutPipe()
```

`cmd.Stdin` is never set. Per Go's `os/exec` docs, a nil `Cmd.Stdin` connects
the child's stdin to `/dev/null`. SSH forwards that (already-EOF) stdin over
the channel to the remote command, so the remote `harnez load-stream` process
(`RunLoadStream`, `loadstream.go:37-73`) sees stdin EOF almost immediately:

```go
go func() {
    defer close(stdinClosed)
    buf := make([]byte, 1)
    for {
        if _, err := in.Read(buf); err != nil {
            return   // fires almost instantly against a /dev/null-backed stdin
        }
    }
}()
...
select {
case <-ctx.Done():
    return nil
case <-stdinClosed:
    return nil   // treated as "parent tore down, exit cleanly"
...
```

`RunLoadStream`'s own doc comment (`loadstream.go:31-33`) describes stdin EOF as
meaning "the local `--watch` session tore down its ControlMaster and killed
this child" — a real signal for *intentional* shutdown. But nothing on the
local side ever intentionally closes/holds open the child's stdin to encode
that signal; it's `/dev/null` from the start, so the remote process reads this
false "shutdown" signal within moments of the stream starting, self-terminates
(a clean, deliberate `return nil`, not a crash), the channel closes, and
`runRemoteLoadManager` (`watch.go:1858-1882`) correctly does exactly what it's
supposed to when a stream ends: falls back to batch polling and retries after
`remoteLoadRetryInterval` (10s) — which is why the box briefly shows
`streaming` and then reverts.

This is not a real network/host issue and not a bug in the fallback logic
itself — the fallback is working exactly as designed against an input
(false stdin-EOF) that shouldn't be happening in the first place.

## Suggested fix direction

Give the local `ssh` child a live stdin that only closes when the local side
actually intends to shut the stream down — e.g. an `io.Pipe()` whose write end
`StartRemoteLoadStream` holds open for the life of the stream and only closes
from within `stop()` (alongside the existing `cmd.Process.Kill()` /
`cm.Close()` calls). That makes stdin-EOF the deliberate, intentional shutdown
signal `RunLoadStream`'s doc comment already describes, instead of an
accidental artifact of an unset `Cmd.Stdin` defaulting to `/dev/null`.

Note `stop()` already calls `cmd.Process.Kill()` directly, which independently
terminates the remote-visible ssh child regardless of stdin state — so
`RunLoadStream`'s stdin-EOF path is somewhat redundant with that as the
*primary* teardown mechanism already in place. Whether to keep stdin-EOF as a
belt-and-suspenders secondary signal (fixed per above) or drop reliance on it
entirely in favor of `Kill()` alone (simplifying `RunLoadStream` by removing
the stdin-watching goroutine) is a judgment call for whoever picks this up —
either resolves the false-trigger bug, but they leave different code behind.

## Acceptance Criteria

1. A `harnez usage --watch` session against a real, stable remote host
   establishes streaming and *stays* in `streaming` mode (not just a brief
   flash) for as long as the connection is genuinely healthy — verified by
   watching the `[R]` box's label over at least 60+ seconds of stable
   connectivity, not just the first few samples.
2. The remote `harnez load-stream` process only exits when it should: real
   SIGINT/SIGTERM, an actual dropped SSH connection, or the local `--watch`
   session's own intentional teardown (`stop()`) — not a false stdin-EOF
   immediately after starting.
3. Existing tests in `loadstream_test.go` / `remoteloadmanager_test.go`
   updated or extended to cover the fix (e.g. a test asserting the remote
   process does NOT exit within some window after start when nothing has
   intentionally torn it down).
4. No regression to the "no zombie" guarantee — the remote process must
   still terminate promptly on real teardown (Ctrl-C, network loss), covered
   by existing zombie-check verification from issue 110.
5. `go test ./...` / `make check` pass; live-verify against a real host
   (`um760` is reachable in this dev environment) that `streaming` is now
   stable rather than flapping.
