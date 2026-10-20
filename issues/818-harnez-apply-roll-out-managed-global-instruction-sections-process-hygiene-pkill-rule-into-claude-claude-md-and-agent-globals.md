# harnez apply: roll out managed global instruction sections into ~/.claude/CLAUDE.md and agent globals

Status: Open — M2 pre-work: do not delete existing global blocks
Priority: P2
Category: Feature

## Problem
The global `~/.claude/CLAUDE.md` is hand-written; `harnez apply` only injects/removes the Tool
Feedback Protocol. Cross-project rules learned in sessions (e.g. never `pkill -f` a pattern that
matches your own shell, which killed the agent's Bash call twice on 2026-10-17) can only reach
repos via `harnez init` rules (`.harnez/rules/Tools.md`), not agents working outside a repo.

## Proposal
Managed sections in config.yaml (`global_sections:`) that `apply` writes between markers into
`~/.claude/CLAUDE.md`, Codex `AGENTS.md` and AGY globals, idempotent, removable, keeping
user-written text untouched. First section: Process hygiene (pkill/pgrep rule, HTO=0 background
rule).

## Acceptance
- `harnez apply` adds the section once between markers; a second run changes nothing; removing it
  from config removes it; user text outside markers is preserved; tested on temp homes.
- The pkill rule (already in `.harnez/rules/Tools.md` via config.yaml and docs/ProcessHygiene.md)
  reaches the global files.

## Delivery

- Implementation: `d0f21665` (`feat(apply): reconcile managed global instruction sections (818)`).
- Config-driven `global_sections` reuse the existing Markdown marker format; section `name` is
  the stable ID. Tool Feedback Protocol keeps its legacy ID, migrates in place without duplication,
  and respects the existing feedback opt-out. Removed sections are pruned; managed revert removes
  blocks while preserving user bytes and file permissions. Diff/check prints unified pending changes.
- Targets: `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, `~/.gemini/GEMINI.md`,
  `~/.prime/agent/AGENTS.md`, `~/.pi/agent/AGENTS.md`. Preflight correction: issue 415 had
  removed global instruction writes entirely in this checkout. Existing files now receive sections;
  missing files and the former `~/AGENTS.md` symlink remain uncreated.
- Temporary-home tests cover add, idempotence, edit, removal, user text/permission preservation,
  missing files, legacy feedback migration, opt-out, diff without writes, duplicate blocks,
  unrelated markers, malformed-block idempotence, and schema conformance. The issue-415 regression
  now checks exact user content plus configured sections, and exact restoration on managed revert.
- Passed: `make lint`, `go test ./internal/claude/ ./cmd/harnez/`, `make test-short`, `make install`.
  No real-home `harnez apply`, agents, or push; host performs live rollout and independent review.

Rendered Process hygiene section (inside its begin/end markers):

```markdown
### Process hygiene
- Never use `pkill -f`/`pgrep -f` with a pattern that can match your own shell command line.
  Match exact argv fields via `ps -eo pid,args`, or kill recorded PIDs.
- Set `HTO=0` for long-running commands, tests, or waits run as background tasks/jobs.
```

## M2 Pre-Work / Required Refinements (host real-home diff 2026-10-20)

`harnez diff` against the real home (not applied) shows 818 would DELETE three existing
harnez-marked blocks from ~/.claude/CLAUDE.md: `Instructions Hierarchy`, `Issue Tracker Discovery`,
`Agent-Filed Feedback`. They were written by older harnez before #415 stopped managing global root
docs, and the user relies on them. It also rewrites the Tool Feedback Protocol text (shortened).

1. Never remove a `harnez:begin <id>` block just because <id> is absent from `global_sections`.
   Remove only ids harnez knows it owns under this mechanism: ids listed in config
   (`global_sections` or an explicit `retired_global_sections:` list), never unknown ids. Unknown
   harnez-marked blocks are left byte-identical. Regression test with the three real block ids.
2. Do not change existing section text as part of this ticket: the Tool Feedback Protocol content in
   config must render exactly as it does today (keep the current wording; byte-compare a test against
   the pre-818 rendering). Only the new Process hygiene section is added.
3. Expected real-home diff after the fix: only the added Process hygiene block per target, nothing
   removed. Do not run apply on the real home; the host runs `harnez diff` then apply.
