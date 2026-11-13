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
