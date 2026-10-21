# 785 — cloud session bootstrap: install harnez and test tools via SessionStart hook

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Tooling / Cloud Agents
**Related**: #780

---

## 1. Problem & Motivation
`CLAUDE.md` and `.harnez/rules/Tools.md` tell agents to run `harnez read`, `harnez find`
and `make test-q1`, but a fresh cloud session has no `harnez` on PATH and no
`~/go/bin` in PATH. Every session starts with a failed command and a manual install.

## 2. Technical Specification / Findings
- Claude Code on the web runs `.claude/settings.json` SessionStart hooks; the repo has
  no `.claude/` dir. Other cloud agents (Jules, Codex) use their own setup scripts.
- Needed: `go install ./cmd/harnez`, `$(go env GOPATH)/bin` on PATH, and optional test
  tools (`minisign`) once #781 no longer needs them.

## 3. Implementation & Verification Plan
- Add `scripts/cloud-setup.sh` (idempotent, Bash rules in `docs/Bash.md`) and call it from
  a SessionStart hook gated on `CLAUDE_CODE_REMOTE`; document it for Jules/Codex setup.
- Verify: a fresh cloud session runs `harnez read ...` and `make test-q1` without manual steps.
