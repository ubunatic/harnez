# 1224 — Core agent docs trim, iteration 2

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 1221, 1222, 1223

---

## 1. Problem & Motivation

1221 trimmed AgenticLoop, Bash, `AGENTS.md` and the generated rule files and stopped there on
purpose. The items below were seen during that work and left for a second pass.

## 2. Technical Specification / Findings

- **Lite AgenticLoop is still 9.7 KB** (target was about half of 13.6 KB). The invariants are
  about 4.5 KB of it; the fixed model-picks block is 1.1 KB.
- **"Read local `@<file>` links immediately"** (`AGENTS.md`, Local Context Links): taken
  literally this means reading some twenty docs at session start. No test session did;
  `luna:high` named it as a rule it set aside. The practised rule is "read a doc before work it
  covers". Owner decision: reword or keep.
- **Stale Bash hint copy**: `internal/subagent/subagent.go` (line 122) and
  `internal/readcard/read_test.go` carry "- No ';', break before then/else", the wording that
  1221 replaced in `config.yaml` because agents read it as "`then` alone on a line".
- **Test-pinned `Tools.md` phrases** block a further trim: the "Harnez Agent" and "Code and
  Documentation Search" headings and five sentences (`cmd/harnez/agent_test.go`), and the
  code-memory MCP and selftest wording (`internal/claude/rules_init_test.go`).
- **Rules read**: 9.2 KB in one call. haiku pipes it through `head -c N` on its own; the header
  now forbids that, confirmed by one run only.
- **Other copyable docs** were not touched: IssueTracking, GoRelease, Go, Git, Make, Canary, Spec.
- **Read guard threshold**: a doc meant to be read on demand should stay under 100 lines, or
  the native-read guard blocks it (seen with the 104-line `docs/Bash.md`). No test enforces this
  for lite docs.

## 3. Implementation & Verification Plan

- Decide the `@<file>` rule with the owner first; it changes what every session reads.
- Trim one doc at a time and rerun the 1221 task (prompt and 15 checks are recorded there) on
  `haiku:low` and `luna:low`, three runs each, because single haiku runs vary.
- Consider a test that fails when a `*.lite.md` source reaches 98 lines.
- Run `make test-q1` once per change; update pinned tests only together with the wording.
