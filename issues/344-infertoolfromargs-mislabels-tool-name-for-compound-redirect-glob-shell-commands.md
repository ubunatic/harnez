# 344 — inferToolFromArgs mislabels tool_name for compound/redirect/glob shell commands

**Status**: Closed — resolved
**Priority**: P2
**Category**: telemetry

---

## Problem

The ubunatic.com telemetry dashboard (`telemetry/index.html`, backed by `telemetry/data.json`) shows a long tail of `tool_stats` rows whose `tool_name` is clearly not a tool: `null`, `null || true`, `for`, `&&`, `*.lock`, `README.md`, `test-crispasr.tar.gz`, `⚙`, `termaid`, `web__run`. These aren't corrupted data — they're the literal first whitespace-delimited token `inferToolFromArgs` (`cmd/harnez/exec.go:202-238`) extracted from a real captured shell command, and the heuristic isn't shell-aware:

- it splits on `strings.Fields` (whitespace only), so pipes, `&&`/`||`, redirects, heredocs, and glob arguments are not recognized as structure;
- it skips `VAR=val` and `-flag` tokens but otherwise just returns the basename of the next bare token — which can be a redirect target (`null` from `2>/dev/null`), a shell keyword (`for`), an operator (`&&`), or a filename argument when the real command word got skipped or mis-tokenized.

This runs on every Bash tool call rewritten through the `⚙` exec wrapper (`cmd/harnez/exec.go:581-616`, `formatGearRewrite`), so it affects any project consuming harnez's `tool_stats` telemetry, not just the website.

## Fix

Make `inferToolFromArgs` stop at the first shell metacharacter/operator/keyword instead of blindly taking the next bare token, and fall back to `"Bash"` when no clean leading command word can be identified — rather than returning whatever token happens to be left. Reuse the metacharacter/keyword list already maintained in `isSimpleShellCommand` (`cmd/harnez/exec.go:526-565`) as the boundary set.

## Verification

- Add table-driven cases to the existing `inferToolFromArgs`/exec hook tests covering: `... 2>/dev/null || true`, a `for ... in *.lock; do ...; done` loop, a `cmd1 && cmd2` chain, and a bare filename argument — each must resolve to the real leading binary or fall back to `Bash`, never to the filename/operator/keyword itself.
- Re-run `harnez stats`/regenerate telemetry against a session log containing the commands above and confirm the garbage `tool_name` entries disappear from `tool_stats`.

## Context

Surfaced while investigating why `ubunatic.com/telemetry` showed tool names that "look like arguments" or bare filenames instead of tools.

## Resolution

Implemented exactly as scoped: `inferToolFromArgs` now stops scanning and falls back to
`"Bash"` as soon as it hits a token carrying a shell metacharacter (`shellMetacharacters`,
factored out of and now shared with `isSimpleShellCommand`) or a shell keyword
(`shellKeywords`, same sharing), and also falls back when the only bare token found is a
data-file-shaped positional argument (`looksLikeDataFile`: glob chars or a common
non-executable extension like `.md`/`.lock`/`.json`) with no real command word before it.
`isSimpleShellCommand` was refactored to consume the same two shared vars instead of its
own inline copies, per this ticket's "reuse the boundary set" instruction.

Verified with a before/after probe against the exact reported shapes:

| input | before | after |
|---|---|---|
| `npm test 2>/dev/null \|\| true` | `npm` (already correct) | `npm` |
| `for f in *.lock; do echo $f; done` | `for` | `Bash` |
| `make check && make install` | `make` (already correct) | `make` |
| `README.md` (bare positional, no leading command) | `README.md` | `Bash` |
| `2>/dev/null \|\| true` (bare redirect) | `null` | `Bash` |

Added these five cases to `TestInferToolFromArgs`. Full `go test ./...` passes (one
pre-existing, unrelated flake in `TestIssuesRebaseRepairsIndependentTicketCollisions` —
a tempdir-cleanup race — confirmed to pass in isolation and untouched by this change).
`make install` run to update the live binary.

**Scope note — historical data not touched.** This is a forward-looking code fix only,
same split as issue 331 (code fix vs. retroactive DB cleanup are separate concerns): the
real `~/.harnez/tool_catalog.sqlite` still has ~85 pre-existing garbage `tool_calls` rows
(`for`×9, `*.lock`×6, `README.md`×5, `null`×8, `&&`×4, `command`×6,
`test-crispasr.tar.gz`×11, plus assorted `call_type='shell'` rows whose `tool_name` is a
full multi-word command string like `"null || true"` or `"*.lock && git add -u ..."`).

The single-token rows match this ticket's diagnosis and are what the fix above prevents
going forward. The multi-word, full-command-string rows are **not** explained by
`inferToolFromArgs` (it only ever returns one `strings.Fields` token or one raw `args[]`
element, never a reassembled multi-operator string) despite also being `call_type='shell'`
— they point at a distinct, likely older, tool-name-selection code path not covered by
this ticket's original diagnosis. Left uninvestigated/unfixed here; flagged for a
follow-up ticket if it recurs post-fix or if the dashboard's ~1%-of-total noise floor
(comfortably inside the ~90%-correct bar established in issue 331) ever needs tightening
further.
