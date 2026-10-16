# 791 — Read harnez rules once per session and resume backend sessions natively

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature / Agent Instructions
**Related**: #774, #776

---

## 1. Problem & Motivation
- The generated AGENTS.md/CLAUDE.md header said "**Before any work, read all Harnez rules …**".
  Every `harnez tui` codex turn, even a resumed one, started with "I'll check the required
  Harnez rules first", burning a tool call and tokens per prompt.
- User decision: subagents read the rules once per session, and resumed sessions must be cheap.
- `harnez tui` kept backend session ids in memory only: `harnez tui --session X` after a restart
  started fresh provider sessions, so the rules (and all context) were read again.

## 2. Technical Specification / Findings
- Header (`config.yaml` `agents_md.rules.header`): "Once per session, before your first task,
  read all Harnez rules in one call: `…`. Skip this if you already read them in this session."
  `isRulesHeaderLine` also recognises the new prefix, so `harnez init` replaces older headers.
- tui store: one JSON file per tui session id under `sessions.dir` of `spec/tui.yaml`
  (`~/.harnez/tui/sessions/<id>.json`), map `provider:model` -> backend session id. Updates take
  an flock on `<id>.json.lock`, re-read, merge and atomically rename, so two tui processes never
  clobber each other (different ids: different files; same id: merged under the lock).
- Shared resume: `subagent.ResumeTurn` resumes natively (`Resume`/`ResumeStream` with the
  provider id, no history replay); on a stale id (`ResumeChecker` says not resumable, or the
  retry policy accepts the error) it runs a fresh session and reports a note.
  The router retries on any resume error (kept from 776); `harnez agent resume` retries only on
  `subagent.IsStaleSession` errors (missing conversation/rollout/session) so quota and auth
  errors keep their bookkeeping.
- Header shows `[resumed]` / `[new session]` after a turn (`sessions.labels`).

## 3. Implementation & Verification Plan
- Tests: header renders once-per-session wording and replaces the pre-791 header; store
  roundtrip (write, new router, resumes stored id); stale id -> fresh session + note, store
  updated; concurrent writers on one id keep all keys; agent resume stale recovery.
- Open: real-CLI check of the stale-error markers per provider (claude "No conversation found"
  is known; codex/pi/agy wording unverified, codex is covered by `CheckResumable`).
- Open: no pruning of old tui session files yet.
