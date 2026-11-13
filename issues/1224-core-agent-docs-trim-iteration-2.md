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
- **Prose-asserting tests** (done, see "Tests loosened" below) blocked a further trim: the
  "Harnez Agent" and "Code and Documentation Search" headings and five sentences
  (`cmd/harnez/agent_test.go`), and the code-memory MCP and selftest wording
  (`internal/claude/rules_init_test.go`).
- **Rules read**: 9.2 KB in one call. haiku pipes it through `head -c N` on its own; the header
  now forbids that, confirmed by one run only.
- **Other copyable docs** were not touched: IssueTracking, GoRelease, Go, Git, Make, Canary, Spec.
- **Read guard threshold**: a doc meant to be read on demand should stay under 100 lines, or
  the native-read guard blocks it (seen with the 104-line `docs/Bash.md`). No test enforces this
  for lite docs.

### Tests loosened (2026-11-13)

The owner judged the wording tests too rigid. They now check names agents type and structure,
not sentences:

- `TestManagedAgentExamplesParseAgainstCobra` parses every `` `harnez agent ...` `` example in
  `config.yaml` against the CLI. The two heading anchors and five required sentences are gone.
- `TestRunInit_GeneratedToolsMCPAndSelftestGuidance` keeps tool and command names
  (`list_projects`, `index_status`, `index_repository`, `harnez find code|docs`, `rg`,
  `background_task_list`, `--session <id>`, the self-test exactly once) and drops six phrases.
- `TestRunInit_GeneratesHarnezRules` keeps the rules read command, "once per session" and the
  ban on routine issue images; three header sentences and the PNG sentence are no longer asserted.
- `TestHarnezInitSkillInstallToEveryConfiguredTarget` checks four names instead of 22 phrases
  (found by the 1225 run).

Not touched: the other `internal/claude/*_skill_test.go` files (handoff, issue, review, decide,
next, wrap, peer-assistant, advisor). Loosen each when its skill is trimmed.

### Why no session read the `@docs/...` links (canary, 2026-11-13)

Claude Code inlines `@path` from `CLAUDE.md` only when the path is clean. A scratch project with
a symlinked `CLAUDE.md` and four forms, checked with `/context`:

| Form in `AGENTS.md` | Inlined |
|---|---|
| `- Doc A @docs/A.md,` (trailing comma) | no |
| `- Doc B @docs/B.md` (end of line) | yes |
| `- See @docs/C.md for more.` | yes |
| `` `@docs/D.md` `` (code span) | no |

The doc-list renderer writes `ref + ","` whenever a doc has a hint (`internal/claude/apply.go`,
lines 155-159), so none of the 18 docs listed in this repository is inlined. Dropping the comma
would inline all 18, about 115 KB, into every Claude session. For Codex and other harnesses
`@path` is plain text either way.

### Owner intent and design direction

The owner's intent behind "Read local `@<file>` links immediately" is "read them as needed":
agents often never found the docs or dismissed them as unimportant. The 1221 sessions agree: the
same Bash rules gave haiku a clean script when they were inlined and a poor one when it had to
fetch them, and haiku cut the instructed rules read short in three of six runs.

Direction to evaluate: one content source, per-agent delivery.

- A `harnez read --rules` verb (name open) that prints the rule set for the calling agent,
  selected by harness, model tier and role: weak tiers get the lite docs they need inlined,
  strong tiers get hints only, leaf workers skip orchestration rules, and harness-specific
  lines (Codex `apply_patch`, Claude `run_in_background`) go only to that harness.
- Delivery through the session-start hook where one exists, so the rules arrive without a tool
  call the model can truncate; by instruction elsewhere.
- Selection lives in `spec/` (per model, tier and role), not in per-model doc copies.

Unverified: whether the hook knows the model of a user-started session, and whether Codex
session-start hook output reaches the model's context.

## 3. Implementation & Verification Plan

- Canary before building delivery: `haiku:low`, three runs with the rules and the lite Bash doc
  injected at session start against three runs as now, scored with the 1221 checks.
- Make the renderer's comma a decision: a doc marked inline gets a clean `@docs/X.md`, every
  other doc a form that Claude Code does not import.
- Decide the `@<file>` rule with the owner first; it changes what every session reads.
- Trim one doc at a time and rerun the 1221 task (prompt and 15 checks are recorded there) on
  `haiku:low` and `luna:low`, three runs each, because single haiku runs vary.
- Consider a test that fails when a `*.lite.md` source reaches 98 lines.
- Run `make test-q1` once per change. A test should assert commands and structure, not sentences.
