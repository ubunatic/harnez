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
  `config.yaml` against the CLI and requires at least one in the parsed rule files, plus the
  names `subagent_mode` and `/goal clear`. The two heading anchors and five required sentences
  are gone.
- `TestRunInit_GeneratedToolsMCPAndSelftestGuidance` keeps tool and command names
  (`list_projects`, `index_status`, `index_repository`, `harnez find code|docs`, `rg`,
  `background_task_list`, `--session <id>`, the self-test exactly once) and one stem,
  "authoriz", for the `index_repository` consent rule. Five phrases are dropped.
- `TestRunInit_GeneratesHarnezRules` keeps the rules read command, "once per session" and the
  ban on routine issue images; three header sentences and the PNG sentence are no longer asserted.
- `TestHarnezInitSkillInstallToEveryConfiguredTarget` checks five names instead of 22 phrases
  (found by the 1225 run).

Reviewer pass (`codex:sol:low`, read-only): it found that the first rewrite of the command test
could pass on a YAML comment alone, and that the consent rule for `index_repository` had lost
its only check; both are fixed as described above. Accepted risk, not fixed: unit tests no
longer notice when a trim drops the goal exit guidance, the three-state MCP distinction, the
"absent optional files are skipped" sentence, the PNG-on-request sentence or the init skill's
stop-and-confirm guard. Those are rules for agents; `/harnez-cleanup` test sessions are the
check for them. The command test still does not validate positional arguments.

### Skill tests on one helper (2026-11-13, second pass)

All per-skill tests now use `checkInstalledSkill` (`internal/claude/skill_install_test.go`):
registration, installation to every target, `names` matched verbatim and `rules`, short
lower-case stems for rules with side effects. What each test dropped and what it keeps:

| Skill | Dropped (headings and sentences) | Kept |
|---|---|---|
| `decide` | "## Rules", "Very critical only", "relayed answer does not count", "Circuit breaker" | `--model cheap`, `--model smart`, `docs/lang/Git.md`; stems "force-push", "no release" |
| `next` | three headings | the `harnez find` command; stem "ask the user to choose" |
| `peer-assistant` | four headings, five sentences on watching, messaging and cleanup | three commands; stem "writer per shared workspace"; identical copies in all targets |
| `wrap` | one heading, "issue index", "partial or broken", "handoff issue", "curates", "learnings" | `disable-model-invocation: true`, `/evergreen`; stem "stop background helpers" |
| `review` | five step headings | `git diff`, `git status`, `harnez issues new`; idempotent second apply |
| `issue` | "## TL;DR", two goal sentences, eleven publication sentences, the PNG sentence | five commands, `/goal`, `--force`; stems "do not fall back to a direct push", "never delete or overwrite", "stop and report"; route order; no routine image flags |
| `harnez-handoff` | five headings, six sentences | request words, profile names, four command fragments; every spec line; banned strings |
| `harnez-init` | (loosened earlier) | five names; new stem "if not initialized, stop" |
| `harnez-advisor` | "There are no portable skill flags." | five targets, identical copies |

Also done in this pass:

- `TestLiteDocsStayUnderReadLimit`: every `lite_source` must install with fewer lines than
  `max_lines` in `spec/reading_discipline.yaml`. `Make.lite.md` (118) and `GoRelease.lite.md`
  (278) are listed as exceptions; the test fails for an exception that no longer applies.
- `Tools.md`, Harnez Agent: read a worker's output in full; where the provider transcripts are.
  The pointer names the paths, not `docs/OrchestratedAgentFlow.md`, because the rule file is
  installed in projects that do not have that doc.
- `AGENTS.md`, Local Context Links: "read `@<file>` links immediately" is now "read a doc before
  the work it covers". The renderer comma as a per-doc choice stays open.

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
- Done 2026-11-13: the `@<file>` rule is reworded (owner's go). Still open: the renderer comma.
- Trim one doc at a time and rerun the 1221 task (prompt and 15 checks are recorded there) on
  `haiku:low` and `luna:low`, three runs each, because single haiku runs vary.
- Done 2026-11-13: `TestLiteDocsStayUnderReadLimit`. Still open: trim `Make.lite.md` and
  `GoRelease.lite.md` under the limit and delete their exceptions.
- Run `make test-q1` once per change. A test should assert commands and structure, not sentences.
