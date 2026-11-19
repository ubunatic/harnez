# 1237 — Forge-tracker friction: duplicate search blocked, issues next fails, labels not applied on fj

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 194 (local next-number command), 1138 (forge 404 hint), 1195 (forge create via fj/tea/gh), 1228 (fj wrapper host)

---

## 1. Problem & Motivation

Filing a ticket for taia (tracker `none`, tickets on Codeberg) hit three harnez friction points. Each one blocked or degraded a required step of the issue skill.

1. **Duplicate search is blocked for private forge repos.** `harnez find -d . issues "<terms>"` on a private repo fails with `HTTP 404 ... a private repo needs a token: run harnez forge token import or harnez forge token create`. The issue skill forbids debugging token setup, so an agent cannot run the required duplicate search and has no clear status to report. `fj` was logged in and could create issues on the same repo, but `harnez find` did not use that login.
2. **`harnez find -d <repo> issues next` fails on forge trackers.** With `issues.tracker: none` it errors with `needs a local issues/ dir`. There is no way to get the next free number for a forge-tracked repo.
3. **Inbox fallback does not say that labels are lost.** `harnez issues new "<title>" --label enhancement` with no forge CLI installed warned `no forge CLI found ... install one of tea, fj (fj cannot set labels)` and wrote `issues/inbox/<slug>.md`. The draft kept the label (`**Labels**: enhancement`). The message does not say the label will not be applied when the draft is later filed through fj.

## 2. Technical Specification / Findings

- Observed on 2026-11-17 in taia (`issues.tracker: none`, forge `codeberg.org/harnez/taia`).
- Duplicate search: `harnez find -d . issues "fj"` in this repo works because it is local. The failing case is the private taia forge repo with no harnez read token (`404`, token hint).
- `issues next`: same error text as above, reproduced in taia.
- Fallback: the first `harnez issues new` in taia, with no `fj` or `tea` installed, wrote the inbox draft. A later run with `fj` installed filed #295 through `fj issue create` (dry run showed no label flag).
- Not verified: whether `fj` actually drops labels. The note in harnez's own warning says `fj cannot set labels`.
- Related but distinct: 1138 covers the 404 hint text; 1195 covers forge creation; 1228 covers a fj wrapper; 194 covers the local next-number command.

## 3. Implementation & Verification Plan

- `harnez find issues` on a private forge repo: either use the configured forge login (`fj`) when present, or return a status an agent can report without debugging tokens.
- `harnez find issues next` for forge trackers: support the next-number lookup, or report the forge limitation with a non-error status.
- Inbox fallback: when a label cannot be applied on the target CLI, say so in the fallback warning and in the draft.
- Verify with a private forge repo that has no `harnez` read token but a working `fj` login: `find issues` and `find issues next` both return results or a clear status, and `issues new --label` with `fj` prints a warning that labels are not applied.

## Decision

- **Private-repo `find issues` / `find issues similar`**: Wrap `forgetracker.NewSource` so Forgejo `401`/`404` when no read token is configured falls back to read-only `fj --style minimal issue search --repo <host>/<owner>/<repo> --state open|closed` (parsing `#N: <title> (by <user>)`, synthesizing `HTMLURL`, empty body, stderr note `note: using fj issue search for <target> (titles only; no read token)`). If `fj` is absent or fails/unauthenticated, exit **2** (`silenceIfExitCode`) with an agent-reportable `search blocked on <target> (private repo; no read token or fj login)` status + `warnNoTokenDigging`, rather than exit 0 `[]` which callers could misread as zero duplicates. (Host `/decide` + Consult `consult-1237`, 2026-11-19.)
- **`harnez find issues next` on forge trackers**: Return exit **0** with zero API calls (`forge-assigned on <target> (assigned by forge on harnez issues new)`, JSON `{"number":"","reserved":false,"forge_assigned":true,"forge":"<target>"}`) rather than guessing `max+1` over shared/deleted issue+PR IDs; drop `next` from `findIssuesLocalOnly`. (Host `/decide` + Consult `consult-1237`, 2026-11-19.)
- **Inbox label-drop warnings**: Keep `PlanCreate`'s refusal to drop labels silently via `fj`; whenever inbox fallback saves `Labels` on a target whose `issue_cli` includes `fj` (real run and `--dry-run`), warn explicitly on stderr and embed a `> Note: fj cannot set labels; ...` blockquote under `**Labels**: ...` in `issues/inbox/<slug>.md`. (Host `/decide` + Consult `consult-1237`, 2026-11-19.)

## M1 — Forge-tracker friction fixes

1. **`internal/forgetracker` + `cmd/harnez/find.go` + `cmd/harnez/issues_forge.go` (private-repo fallback & exit 2 status)**:
   - Parse `fj --style minimal issue search --repo <host>/<owner>/<repo> --state open|closed` lines `#<N>: <title> (by <user>)` cleanly (skipping summary lines like `<N> issues`). Keep CLI invocation injectable (and stubbed in tests; never hit live `fj`).
   - Trigger `fj` fallback in `NewSource` wrapper only for `kind: forgejo` when API returns 401/404 and no read token is available. Surface `(titles only; no read token)` note to stderr once. Also works transparently for `checkForgeNewDuplicates`!
   - Preserve `issues show` 404/token hint behavior and `forgeTrackerHint` tests; when `find issues` / `find issues similar` still fail private-repo auth with no working `fj`, exit 2 with `search blocked on <target> (private repo; no read token or fj login)` + `warnNoTokenDigging`.
2. **`harnez find issues next` on forge trackers**:
   - Remove `"next"` from `findIssuesLocalOnly` (`cmd/harnez/issues_forge.go:28`).
   - Handle `find issues next` and `--next` (`cmd/harnez/find.go:170-187`) on forge repos: 0 API calls, exit 0, stdout `forge-assigned on <target> (assigned by forge on harnez issues new)` (or `--json` extending `find.NextResultJSON` with `forge_assigned` and `forge`).
   - Update `TestFindForgeClosedWordAndActiveAlias` (`issues_forge_test.go`), `cmd/harnez/find.go` help, `cmd/harnez/testdata/help.golden`, `docs/commands/issue.md`, and `docs/practices/IssueTracking{,.lite}.md` (if changed, sync via `make install && ./harnez init -d .`).
3. **Inbox label-loss warnings**:
   - `formatInboxIssue` + `writeInboxIssue` / dry-run warning (`cmd/harnez/issues_forge.go`): when `len(labels) > 0` and `"fj"` is in `t.Forge.IssueCLI`, emit stderr `WARNING: labels (<joined>) were saved in the draft only; fj cannot set labels — apply via write token, tea, or forge UI when filing.` and place `> Note: fj cannot set labels; apply via write token, tea, or forge UI when filing.` directly below `**Labels**: ...`.
   - Update `TestIssuesNewInboxFallbackCases` (`cmd/harnez/issues_forge_test.go`) and add unit tests for `fj` read fallback, exit-2 blocked search, `find issues next [--json]` on forge, and inbox `--label` `--dry-run` + real fallback warnings.

## Refinements (from `rev-1237` review of `93281368`)

1. **[blocking] `internal/forgetracker/fj_search.go` pagination & `truncated`**:
   - Pass `--all` to `fj --style minimal issue search --repo <host>/<owner>/<repo> --state <state>` (note live canary proved `0 issues` exits 0 and missing/private fails exit 1).
   - Respect `opts.PageSize * opts.MaxPages` when `> 0`: if fetched count exceeds `limit`, slice to `limit` and return `truncated = true`. Test `--all` in argv, `truncated` capping, and partial `state == "all"` failure (`open` succeeds, `closed` fails -> returns error, not partial silent success).
2. **`handleForgeFindErr` message accuracy (`cmd/harnez/issues_forge.go`)**:
   - Distinguish when a read token IS configured (`t.Forge` has non-empty token) vs no read token (`private repo; no read token or fj login`), so a 404/401 with an existing token reports `search blocked on <target> (HTTP <status>; read token rejected or repo inaccessible)` rather than falsely saying "no read token". Keep `warnNoTokenDigging` and exit 2. Test both cases.
3. **`issues new --forge` stderr wiring (`cmd/harnez/issues.go`)**:
   - Wire `forgetracker.SetFallbackStderr(src, stderr)` right after `forgeTrackerSource(spec)` in the `--forge` branch so the `fj` fallback note reaches `cmd.ErrOrStderr()`.
4. **Docs (`docs/commands/issue.md`, `docs/practices/IssueTracking{,.lite}.md`)**:
   - Note in `docs/commands/issue.md` that `find issues next --json` emits `"number":""` and `"forge_assigned":true` on forge trackers.
   - Update `docs/practices/IssueTracking.md` and `docs/practices/IssueTracking.lite.md` where `find issues next` is described as local-only/max+1 so forge trackers are noted (and check `config.yaml` `.harnez/rules/Issues.md`); if any copyable doc or `config.yaml` rule changes, run `make install && ./harnez init -d .` cleanly and commit source + copy together.
