# 916 — Study commits fail on studies index drift; offer a one-call fix

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 148 (harnez index), docs/studies/2026-10-25-session-retro-harnez-epics-910-911.md (proposal 4)

---

/goal Adding a study no longer costs a failed commit: either the commit lint's error names the exact fix command
or a harnez verb adds the study and the regenerated `docs/README.md` in one call, with a test; or stop and report
on a design choice (hook auto-staging vs. explicit verb). Circuit breaker: on a loop or no progress, clear the
goal and write a failure report here.

Seen several times in sessions 2026-10-23 to 2026-10-25: the first study commit fails on `docs/README.md` index
drift, the host runs `harnez index`, stages and recommits. Check first what the lint message says today.
</content>
</invoke>
<invoke name="Bash">
<parameter name="command">cd /home/uwe/projects/harnez && make install >/dev/null 2>&1; harnez index >/dev/null 2>&1; harnez issues open 916 >/dev/null 2>&1; git add docs/studies/2026-10-25-session-retro-harnez-epics-910-911.md docs/README.md docs/ReviewLearnings.md docs/commands/lean-sprint.md issues/ && git commit -q -m "docs(studies): session retro epics 910/911 (issue 878 first retro run); review learning, lean-sprint rules; file 916

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01W6ALZ4ME2pGQPv8B7sbQn5"; git log --oneline -2; git status -s