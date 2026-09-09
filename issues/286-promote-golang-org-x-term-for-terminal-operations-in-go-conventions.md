# 286 — Promote `golang.org/x/term` for terminal operations in Go conventions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Instruction gap (performance/correctness risk from missing convention)
**Category**: Agentic Ergonomics / Go Conventions
**Related**: [issues/285](285-agent-must-not-claim-it-noted-something-without-writing-it-to-a-durable-file.md) (precedent: narrow instruction-text ticket, same shape), [docs/lang/Go.md](../docs/lang/Go.md) (source doc; synced to `docs/Go.md` and `~/.claude/docs/Go.md` via `config.yaml`)

---

## 1. Problem & Motivation

Live incident, 2026-09-08, sibling project `voxi` (Claude Code session):
`internal/monitor/render.go`'s `getTerminalWidth()` shelled out to
`stty -F /dev/tty size` (fork+exec a subprocess, parse its stdout) on every
screen redraw. When that redraw loop was bumped to 30fps for a live
mic-loudness meter, live measurement via `/proc/<pid>/stat` utime+stime
showed the `stty` subprocess spawning cost ~8.2% of a CPU core over a 5s
window — roughly 3x more expensive than all the actual mic-level audio
processing combined (~2.8%).

Fixed by switching to `golang.org/x/term.GetSize(fd)` (a direct
`TIOCGWINSZ` syscall, no subprocess) plus caching and refreshing only on
`SIGWINCH`. Re-measurement after the fix: the same cost dropped from 41
ticks to 1 tick over the same 5s window — ~7x total CPU reduction for the
whole watch-loop process. Committed in voxi as `6e53f3a`.

Notably, `golang.org/x/term` was **already a direct dependency** in
voxi's `go.mod` and already used elsewhere in that same repo for
`term.IsTerminal`, `term.MakeRaw`, `term.Restore` — but nobody reached
for it for the size query specifically; the `stty` subprocess pattern was
written first and never revisited.

**harnez has the identical pattern in its own code.** `internal/usage/watch.go`
already has the *correct* pattern for size queries — a hand-rolled direct
`TIOCGWINSZ` syscall in `ttySize()` (lines 271-282), with a comment
explicitly noting it's "more reliable than shelling out to
`stty -F /dev/tty size`" — but still falls back to exactly that `stty`
subprocess at line 312 when the direct syscall fails, and separately uses
`exec.Command("stty", ...)` for raw-mode enter/restore in
`RunWatchWithOptions` (lines 2527, 2529, 2531) instead of
`term.MakeRaw`/`term.Restore`. This is a second, independent concrete
instance of the same anti-pattern in a different codebase — exactly the
kind of thing a documented convention would catch during review, rather
than relying on someone asking "how expensive is this?" after the fact.

`golang.org/x/term` is **not currently a harnez dependency**
(`go.mod` requires `golang.org/x/sys` only, indirectly, via
`modernc.org/*`) — it would be a new (but very small, stdlib-adjacent,
zero-transitive-dep-beyond-x/sys) addition.

## 2. Technical Specification / Findings

- Amend `docs/lang/Go.md`'s **Language & Deps** section (the section that
  already carves out `cobra` for CLI and `yaml.v3` for config as
  pre-approved exceptions to "minimise external deps") with a matching
  carve-out: `golang.org/x/term` is the correct, pre-approved choice for
  terminal size queries, raw-mode, and `IsTerminal` checks — never shell
  out to `stty`/`tput`/similar CLI tools for these, and prefer it over a
  hand-rolled `syscall.TIOCGWINSZ` ioctl (harnez's own `ttySize()` is a
  reasonable *fallback-of-last-resort* pattern, but `x/term.GetSize` gets
  the same syscall with less bespoke unsafe-pointer code to maintain).
- Concrete phrasing to add to `## Language & Deps`, matching the existing
  bullet-list house style:
  ```
  - **Terminal I/O**: Use `golang.org/x/term` for terminal size queries
    (`term.GetSize`), raw-mode (`term.MakeRaw`/`term.Restore`), and
    is-a-terminal checks (`term.IsTerminal`) — never shell out to
    `stty`/`tput` (subprocess spawn cost dwarfs the syscall it wraps;
    measured ~3x the CPU of the actual work in a 30fps redraw loop) and
    prefer it over hand-rolled `TIOCGWINSZ` ioctl code.
  ```
- Remember `docs/lang/Go.md` is the *source*; `docs/Go.md` (local sync)
  and `~/.claude/docs/Go.md` (target) are generated copies per
  `config.yaml` lines 431-435 — edit only the source and resync via
  whatever this repo's existing doc-sync mechanism is (`make apply` /
  harnez's own bundled-docs sync), do not hand-edit the copies.

## 3. Implementation & Verification Plan

1. Add the bullet above (or equivalent, matched to house style) under
   `## Language & Deps` in `docs/lang/Go.md`.
2. Resync generated copies (`docs/Go.md`, `~/.claude/docs/Go.md`) via the
   project's existing bundled-docs sync mechanism; confirm all three
   copies match afterward (`diff docs/lang/Go.md docs/Go.md`).
3. `go test ./...` to confirm nothing else depends on doc content shape.
4. Update `issues/README.md` via `harnez index`.
5. Optional, separate follow-up (not in this ticket's scope): file a
   quick-fix ticket for `internal/usage/watch.go` itself — replace the
   `stty -F /dev/tty size` fallback at line 312 and the `stty`-based
   raw-mode enter/restore at lines 2527/2529/2531 with
   `golang.org/x/term` equivalents, now that the dependency would be
   pre-approved. Left as a candidate rather than filed here since this
   ticket is about the instruction, not the code fix.

## 4. Non-Goals (for now)

- Not fixing `internal/usage/watch.go`'s own `stty` subprocess calls in
  this ticket — see optional follow-up above.
- Not adding `golang.org/x/term` to `go.mod` in this ticket; that happens
  naturally the first time some code (harnez's own or a consumer
  project) actually imports it under the new convention.
- Not building lint/CI enforcement that greps for `exec.Command("stty"`
  — this is a documented-convention change, matching how prior
  Language & Deps entries (e.g. the cobra/yaml.v3 carve-outs) ship as
  static instruction text, not enforced tooling.
