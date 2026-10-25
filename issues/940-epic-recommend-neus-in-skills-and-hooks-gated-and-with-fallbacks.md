# 940 — Epic: recommend neus in skills and hooks, gated and with fallbacks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Epic
**Related**: neus issue 091 (assessment, findings, options); 582, 686, 897

---

/goal Make harnez skills, managed instructions and hooks recommend neus for the query types where it
helps, only when it is available and fresh, with a fallback in every environment; verify generated
output with and without neus. Stop and report when blocked on an owner decision or a denied
permission. On a loop or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

neus now answers many questions well (`search`, `ask`, `modules guide`) and has a continuous indexer.
harnez already wraps `neus search` in `harnez find` (582), but no skill, managed AGENTS text or hook
tells agents about neus. The tools must stay decoupled: neus is optional, and guidance always reads
"if available, use `neus <verb>`, otherwise fall back to `rg` / `harnez find`".

Full findings with file:line citations: `../neus/issues/091-*.md` sections 5–7. Re-verify against live
code before each milestone.

## 2. Owner decisions (2026-10-25)

- **Strength (1B):** prefer neus conditionally for conceptual discovery (`neus search`), source-grounded
  Q&A (`neus ask`) and Go module reuse (`neus modules guide`). Keep `rg` for exact strings, graph tools
  for symbols/call chains, `harnez find issues` for issues.
- **Gate (2C):** config sets the preference (`auto|prefer|off`); a runtime check in session hooks
  decides promotion. Installation is not health: the check also uses `neus indexer status`
  (daemon running, roots fresh) and stays bounded in time.
- **Environment config (3B):** small structured policy plus an optional referenced instruction
  fragment, built on existing config sources, with explicit precedence.
- **No misleading skills (4B):** skill text always carries the conditional recipe and an explicit
  error/timeout fallback; never bake "installed" into generated text.

## 3. Milestones

- **M1 — Policy spec:** query-type → tool mapping and `auto|prefer|off` semantics, in a spec file.
- **M2 — Shared conditional guidance:** one rendered text used by Search practice, managed AGENTS
  section and relevant owned skills.
- **M3 — Hook check:** bounded PATH + `neus indexer status` check in supported session hooks; emits a
  short hint only when neus is usable. No indexing or model calls at startup.
- **M4 — Config:** structured preference and optional instruction fragment; validation, precedence, docs.
- **M5 — Fallback tests:** binary missing, backend down, timeout, stale/unindexed root, policy off,
  hook-less agents; `harnez find` falls back to a non-neus finder when neus fails (currently it does not).
- **M6 — Delivery and docs:** check embedded sources → init/apply → installed skills, AGENTS and hook
  settings; fix stale `docs/NeusIntegration.md` (it says neus has no indexer); idempotence.

## 4. Done when

All milestones are delivered, generated output is verified in a setup with neus and one without, and
the epic is closed with that evidence.
