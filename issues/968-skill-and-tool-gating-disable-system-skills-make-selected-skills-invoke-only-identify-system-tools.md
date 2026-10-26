# 968 — Skill and tool gating: disable system skills, make selected skills invoke-only, identify system tools

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 601 (invoke-only wrap skill), 347/348 (disableBundledSkills research)

**Goal**: Cut the Claude Code context used by unneeded skills and tools, and gate skills that should run only on explicit request. Done when the listed items are disabled or gated and `/context` shows the lower usage, or when blocked on a user decision or denied permission, then clear the goal and write a failure report.

---

## 1. Problem & Motivation
`/context` (Haiku 5.5, 200k window) shows about 33k tokens of fixed overhead before any work starts: 5.3k for skills, 12.9k for system tools and 15k for deferred system tools. Many listed skills are never used in this repository. Some skills should run only when the user types them, but today most of them are gated only by their description text, so the model can still trigger them by mistake.

## 2. Technical Specification / Findings
- **Skill sources in `/context`:** "User" (`~/.claude/skills`), "Built-in" (`plugin-authoring`) and "claude.ai sync" (about 20 Anthropic skills such as `docx`, `pdf`, `pptx`, `xlsx`, `computer-use`, `built-in-browser`).
- **Invoke-only today:** only skills with `disable-model-invocation: true` in frontmatter are blocked from model invocation. Found in `~/.claude/skills`: `harnez-handoff`, `harnez-init`, `harnez-selftest`, `harnez-status`, `next`, `peer-assistant`, `respect`, `wrap`. Other "only on request" skills (`bug-hunt`, `epic`, `jam`, `new-project`, `harnez-advisor`) rely on their description text only.

## 3. Open Questions
1. Can a "claude.ai sync" or "Built-in" skill be disabled from its own file, from settings, or only from the claude.ai account?
2. Which settings or flags control the "System tools" and "System tools (deferred)" blocks (about 28k tokens combined)?
3. Does `harnez apply` overwrite `~/.claude/skills/<name>/SKILL.md` from a source in this repo? If so, the frontmatter change in step 2 must go into that source, not only the installed copy.
4. User decision: gate also `internal-comms`, `canvas-design`, `web-artifacts-builder`, `chrome-browser`, `gokay`, `sitegen`, `publish`, `tool-feedback-protocol`, `codebase-memory`, `codebase-memory-scout`, `codebase-memory-auditor`, and all `codebase-memory` and `harnez` MCP tools? Not applied until answered.

## 4. Implementation & Verification Plan
0. **Baseline:** run `/context` and record the Skills, System tools and System tools (deferred) totals in this ticket.
1. **Disable system skills:** `google-workspace`, `xlsx`, `pptx`, `computer-use`, `built-in-browser`, `artifact-emulator`, `deep-research`, `mcp-builder`, `slack-gif-creator`, `import-memory`, `plugin-authoring`. Answer Q1 first and record the mechanism used per skill. Check: none of them appears in the `/context` skill list.
2. **Make invoke-only:** `gokay-sweep`, `sitegen-sweep`, `reverse-sprint`, `epic`, `lean-sprint-v1`. Add `disable-model-invocation: true` to each skill's frontmatter (in the source per Q3), then run `harnez apply`. Check: each installed `SKILL.md` has the key, and a plain request matching its description does not trigger it, while its slash command still does.
3. **System tools:** answer Q2 and record the exact setting in this ticket. Apply it only if no tool this repository relies on is removed. Check: the System tools totals in `/context` drop.
4. **User decision items:** apply Q4's answer with the same mechanism and checks as step 2.
5. **Compare:** rerun `/context` and record the new totals next to the baseline.

## Decision (lean sprint 2026-10-26)

Basis (owner note, 2026-10-26): files only (docs, issues, code, YAML spec, reports); no MCP unless really needed; no
Google Workspace, Slack, Linear/Jira, cloud connectors; missing access becomes a `harnez <verb>` or flag; less
preloaded context is better; tools are git, harnez, bash, minimal OS tools, Go over Python. Default: **disable**.

Findings: searchers s968a/s968b (luna:med), refiner r968 (sol:med), advisor a968 (agy:argon:low).
- Q1: `skillOverrides` in `~/.claude/settings.json` (`"off"`, `"user-invocable-only"`) hides User and Built-in skills;
  `disableBundledSkills: true` covers bundled ones (`plugin-authoring`). claude.ai-sync skills: documented removal is
  the claude.ai account; `skillOverrides` on them is unproven (study 2026-09-16 tested only Built-in and User).
  harnez writes `skillOverrides` from `config.yaml` `preset_skill_overrides` (`internal/claude/debloat.go:235`), but
  only under `harnez apply --debloat`/`--debloat-preset` (`cmd/harnez/apply_cmd.go:128`), not plain `apply`.
- Q2: bare-name `permissions.deny` entries remove a tool definition; scoped rules only block calls. MCP tools go
  away when the server is removed; claude.ai connectors go away with `disableClaudeAiConnectors: true` (existing
  `--debloat-disable-claude-ai-connectors`) or `ENABLE_CLAUDEAI_MCP_SERVERS=false`. Keep `ToolSearch`.
- Q3: `harnez apply` regenerates managed skills from `config.yaml` `skills:` and keeps source frontmatter
  (`internal/claude/skill_content.go:25`, test `internal/claude/skill_frontmatter_test.go`).
- Measure: `claude -p "/context"` works non-interactively (study 2026-09-16); compare Skills table rows, not only
  totals (issue 348 accounting bug).

Decisions:
- D1 invoke-only: for harnez-sourced skills add `disable-model-invocation: true` to the source frontmatter in
  `docs/commands/` (applies on every plain `apply`, all harnesses); also add all five to `preset_skill_overrides`
  as `user-invocable-only`. Skills with no harnez source: override only. (Advisor changed the host's
  override-only proposal.)
- D2 disable step-1 skills: `"off"` overrides; synced skills enter `config.yaml` only if the M1 canary shows them
  gone from `/context`; otherwise the owner turns them off in the claude.ai account (listed in the report).
- D3 system tools: disable claude.ai connectors via the existing flag in the preset; bare-name deny only for
  tools proven unused; no `--tools` allowlist; keep `ToolSearch`.
- D4 Q4 answered by the owner note: gate `internal-comms`, `canvas-design`, `web-artifacts-builder`,
  `chrome-browser` as `"off"`; `gokay`, `sitegen`, `publish` as `user-invocable-only`; keep
  `tool-feedback-protocol`. codebase-memory skills and MCP: disable unless the owner says keep (asked 2026-10-26).
  harnez MCP: disable in the preset (CLI covers it).
- D5 debloat stays opt-in (`--debloat-preset`); changing plain `apply` is out of scope.

## Milestones

- **M1 evidence (no code):** record Claude version and baseline `claude -p "/context"` Skills/System tools rows in
  this ticket; canary `claude -p --settings '{"skillOverrides":{"xlsx":"off","epic":"user-invocable-only"}}' "/context"`
  and record whether each row disappears. Commit the ticket.
- **M2 config + sources + tests:** `config.yaml` (`preset_skill_overrides` per D1/D2/D4, connectors, MCP), schema in
  `spec/` if the keys change, `docs/commands/*.md` frontmatter for harnez-sourced D1 skills,
  `internal/claude/debloat_test.go` (update hardcoded override counts, idempotency, revert), frontmatter test.
  Run `make install`, `harnez apply --debloat-preset minimal`, then rerun the `/context` check and record it.
- **M3 docs:** README/debloat docs for the new entries; final before/after table in this ticket.

## M1 — Live gating canary evidence (2026-10-26)

Claude version: **2.1.310 (Claude Code)**. All three `/context` runs reported
`claude-haiku-5-5`, a 200k window, and exited successfully (exit 0). Commands ran
from `/home/uwe/projects/harnez`; overrides were session-local `--settings` JSON.
No persistent settings, account settings, or repository implementation changed.

Exact commands (stdout and stderr saved separately per run):

```bash
claude --version
HTO=0 claude -p "/context"
HTO=0 claude -p --settings '{"skillOverrides":{"xlsx":"off","epic":"user-invocable-only"}}' "/context"
HTO=0 claude -p --settings '{"deniedMcpServers":[{"serverName":"codebase-memory-mcp"},{"serverName":"harnez"}],"disableClaudeAiConnectors":true}' "/context"
```

Raw outputs are in the owner-specified scratchpad:
`/tmp/claude-1000/-home-uwe-projects-harnez/b9d7784c-d8ce-4f4e-b828-4764f82419e7/scratchpad/`
as `968-m1-version.txt`, `968-m1-baseline.txt`, `968-m1-skills.txt`, and
`968-m1-mcp.txt`. These are local evidence files, not committed artifacts.

Reported category rows (values copied as displayed; absent means no category row):

| Row | Baseline | Skill overrides | Named MCP denial + connectors off |
| --- | --- | --- | --- |
| Total context | 15.5k (8%) | 15.5k (8%) | 14k (7%) |
| Skills | 5.3k (2.6%) | 4.9k (2.4%) | 5.3k (2.6%) |
| System tools | 521 (0.3%) | 900 (0.4%) | 522 (0.3%) |
| System tools (deferred) | 6.7k (3.3%) | 6.7k (3.3%) | 6.7k (3.3%) |
| MCP tools | 673 (0.3%) | 673 (0.3%) | absent |
| MCP tools (deferred) | 11.8k (5.9%) | 11.8k (5.9%) | absent |
| MCP server instructions | 979 (0.5%) | 979 (0.5%) | 151 (0.1%) |

Observed individual rows:

| Row or server group | Baseline | Skill overrides | Named MCP denial + connectors off |
| --- | --- | --- | --- |
| `xlsx`, claude.ai sync | ~320 | absent | ~320 |
| `epic`, User | ~50 | absent | ~50 |
| `plugin-authoring`, Built-in | ~90 | ~90 | ~90 |
| `codebase-memory`, User skill | ~160 | ~160 | ~160 |
| `codebase-memory-mcp` tool rows | 17 | 17 | 0 |
| `harnez` tool rows | 8 | 8 | 0 |
| `claude_ai_Claude_Docs` tool rows | 8 | 8 | 0 |
| `plugin_harnez-channel_harnez-channel` tool rows | 1 (~150 tokens) | 1 (~150 tokens) | 1 (0 tokens) |

Conclusions and limits:
- `xlsx: off` removes the tested claude.ai-sync skill from the Skills table;
  `epic: user-invocable-only` removes the tested User skill from that table.
  This proves row removal for these two names on this version, not slash-command
  invocation behavior or removal of every other synced skill. Other synced names
  still need evidence before adding them under D2's per-skill condition.
- The named `deniedMcpServers` entries remove both registered servers' tool rows
  from `/context` without deleting their registrations. Combined with
  `disableClaudeAiConnectors: true`, the Claude Docs connector rows also vanish.
  The channel-plugin tool remains; MCP denial does not remove the separate
  codebase-memory skill or its three Custom Agents rows.
- System tools (deferred) stayed at 6.7k. System tools rose to 900 in the skill
  canary and were 522 in the MCP canary; do not infer system-tool savings from
  these runs. The displayed total stayed at 15.5k in the skill canary despite
  lower Skills usage, so individual rows are the acceptance evidence.
- M1 only: no `make test-q1`, install, apply, M2/M3 changes, or ticket closure.
