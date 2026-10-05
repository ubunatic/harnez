# 715 — Remove terra from the models and the guides

**Status**: Closed — removed Terra from model specs and current guidance; old Terra sessions fail clearly (issue 715)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: `spec/agent.yaml`, [Models](../docs/Models.md), [Model roles](../docs/practices/ModelRoles.md), issue 711 (effort cost matrix)

---

## 1. Problem & Motivation
User decision (2026-10-05): drop `codex:terra` (GPT-5.6 Terra) from harnez's model
list and from the guides that recommend it. It is an older-generation model next
to Luna, Sol and Astra (GPT-6 / 6.1).

## 2. Findings
Live references at filing time (re-check on HEAD before starting):
- Spec and code: `spec/agent.yaml` (model entry, aliases such as `terra:low`),
  `config.yaml`, `internal/agentpolicy/policy.go`, tests in
  `internal/subagent/` and `cmd/harnez/agent_test.go`.
- Guides: `docs/Models.md`, `docs/ModelResearch.md`, `docs/ModelAdvisoryEval.md`,
  `docs/practices/ModelRoles.md` (copyable source; sync its root copy),
  `docs/commands/lean-sprint.md`, `docs/commands/HarnezStatus.md`,
  `docs/Roadmap.md`, `docs/README.md`, `.harnez/rules/Tools.md`,
  `.harnez/rules/Subagents.md`.
- Leave history alone: `docs/studies/`, `docs/feedback/` and closed tickets
  record past runs and stay unchanged.

Open questions to settle while working, not by guessing: where a guide names
terra for a role (e.g. "best-value auditor"), pick the replacement from the
remaining models' roles in `spec/agent.yaml` and note the choice in the commit;
whether old sessions that resolved `terra` still need a clear error rather than
a silent fallback.

## 3. Implementation & Verification Plan
Start after issue 711 is closed: 711 is editing the same `spec/agent.yaml` model
rows and `docs/Models.md`.

/goal Remove terra from the spec, code and current guides so `harnez agent models`
no longer lists it and no current guide recommends it, with tests and
`make install` green apart from known pre-existing failures; or stop and report
when blocked on a user decision (e.g. a role with no clear replacement) or a
denied permission.
