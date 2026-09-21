# 486 — Agent roles: enforced leaf workers, role rules in spec/agent.yaml, concise role guidance for subagents

**Status**: Open

**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: #479 (epic, closed), #145, #176, `docs/practices/AgenticLoop.md`, `docs/commands/lean-sprint.md`

---

## 1. Problem & Motivation

A sprint needs a fixed shape: an orchestrator that coordinates and never codes,
and leaf workers that do the work. Nothing enforced that. A `luna:low` developer
(or the orchestrator itself) could call `harnez agent` again, start native
subagents, or hand the whole sprint to another orchestrator, so work drifted
into agents calling agents calling agents. Spawned Codex workers also had no
lineage: `HARNEZ_SESSION_ID` was never set for them, so their parent was empty
and they could manage any session.

## 2. Specification

- `spec/agent.yaml` defines `default_role` and a `roles` table. Each role has
  `spawns` (the roles it may start) and `rules` (text added to the protocol
  preamble of every turn of that role). Roles: `orchestrator` (spawns developer,
  reviewer, advisor), and the leaf roles `developer`, `reviewer`, `advisor`.
- `--role` selects the role of a new session (default `developer`, the safe
  leaf role). The role is stored on the session; `resume` keeps it and rejects a
  conflicting `--role`.
- harnez exports `HARNEZ_AGENT_ROLE` and `HARNEZ_SESSION_ID` (the session name,
  which is the parent id of anything the worker starts) to the provider process.
  A worker without a role (a human or an untracked host) is unrestricted.
- Enforcement in the CLI: for a leaf caller `start`, `resume`, `stop`, `delete`,
  `compact`, `chat`, `enable`/`disable` and every root prompt or slash command
  fail with a message saying it is a leaf worker; `list`, `status`, `models`
  stay available. An orchestrator may start only its `spawns` roles, never
  another orchestrator.
- Guidance stays concise and in one place per audience: the rules travel in the
  preamble of every turn (spec), `docs/practices/AgenticLoop.md` has a short
  "Roles and delegation depth" section plus the role table mapping, and the
  sprint commands dispatch with `--role`.

## 3. Verification

- Spec tests: roles parse and validate (undefined default, undefined or
  self-spawn, empty rules), `CheckSpawn` and `IsLeafRole` tables.
- CLI tests: role stored, child environment, preamble contains the role rules,
  default developer, unknown role, leaf refusal of every mutating verb, read-only
  verbs allowed, orchestrator may start helpers but not orchestrators, helper
  parent is the orchestrator and not itself.
- Live flow: a `luna:med` orchestrator drives a `luna:low` developer through
  tickets without either calling `harnez agent` outside its role.
