---
title: Shared Disk Cache for Local Processes
weight: 40
---

# Shared Disk Cache for Local Processes

A pattern for when several independent processes on one machine share a
single rate-limited or expensive resource (a live API, a slow local query)
and need to avoid duplicating that work — without a daemon, a database, or
a hand-rolled coordination protocol.

Two rules, applied to one plain file:

```
before fetching → is the cache fresh enough? → yes: read it, skip the fetch
                                              → no:  flock, fetch, write, unlock
```

That's the whole mechanism. No peer discovery, no leader election, no PID
files.

---

## Why not detect "is another instance running"?

The instinct is often to ask "is a sibling process already active, and
what is it doing?" — that's more state than the problem needs. A process
only needs one answer: *do I need to do the expensive thing right now?*
File age answers that directly, symmetrically, whether there are zero,
one, or ten other processes running. It's self-healing for free: a crashed
writer just leaves a file that ages out on its own — there is no "who owns
this" state to reconcile after a crash.

## The write race, and why a lock beats a hand-rolled check

Cache-aside (above) tells a process *whether* to fetch; it says nothing
about what happens when two processes both decide to fetch at once (both
saw a stale cache) and are now both about to write. Two failure modes:

- a slower response can clobber a faster one already on disk
- both processes do the expensive fetch when one would have done

The tempting fix is a manual **check-fetch-check-write** protocol: check
freshness before fetching, re-check before writing to avoid clobbering
something fresher, read back after writing to detect being clobbered
anyway. Worth naming why this is the wrong tool even though it can be made
to work:

- it's an approximation of optimistic concurrency control (the same idea
  as an HTTP `If-Match` or a DB version-check-then-update), but real OCC
  needs the compare-and-the-write to be *one* atomic operation. Plain
  files don't offer that — checking and writing are separate syscalls with
  a gap between them (TOCTOU), which a timestamp comparison can narrow but
  not close.
- mtime resolution can be too coarse to tell two near-simultaneous writers
  apart, so even the read-back step isn't fully reliable.
- it ends up being *more* code, for a weaker guarantee, than the standard
  tool built for exactly this job.

**Use `flock()` instead.** Hold it only around the fetch-and-write critical
section:

```go
lockFd, err := os.OpenFile(cachePath+".lock", os.O_CREATE|os.O_RDWR, 0o644)
// ... bounded, non-blocking retry:
for i := 0; i < maxRetries; i++ {
    if err := syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
        break // got it
    }
    time.Sleep(retryDelay)
    // ran out of retries → proceed without the lock: fetch live, skip the
    // disk write (a sibling almost certainly holds it and is writing its
    // own fresh copy right now)
}
defer syscall.Flock(int(lockFd.Fd()), syscall.LOCK_UN)
// fetch, then atomic write (temp file + rename), then the deferred unlock
```

`flock` fully closes the race a manual check can only narrow, and it's
less code, not more:

- **released automatically on any process exit** — normal return, crash,
  panic, SIGKILL, OOM-kill. The kernel closes the fd and drops the lock.
  No PID-file, no stale-lock detection, no cleanup logic to write.
- **does not survive a hang** — a deadlocked-but-alive holder keeps the
  lock. This is why the wait must be bounded (`LOCK_NB` + a short retry
  budget, not `LOCK_EX` blocking indefinitely): a waiter that can't get the
  lock quickly should just do the fetch itself and skip writing, not stall.
- **doesn't need to survive a reboot** — it doesn't, but nothing needs it
  to; whatever held it is gone too.
- **advisory only** — it coordinates cooperating processes, not a security
  boundary. Fine for this use case; wrong tool if you need to stop an
  uncooperative process from touching the file.

Still use temp-file-plus-rename for the write itself — that part doesn't
need the lock's help (`rename` is already atomic on the same filesystem),
but doing both together is simplest and avoids readers ever observing a
torn write during the short window the lock is held.

---

## When this pattern fits

- multiple invocations of the same CLI, or several related CLIs, on one
  machine, sharing one external rate-limited resource
- the "expensive thing" is idempotent and safe to occasionally duplicate
  (losing the race just means one wasted fetch, not corrupted state)
- staleness on the order of the fetch's natural refresh interval is
  acceptable to callers

## When it doesn't

- coordination needs to span multiple machines (use a real datastore /
  distributed lock instead)
- the write must never be duplicated even under a race (this pattern
  tolerates rare double-fetches by design; it doesn't forbid them, it just
  makes them rare and harmless)
- you need to know *who* holds the resource, not just *whether* it's fresh
  (that's a different, harder problem — this pattern deliberately avoids it)

## Related

- [harnez, `internal/usage/claude.go`](../../internal/usage/claude.go) — first production use of this pattern, coordinating multiple `harnez usage --watch` processes against Anthropic's rate-limited quota API
- [Issue 033](../../issues/033-usage-shared-quota-cache.md) — the issue that worked through this design, including the rejected check-fetch-check-write alternative
- [docs/studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md](../studies/2026-08-18-usage-quota-fetch-robustness-and-shared-cache.md) — full case study and the design conversation that led here
