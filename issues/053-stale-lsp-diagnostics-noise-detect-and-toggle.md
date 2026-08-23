# 053 — Background LSP diagnostics post stale/wrong findings; harness should detect and offer to disable per-agent

**Status**: Open
**Category**: Agentic Ergonomics / Harness Behavior
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md); observed in and drawn from sibling project `weg`'s session retrospective `docs/feedback/2026-08-23-hardening-automation-and-a-production-incident.md` (not linkable across repos — see that file directly in the `weg` checkout)

---

## 1. Problem & Motivation

Observed repeatedly in a `weg` session (2026-08-23, Go codebase): after
`Edit` tool calls, Claude Code's own background Go-language-server
integration automatically posted `<system-reminder>` diagnostic blocks
into the conversation — without being invoked — reporting compiler errors
that did not reflect real `go build`/`go vet` state. Two recurring
patterns:

- **Cross-file rename lag**: renaming an exported symbol in file A (e.g.
  `occCmd` → `OCCCmd`), then immediately editing file B to call the new
  name, produced an `undefined: nextcloud.OCCCmd` diagnostic on file B —
  the LSP server hadn't reindexed file A's rename yet.
- **Stale-reference lag**: diagnostics referencing code that had been
  fixed several edits/turns earlier (a field type change from `int` to
  `*int`, resolved turns before) kept reappearing on later, unrelated
  edits to the same file.

Every instance was independently caught by re-running the real toolchain
(`go build ./... && go vet ./... && gofmt -l .`) before trusting or acting
on the diagnostic — so no incorrect action resulted this session. But it
happened often enough (at least 5 separate times in one session) that it's
a real tax on trust and turn count: each occurrence requires an explicit
"let me verify with the real toolchain" detour, and a less careful agent
session (or one under different verification discipline) could plausibly
act on a false positive — reverting correct code, or reporting a build as
broken when it isn't.

## 2. What's being requested (three related asks, priority not yet decided)

1. **Investigate whether this specific harness's background LSP
   integration can simply be disabled** for sessions/projects where its
   diagnostics are proving net-negative (noisy relative to their hit
   rate) — at minimum document how, if a toggle already exists.
2. **The harness should detect whether a Core Language Server (or
   language-specific LSP) is enabled or disabled** for a given agent/
   session, surface that state somewhere inspectable (status output,
   session metadata), rather than it being an invisible background
   behavior the agent only discovers by observing false positives.
3. **Decide what the harness does with that state** — options, not yet
   narrowed to one:
   - Emit a one-time warning/notice when LSP diagnostics disagree with a
     subsequently-run real toolchain check (a concrete, checkable signal
     of staleness) — this is naturally where an agent already re-verifies
     after being surprised, so the harness could observe the same
     disagreement itself.
   - Let a project or session opt out of background LSP diagnostics
     entirely, defaulting to whatever's least surprising.
   - Do nothing structurally, just document the "always verify with the
     real toolchain, never trust the diagnostic block alone" discipline
     more prominently (e.g. in `docs/practices/`), on the theory that the
     existing discipline already caught every instance and the tool has
     enough true-positive value to keep as-is.

This ticket intentionally does not pick one of these — that's a design
decision for whoever picks this up, informed by how often the false-
positive pattern recurs across other sibling projects, not just `weg`.

## 3. Notes

- This is a harness-level behavior (Claude Code's own background tooling),
  not something `weg` or any other managed project can fix from its own
  side — hence filing here rather than in the sibling project.
- The failure mode is specifically about *staleness* (diagnostics lagging
  the actual current file state after a burst of edits), not about the
  LSP being wrong in principle — worth distinguishing from a general
  "LSP integration is unreliable" claim, which isn't what was observed.
