# 182 — `harnez distill` Should Truncate Very Long Output With an Explicit Note

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/178](178-distill-smart-mode-error-pattern-preservation.md) (smart-mode error pattern preservation)

---

## 1. Problem & Motivation

`harnez distill` currently has no explicit cap on output length for pathologically large tool
output (e.g. a build log with thousands of repeated warning lines, a huge JSON dump). Unbounded
output defeats the purpose of distillation (keeping noisy output out of agent context) and can
blow context budgets exactly in the cases distillation exists to prevent.

## 2. Technical Specification / Findings

Needs research before implementation, specifically:

- Whether truncating with a visible marker (e.g. `... [truncated N lines / M bytes] ...`) risks
  confusing agents into thinking the truncation marker itself is meaningful output, or into
  retrying the same command expecting different (untruncated) results.
- How comparable tools handle this. Named for comparison: RTK (Anthropic's internal/eval tool
  reference in this space) and other CLI-output-shaping tools — research what truncation-notice
  conventions they use (position of the note — head, tail, or both; exact wording; whether they
  offer a way to fetch the untruncated original).
- Whether the note should be machine-parseable (a fixed sentinel string) so agents can reliably
  detect "this was truncated, don't treat it as complete" versus a human-readable free-text note.

## 3. Implementation & Verification Plan

1. Research phase: survey how 2-3 comparable tools (RTK and others identified during research)
   signal truncation in tool output; write findings into this ticket's Technical Specification
   section before writing code.
2. Add a configurable max-output cap to `harnez distill` (length in bytes or lines); when
   exceeded, truncate and append a clearly-delimited note stating what was cut and by how much.
3. Decide truncation placement (keep the head, the tail, or both with the middle elided) based on
   what's more useful for typical distilled content (likely: keep both ends for build/test logs
   where the final error matters most).
4. Add a unit test asserting the note is present and the byte/line count is respected.
5. Verify with `go test ./...`; run `make install`.

## 4. Research Findings

Web search for comparable tool-output-truncation conventions (queried 2026-09-01/02; "RTK" as
named in the original ticket text could not be identified as a public tool matching this
description — treated as an internal/informal reference rather than a verifiable source, so the
research instead surveyed the actual comparable tools that turned up):

- **OpenAI Codex** truncates tool output at hardcoded limits (10 KiB / 256 lines) using a
  **head+tail** strategy — not head-only or tail-only.
- A documented UTF-8-safe truncation taxonomy (from a Codex-adjacent extension) names four
  strategies: `head`, `tail`, `middle` (byte-based elision), and `middle_lines` (splits on
  newlines and takes whole lines from head and tail, specifically to avoid landing mid-line and
  emitting a broken partial line at the cut boundary).
- **Gemini CLI** uses a free-text note style: `"...(output truncated - X more lines)"`.
- One MCP-adjacent truncation note format seen in the wild is fully explicit about both sides of
  the cut: `"[Tool output truncated] Original: 128,430 characters Returned excerpts: characters
  1-19,000 and 109,431-128,430 Omitted: 90,430 characters"` — i.e. exact counts on every side, not
  just "how much was cut."
- Claude Code's MCP tool-output path allows servers to opt individual results out of truncation
  entirely via an `anthropic/maxResultSizeChars` annotation (durable up to 500,000 characters) —
  i.e. truncation is a default a specific caller can waive, not a universal hard rule.

Sources: [pi-mono #134](https://github.com/badlogic/pi-mono/issues/134),
[codex #14206](https://github.com/openai/codex/issues/14206),
[pi-extensions/tool-output-truncation.md](https://github.com/qduc/pi-extensions/blob/main/tool-output-truncation.md),
[codex #6544](https://github.com/openai/codex/issues/6544),
[gemini-cli #12172/#12173](https://github.com/google-gemini/gemini-cli/pull/12173).

**Decisions made from this research:**

- **Placement: head + tail, middle elided** (not head-only, not tail-only). For build/test log
  content specifically, the invoked command and early setup/compile errors live at the head, and
  the final failure/summary line lives at the tail; both carry signal, only the noisy middle
  (repeated warnings, passing-test spam) doesn't. This matches Codex's approach and harnez's
  pre-existing `FilterHeadTail` (line-based cap, already shipped) rather than requiring a new
  placement policy.
- **Line-safety**: the new byte-based cap rounds its cut points to the nearest newline
  (`middle_lines`-style) rather than cutting mid-line, so the truncated output never ends in a
  broken partial line.
- **Sentinel, not free text**: the note starts with a fixed, grep-able prefix
  (`[harnez-distill:truncated`) followed by exact `<omitted bytes> bytes omitted, <kept> of <total>
  total bytes kept, <cap>-byte cap` counts — closer to the fully-explicit MCP-style note above than
  Gemini CLI's vaguer "-X more lines" phrasing, on the theory that an agent deciding whether to
  retry a command benefits from knowing the omitted-to-total ratio, not just "something was cut."
  The sentinel is a constant (`truncationSentinel` in `internal/distill/distill.go`) so any caller
  can reliably detect "this was truncated" by substring match, addressing the ticket's "risk of
  confusing agents into retrying" concern directly — the note is unambiguous and machine-checkable
  rather than prose an agent might mistake for real output.
- **No separate "fetch the untruncated original" mechanism** was added — out of scope for this
  ticket's minimal v1; `--max-bytes 0` (or a larger `--max-bytes`) disables/raises the cap for a
  caller that wants the full output, which is the same opt-out shape as Claude Code's
  `maxResultSizeChars` annotation, just simpler.

## 5. Resolution Note

Implemented as an additional, independent cap layered on top of the pre-existing line-based
`FilterHeadTail` (which already covered "too many lines" with a head/tail note — see
`internal/distill/distill.go`). The gap this ticket closes is specifically the case a line cap
can't catch: a handful of pathologically long lines (a giant JSON blob, one huge stack trace)
that blow the byte budget without ever exceeding a line-count threshold.

Changes:

- `internal/distill/distill.go`: added `Options.MaxBytes`, `FilterHeadTailBytes(s, maxBytes)`
  (head+tail-by-bytes, newline-aligned cut points, fixed `truncationSentinel` note with exact
  omitted/kept/total/cap byte counts), and wired it into `Distill()` after the existing
  `FilterHeadTail` line-based pass — the byte cap runs last as a hard backstop, regardless of
  mode or line-count settings.
- `cmd/harnez/distill.go`: added a `--max-bytes` flag (default `200_000`, `0` disables), plumbed
  into `distill.Options.MaxBytes` the same way `--max-lines` already was.
- `internal/distill/distill_test.go`: added `TestFilterHeadTailBytes`,
  `TestFilterHeadTailBytes_UnderLimit`, `TestFilterHeadTailBytes_Disabled`, and
  `TestDistill_MaxBytesCap`, asserting the sentinel is present, head/tail content survives, the
  byte cap is respected, and the disabled (`0`) case is a no-op.

Verification: `go build ./...` and `go test ./...` pass (all packages, including the new tests).
`make install` run.

No scope was trimmed — the byte cap was implementable as a small, self-contained addition next to
the existing line cap rather than requiring a distill architecture change, so the "downgrade to a
research-only finding" fallback allowed by the ticket wasn't needed.
