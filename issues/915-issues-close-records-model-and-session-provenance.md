# 915 — issues close records model and session provenance

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 904 (Recommendation 5), 911 (epic)

---

/goal `harnez issues close` accepts `--model <spec>` and `--session <id>`, writes them into the ticket's close
metadata so multi-provider attribution survives, with tests; or stop and report on a design choice (where the
fields live, whether the generated index shows them). Circuit breaker: on a loop or no progress, clear the goal
and write a failure report here.

Split out of 904 by epic 911: 904 keeps the doc-only skill changes. Check first whether `harnez agent` telemetry
already links a session to its ticket close, so the flags may default from it.
</content>
</invoke>
<invoke name="Bash">
<parameter name="command">cd /home/uwe/projects/harnez && harnez issues open 915 >/dev/null 2>&1; git add issues/ && git commit -q -m "docs(issues): 911 decisions recorded; file 915, close provenance flags split from 904

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01W6ALZ4ME2pGQPv8B7sbQn5" && git log --oneline -1 && git status -s