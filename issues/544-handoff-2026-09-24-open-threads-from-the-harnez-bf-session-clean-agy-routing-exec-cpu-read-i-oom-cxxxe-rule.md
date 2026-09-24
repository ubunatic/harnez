# 544 — Handoff 2026-09-24: open threads from the harnez-bf session (clean, agy routing, exec CPU, read -I OOM, CxxxE rule)

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Handoff
**Related**: [[ProcessHygiene]], [[353-tell-only-claude-never-propose-claude-md-changes-assume-other-agents-respect-agents-md]], [[534-capture-agent-session-tokens-continuously-not-only-at-session-end]], [[535-review-agent-collector-lifecycle-automation-install-auto-start-keep-current]], [[538-harnez-agent-report-leftover-processes-at-turn-end-reap-only-stopped-ones]], [[539-agents-in-other-projects-don-t-know-harnez-agent-or-model-names-like-terra-low]], [[540-harnez-agent-short-model-aliases-opus-terra-low-and-a-correct-error-when-p-swallows-model]], [[542-reduce-idle-cpu-of-harnez-usage-compact-watch-8-of-a-core]], [[543-harnez-process-grew-to-15-16-gb-rss-and-was-oom-killed-twice-agent-resume-under-harnez-exec]]

---

## /goal

A later session can pick up every open thread from 2026-09-24 without the chat log: finish or
explicitly park each item below, then close this ticket. Re-check HEAD first; this list is a snapshot.

## Done today (for context, all closed and installed)

| Ticket | Result |
|---|---|
| 533 + 532 | `harnez clean procs q1 [--kill]`; old `clean` → `revert --managed`; quota-1 run records; stopped-child recovery |
| 536 | `harnez statusline` registered again + guard test; cwd/drift shown (`project → current`) |
| 537 (+273 superseded) | agy shell commands via `harnez exec`: bash shim when active (quiet), PreToolUse rewrite fallback; `harnez agent` provisions the shim; coverage view in `harnez stats --agents` |
| 541 | `harnez exec` busy-wait fixed (94% → 0.3% CPU); descendant stops detected |
| — | `harnez read -I` recommendations paused everywhere (543); new managed rule "Always `make install`" |

## Open threads (in suggested order)

1. **353 (P1, lean sprint in progress).** M1 committed `746418b` (Claude-only SessionStart rule via
   `harnez hook claude-instructions`), but it is **not live**: run `harnez apply` and verify that a fresh
   Claude session shows the rule. Then M2 (mention counter; deliver the tip via `UserPromptSubmit`, since
   Stop output usually does not reach the model; see the ticket's pre-work). Host review of M1 was diff-only.
2. **543 (P0): `harnez read -I` OOM.** Multiple files + `-L` reached 2.1 GB in 3s and 15–16 GB before the
   OOM kills. Fix, then lift the pause (templates, AgenticLoop, lean-sprint/roadmap commands, hook deny
   message, session tip).
3. **Roll out managed-block changes to other repos.** The `read -I` pause and "Always `make install`" are
   only in harnez + `~/.claude`. Run `harnez init` in loom, voxi, cati, … when their sessions are idle
   (the user was asked; no answer yet). Check with `harnez scan-docs`.
4. **539 (P1):** agents in other repos don't know `harnez agent` or model names ("ask terra:low …" failed in cati).
5. **540 (P1):** model aliases (`opus`, `terra:low`, stats model names) + the correct error when `-p`
   swallows `--model`. Also seen: `harnez agent delete <id>` is rejected (only `--name` works).
6. **538 (P2):** report processes left over at turn end, reap only stopped ones (no turn time limit, by decision).
7. **542 (P2):** `harnez usage --compact --watch` idle CPU still 8.7%.
8. **534 / 535 (P2):** live session tokens (the statusLine payload has them per session; see 534) and the
   collector lifecycle, which needs a policy decision from the user.
9. **537 follow-up note:** agy launched from a Claude session inherits Claude env markers, so shim exec rows
   are labelled `claude`; the stats join compensates. The cleaner fix: drop the markers in `agyLaunchEnv`.

## Session notes

- Codex plan: 5h window at 61% at 16:40 (resets ~18:10). No zombie agent processes were found; the drain was
  normal sprint turns (this session's plus the voxi/loom sprints).
- Nothing is pushed: `main` is far ahead of origin and github (push both remotes when asked, see AGENTS.local).
- Starting agy as a user: `harnez agent chat --model agy:flash37:low` (shim route, clean UI). Plain `agy`
  still works via the hook fallback, but shows `harnez exec …` in its UI.
- Rule reminders from the user: always `make install`; only harnez calls agy; name milestones with a
  short label ("M2 (descendant stops)"); write CxxxE.md, never the literal name.
