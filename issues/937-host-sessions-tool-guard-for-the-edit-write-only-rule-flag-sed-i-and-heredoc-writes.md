# 937 — host sessions: tool guard for the Edit/Write-only rule (flag sed -i and heredoc writes)

**Status**: Closed — Warn-only edit guard in the single Bash hook (709df88a, M2 ed8dbeaf): warns on repo sed -i/perl -i/redirects incl. absolute paths, skips heredoc bodies, && chains, $VAR, /tmp; escalates on 6th unanswered warning; wording and threshold in spec/edit_guard.yaml. Host make test-q1 green after M1, developer make test-q1 green after M2; host live probe of installed hook: 4/4 expected.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: story `docs/studies/2026-10-25-four-days-of-epics-server-monitor-and-skills-story.md`

---

/goal A harnez PreToolUse hook (opt-in via `harnez apply`) warns or blocks when a Bash command edits a tracked file
with `sed -i`, `perl -i`, a heredoc or `>` redirect into the repo, pointing to Edit/Write; tests cover hits and
near-misses (reading with sed, writing to the scratchpad); or stop and report when the owner prefers prose only.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Evidence

The owner's rule is "edit files only with Edit/Write". Host sessions broke it at least four times between
2026-10-22 and 2026-10-25 (ticket table sed, heredoc study appends), each time admitted afterwards.

## Decision (owner, 2026-10-25)

Warn, do not block. The warning is brief and asks the agent for feedback: why the shell edit was the
better choice here (`harnez rate` or a one-line reason). When the guard has fired many times in a session
without any feedback, the warning escalates: it asks the agent to prove the scripted approach is better,
with concrete token counts (shell edit vs. Edit/Write calls) and reasoning. Threshold and wording go in
`spec/` (no Go defaults).

## Sprint (epic 939)

Developer `d937` (sol:low), reviewer `r937` (opus:low). M1 delivered (709df88a): guard in the single Bash hook,
`additionalContext` warning beside the unchanged `updatedInput`, per-session counter, `harnez rate` reset,
`spec/edit_guard.yaml` + schema. Host `make test-q1` green.

Host decisions: escalate on the 6th warning (after 5 unanswered); `harnez rate --ok` does not reset (heartbeat,
not feedback).

## M2 — Pre-Work / Required Refinements (reviewer r937)

1. High: absolute paths inside the repo must warn (`pathInRoot`, exec.go:1372).
2. High: skip heredoc bodies; `git commit -F - <<EOF ... a -> b ... EOF` and apostrophes in the body must not warn.
3. Medium: sed/perl scan stops at `&&`, `||`, `;`, `|`; `sed -n p a && grep -i x f` must not warn.
4. Medium: targets with `$VAR` or `"$out"` are unknown → do not warn.
5. Medium: no `session_id` → still warn, only skip the counter.
6. Medium: both warning texts move to `spec/edit_guard.yaml` (schema required).
7. Medium: `rate` reset uses the same state dir as the hook (no hard-coded default).
8. Medium: test that runs the hook and asserts the output JSON has `hookSpecificOutput.additionalContext` and the
   unchanged `updatedInput`; test cases for items 1-5; escalation test checks warnings 1-5 normal, 6 escalated.
9. Low: atomic write of the counter file; delete expired `edit-guard-*.json`; debug log when the spec fails to
   load. Optional: `&>file`, `cd /tmp && echo > f`.
