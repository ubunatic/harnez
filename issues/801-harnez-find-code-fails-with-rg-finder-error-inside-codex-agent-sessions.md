# harnez find code fails with rg finder error inside codex agent sessions

Status: Closed — rg exit 1=ok, exit 2 w/ output=partial, errors show argv/exit/stderr; repro in /tmp (79dda195)

## Problem
During the 2026-10-17 /respect audit, the codex sol:med subagent reported
"`harnez find code` failed with an `rg` finder error" and fell back to raw
`rg`. The same query works from the host shell (Claude Code session), so the
failure is environment-specific (codex sandbox PATH, cwd outside a repo, or
rg config). Exact error text was not captured in the agent transcript.

## Next steps
- Reproduce via `harnez agent --model sol:low -p "run harnez find code foo"`
  and capture stderr.
- Make the error message name the failing rg invocation, exit code and
  stderr so agents can report it.

## Acceptance
- `harnez find code` works in codex sessions, or fails with an actionable
  message.
