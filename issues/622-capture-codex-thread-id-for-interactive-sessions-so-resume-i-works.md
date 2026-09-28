# 622 — Capture codex and agy session IDs for interactive sessions so resume -i works

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: #620, #450

---

/goal `harnez agent start -i` with codex or agy stores the provider session ID, and
`harnez agent resume -i --name <session>` reopens that thread; verify with a test and a manual
start/exit/resume round trip; stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

Observed in `~/projects/neus` after #620 (default model is codex):

```text
$ harnez agent start -i
Harnez Agent Interactive: quiet-badger (97969b7d-6953-40b6-8f0d-6042ae49f729)
...
Reconnect: codex resume 01a0e7a1-07fd-7830-b8b8-b1f0110ca250
$ harnez agent resume -i
Error: resume -i requires --name <session>
$ harnez agent resume -i --name quiet-badger
Error: session "quiet-badger" cannot be resumed: codex did not expose a provider session ID
```

Codex knew the thread ID (it printed it) but Harnez never recorded it. `start -i` sets
`ProviderSessionID` only for claude (`cmd/harnez/agent_interactive.go`, where the harnez ID is
passed as `--session-id`). Codex (and agy) interactive sessions are therefore never resumable,
which defeats the main point of #620.

Same for agy (user report, 2026-09-28):

```text
$ harnez agent resume -i --name swift-falcon
Error: session "swift-falcon" cannot be resumed: agy did not expose a provider session ID
```

## 2. Technical Specification / Findings

- Codex has no `--session-id` input; the ID must be discovered after launch. Candidate sources
  (canary first, per `docs/Canary.md`): the codex session files under `~/.codex/sessions/`
  (match by cwd and start time), or the `Reconnect: codex resume <id>` line on exit. Pick the
  most robust; do not scrape the TUI stream if a file source exists.
- agy has the same gap (confirmed, see §1): find where agy exposes the conversation ID for an interactive run and record it; resume uses `--conversation <id>`.
- Claude `resume -i` is confirmed working by the user; use it as the reference path.
- Candidate (user note): codex, agy and claude all have a `/rename` command. A session titled with the harnez name could be found by title in the provider store, or resumed by name if the provider accepts a name on resume (canary: does `codex resume <name>` / agy accept it?). Rename changes only the title, not the ID, and typing into the TUI is fragile, so prefer a file/ID source if one exists.
- Secondary: `resume -i` without `--name` should resume the latest resumable session in `-d`
  (same rule as plain `resume --continue`), instead of erroring.

## 3. Implementation & Verification Plan

- Canary: confirm where codex exposes the thread ID for an interactive run.
- Record the ID on the session (during run or at teardown) and test it with a fake runner.
- Manual: in a scratch dir, `harnez agent start -i --model codex…`, exit, then
  `harnez agent resume -i --name <session>` and `harnez agent resume -i` both reopen it.
