# 791 — Read harnez rules once per session and resume backend sessions natively

**Status**: Done — header, tui store, shared stale recovery; real-CLI check open
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
  clobber each other. A corrupt file moves to `<id>.json.corrupt-<unix>` with one note; files
  of other tui sessions unused for `sessions.max_age_days` (30) are pruned on tui start.
- Shared resume: `subagent.ResumeTurn` resumes natively (`Resume`/`ResumeStream` with the
  provider id, no history replay). Only `subagent.IsStaleSession` starts a fresh session with a
  note: it matches the provider CLI's own stderr (`ProviderExitError`, never model output) by
  line prefix against `stale_session_errors` of `spec/agent.yaml`. Only verified lines are
  listed (claude "No conversation found with session ID"); other providers are never stale,
  so their resume errors stay errors and the stored id is kept (tui and agent resume alike).
- `harnez agent resume` keeps refusing when `CheckResumable` says no (codex rollout lookup may
  use another CODEX_HOME). A fresh start is noted on stderr and in `--json`
  (`"resumed": false`, `previous_session_id`, `note`).
- Header shows `[resumed]` / `[new session]` after a turn (`sessions.labels`).

## 3. Implementation & Verification Plan
- Tests: header wording; store roundtrip across restart; stale id -> fresh + note, store
  updated; quota error keeps the stored id; concurrent writers keep all keys; corrupt backup;
  prune; agent resume stale recovery incl. `--json`, terminal refusal, non-stale errors.
- Open: real-CLI check of claude's stale line; add codex/pi/agy lines to the spec once verified.
