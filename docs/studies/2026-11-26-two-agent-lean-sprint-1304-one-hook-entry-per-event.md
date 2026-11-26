---
kind: sprint
date: 2026-11-26
ticket: 1304
branch: sprint/1304
models:
  searcher: skipped # a plan-only developer turn inspected the live code
  developer: claude:haiku:high
  reviewer: agy:neon:med # one-shot after M1, M2 and M3; the host reviewed every milestone
---

# Two-Agent Lean Sprint: #1304, one hook entry per agent event

## Outcome

`harnez apply` now writes one hook entry per event: 8 for Claude (`harnez hook <Event>`) and 7 for Codex (`harnez hook codex <Event>`). Before, Claude had 14 entries.
- The manifest (events, handlers in run order, kind, tools, matcher) lives in `spec/hooks.yaml`, and Go binds run functions by name.
- The dispatcher runs handlers in order, each under a 40 s budget with panic recovery, and merges the replies by Claude's precedence rules.
- Migration replaces old harnez entries and keeps foreign ones. Ownership is decided per handler, with POSIX word splitting.

## Milestones

| Milestone | Commits | Review findings that became pre-work |
|---|---|---|
| Plan (read-only) | (none) | union matcher over `*` (37.7 ms median start-up per hook call), sequential order, 40 s budget |
| M1 dispatcher | 5f66da33, b9842066 | a winning deny kept `updatedInput`; `{}` counted as a reply; vacuous goldens |
| M2 Claude entries | 967b120f, b0fbda46 | manifest in a Go var broke the spec rule; `-c` `hooks:` silently ignored; stale docs |
| M3 spec manifest and Codex | bdcf6904 | foreign hook dropped via a quoted env value; Codex Bash filter skipped payloads with no `tool_name`; `Remove` untested on legacy entries |
| M4 review fixes | 22911b09 | none; the host reviewed it alone |

Every developer milestone ended with one `make test-q1`: 83 ok, 0 FAIL.

## Verification

- **Host checks:**
  - In a scratch HOME after each milestone: the entries apply writes, idempotency, and dispatch output for Bash, Read, Grep and SessionStart.
  - Codex PreToolUse on the live binary: the old `harnez codex-hook` and the new `harnez hook codex PreToolUse` give byte-identical stdout and the same exit code. Checked for a Bash payload and for malformed JSON.
- **Host `make smoke` on the real `~/.claude`:**
  - all checks passed;
  - settings outside `hooks` are unchanged (sorted `jq -S` diff against a pre-run backup);
  - the 8 Claude and 7 Codex entries are live.

## Learnings

- **The neon:med one-shot reviewer** found a real, distinct MED gap on all three passes:
  - M1 was a merge-contract bug;
  - M2 was a spec-rule regression;
  - M3 was the quoted-env deletion of a foreign hook.

  Each pass took about 2 to 5 minutes. A host diff review alone passed all three.
- **haiku:high developer:** turn costs grew with resumes. M4, the smallest diff, ran 5.4M tokens including cached ones, against 0.26 to 0.46M for M1 to M3.

  Resuming one developer across 5 turns let the context grow. For a fix-only milestone, start a fresh developer with the ticket's pre-work section.
- **Spec rule:** "move wiring into a Go var" reads as a cleanup but breaks the spec rule. Put a spec-ownership question into every reviewer prompt that touches config.
