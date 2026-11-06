# 1180 — cleanup: remove all Prime Agent (~/.prime) support

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 727 (skills on agents without a native skill copy, lists Prime)

---

/goal harnez no longer writes to, reads from or documents `~/.prime/agent`, with tests updated and `harnez apply` leaving a Prime install untouched; or stop and report when blocked on an owner decision.

## Problem

Owner request (2026-11-06): remove all Prime Agent support. harnez still treats `~/.prime/agent` as a
harness target. Found by search on 2026-11-06 (re-verify before starting):

- `config.yaml`: `global_section_targets` (`~/.prime/agent/AGENTS.md`, :21), the global docs comment
  (:50), `prime_agent_target: ~/.prime/agent` (:410), skill targets.
- Go, non-test: `internal/claude/config.go`, `internal/claude/apply.go`, `internal/claude/init.go`,
  `internal/assess/ramp.go`, `cmd/harnez/apply_cmd.go`; about 26 test files reference it too.
- Docs: `docs/CLIDesign.md`, `docs/CommandsPipeline.md`, `docs/LanguagePipeline.md`,
  `docs/MultimodalContextDelivery.md`, `docs/practices/AgenticLoop.md` (copyable source: sync the root
  copy per `CLAUDE.md`).
- Skill text and generated instructions that mention "the active harness directory (~/.claude or
  ~/.prime/agent)", plus the `⚙` symlink in `~/.prime/agent/bin` that `apply` creates.

## Want

- Remove the Prime target from config, apply, init, assess and skill installation; drop its schema keys
  and spec values if any.
- Do not delete anything under `~/.prime` on the user's machine: apply simply stops managing it. Say in
  the release note that existing Prime files stay as they are.
- Update docs and tests; keep other harnesses (Claude, Codex, Gemini/agy, Pi) unchanged.

## Done when

- `grep -rn "\.prime\|prime_agent" --include=*.go --include=*.yaml --include=*.md` outside `issues/` and
  `docs/studies/` returns nothing, `make test-q1` is green, and `harnez apply` on a machine with
  `~/.prime/agent` writes nothing there.
