# 318 — git-log-style bare -N limit shorthand for harnez find and a new issues list verb

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics

---

## Summary

`harnez find issues status:open -n 3` (or the shorter `harnez find issues is:open -n3`)
already does "show the last N open issues", but the user wants a `git log -2`-style
bare numeric shorthand — no `-n`/`--limit` flag name at all, just `-N` directly.

Confirmed via `AskUserQuestion`: implement the shorthand in **both** places:

1. `harnez find issues -2` (or `harnez find issues is:open -2`) — bare `-N` as an
   alias for `-n N` / `--limit N`.
2. A new `harnez issues list -2` verb that also accepts the same bare `-N` shorthand.

## Why this needs real work, not a quick patch

- **Bare numeric flags aren't free in Cobra/pflag.** `git log -2` works because git
  does its own custom arg pre-processing; Cobra/pflag will error on an unrecognized
  `-2` shorthand unless the args are pre-scanned for a bare `-\d+` token and rewritten
  (e.g. into `--limit 2`) before Cobra's flag parser runs. Needs a shared helper so
  both `find` and `issues list` get identical, tested behavior — a bare `-N` should
  not be swallowed by an unrelated numeric-looking positional query term (e.g. an
  issue search query happens to contain a `-2`-shaped token) or by ticket-number
  positional args in `issues <verb> <n>`.
- **`issues list` breaks the current closed-set verb design on purpose.** `harnez
  issues --help` documents its verbs as "a closed set, mirroring
  docs/IssueTracking.md's Allowed Values" (open, start, block, close, draft, new, mv,
  rebase, lint) — all status-mutation verbs. `list` is a pure read/discovery verb and
  duplicates what `find` already does (see `docs/CLIDesign.md`'s explicit
  `find` = read/discovery vs `issues` = status-mutation separation, the same kind of
  boundary called "load-bearing" for `apply`/`init`). The user explicitly chose to
  add `list` anyway rather than only extending `find` — implement it, but keep its
  filter/limit logic as a thin wrapper over `find`'s existing query engine rather
  than a second, diverging implementation, to avoid the two commands drifting apart
  on ranking/filtering semantics.

## Proposed scope

- Add a shared bare-`-N` pre-parser (used by both `find` and the new `issues list`)
  that only fires when the token is exactly `-` followed by digits, appears where a
  flag is expected, and does not collide with an existing explicit `-n`/`--limit`.
- `harnez find issues -N` / `harnez find issues <query> -N` — equivalent to
  `--limit N`.
- `harnez issues list [status-filter] -N` — new read-only verb; default filter
  `is:open` (matching this session's actual use case) unless a filter is given;
  wraps `find`'s existing issues-query engine rather than reimplementing filtering.
- `issues list` must not commit, must not accept `--commit`/`--no-commit`/etc. (the
  mutation-only flags documented for the other verbs).

## Verification

- [ ] `harnez find issues -3` and `harnez find issues is:open -3` both match
      `--limit 3` output exactly.
- [ ] `harnez issues list -3` and `harnez issues list is:open -3` both match
      `harnez find issues is:open -n 3` output.
- [ ] A query containing a literal `-2`-shaped search term is not misparsed as a
      limit flag.
- [ ] `issues list` appears in `harnez issues --help` documented as read-only,
      distinct from the status-mutation verbs.
- [ ] `go test ./...` passes; new tests cover the shared bare-`-N` pre-parser
      directly (not just through the two command entry points).
