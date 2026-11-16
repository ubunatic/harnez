# 1202 — harnez issues has no way to edit a forge issue body; agents fall back to tea

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1196, 1197, 1201, 1067

---

/goal Agents can reliably update issue bodies and titles through `harnez issues` in local and forge-tracker checkouts, without losing concurrent edits; or stop and report when a user decision, forge capability, or denied permission blocks progress. Recheck live code and recent history first. On a loop, hard blocker or no progress, clear the goal and record the failure and resume point.

## 1. Problem & Motivation
Feedback from another session (2026-11-09): "I updated the issue with tea because harnez issues can't edit an issue body."
On a forge tracker, `harnez issues show` is read-only and no verb edits a body. Agents bypass harnez, so the edit skips its token handling and telemetry.

In Taia, `harnez issues show 161` resolved Codeberg from `origin`, but `fj issue edit 161 body` could not infer the repository. Retrying with explicit host and `harnez/taia#161` worked. Passing the full body as an argument also made failure output noisy. No issue-creation failure was observed; forge creation already supports `-F` (#1195).

## 2. Findings
- `harnez issues --help`: `show` on a forge origin is "read-only"; no `edit` verb exists.
- Forgejo/Gitea: `PATCH /repos/{owner}/{repo}/issues/{index}` takes `title` and `body` (unverified in harnez; canary first, docs/Canary.md).

## 3. Plan
- Add `harnez issues edit <n> [--title T] [--body-file F|-]` for forge and local trackers, with exact multiline preservation and repository discovery consistent with `show`. Define local file and index behavior without discarding unrelated edits. Return the issue URL or local path and actionable errors without leaking tokens or the full issue body.
- Prevent lost updates using a revision or expected-body check from the prior read. Refuse conflicting edits; verify whether the forge supports an atomic conditional update and state any limits rather than claiming an atomic guarantee from a non-atomic preflight. Check issue state before editing and report closed or missing issues clearly.
- Keep the show/create/edit/close experience consistent across tracker types. Verify current forge create `-F` behavior and output rather than assuming it is missing; address any gaps needed for a single-call workflow.
- Test local and fake-forge paths, including repository resolution, stdin/file bodies, exact payloads, conflicts, closed issues, concurrent updates where supported, and secret-safe errors. Canary the real forge mechanism first.

## Sprint 2026-11-16 (lean sprint 1228, 1212, 1202)

Searcher findings (report: `docs/studies/2026-11-16-two-agent-lean-sprint-forge-filing.md`):

- On a forge tracker `new`, `show` and `comment` exist; `close`, `open` and the other status
  verbs are local only (`cmd/harnez/issues.go:450`, `:486`); there is no `edit` and no `reopen`.
- `show` already uses harnez's own API client (`internal/forgetracker/forgetracker.go:88`,
  `internal/forge/issue.go:25`). Token lookup: stored Forgejo token, then
  `CODEBERG_TOKEN`/`FORGEJO_TOKEN`, then `gh auth token` (`internal/forge/api.go:43`). The stored
  token is read-only by design (`harnez forge token`, scopes in `spec/forge_token.yaml`).
- No harnez code sends an issue or pull-request write to a forge API today.
- fj v0.7.0 has `issue edit <n> title|body` with positional text only, `issue close`, and no
  reopen.
- Codeberg swagger: `PATCH /repos/{owner}/{repo}/issues/{index}` takes `title`, `body`, `state`
  and `updated_at`; `POST .../issues` takes `title` (required) and `body`;
  `POST .../issues/{index}/comments` takes `body`. Accepted `state` values and any conditional
  meaning of `updated_at` are unverified.

## Decision

Owner decisions, 2026-11-16 (asked by the host in the sprint session):

- Forge issue writes (create, edit, comment, close, reopen) go through harnez's own Forgejo API
  client with a write-capable stored token. `harnez forge token` gets an issue-write scope; the
  owner creates or rotates that token once. fj stays as the fallback for create and comment when
  no write token is stored.
- Forge writes are allowed only in a host session, never in a subagent.
- The live check runs on both `ubunatic/harnez` and `harnez/taia` on Codeberg, because the two
  repositories use different trackers (`harnez` and `none`). One scratch issue each, closed at
  the end.

Milestones for this ticket follow from the architect's design (next section, to be added).
