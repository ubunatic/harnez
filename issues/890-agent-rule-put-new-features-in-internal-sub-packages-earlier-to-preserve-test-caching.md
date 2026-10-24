# 890 — agent rule: put new features in internal sub-packages earlier to preserve test caching

**Status**: Closed — duplicate of 877, delivered there (f0430be5)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Practice
**Related**: 877 (sub-package convention), 881 (harnez test catalog), loom 377, loom 378, cati 111, cati 112, docs/lang/Go.md

---

/goal Define a brief, concise, generic Go rule in harnez's default agent instructions (bundled `docs/lang/Go.md` and/or AGENTS.md template) that instructs agents to place substantive features and tests into `internal/<feature>` sub-packages early rather than accumulating monolithic `cmd/` packages, keeping default agent prompts short and easily actionable; or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Across multiple repositories (`loom`, `harnez`, `cati`), we have observed a recurring pattern: coding agents repeatedly add feature logic, state management, and heavy tests directly to `cmd/<app>` or root packages.

Eventually, `cmd/` packages swell to 15,000–45,000 lines (e.g. `cmd/harnez` with 46k lines across 182 files, `cmd/loom` with 6.3k test lines, `cmd/cati` with 17.8k lines). Because Go caches test results per package, modifying any minor command or sub-feature busts the cache for the entire command package, forcing 40–90 second test runs and excessive CPU/memory usage.

Agents do not proactively prevent this accumulation unless explicitly instructed in their base prompt.

## 2. Requirement: Brief, Actionable Agent Rule
The rule must be concise and actionable—stating *what to do* directly without lengthy prose or rationale. Default agent instructions must stay short to minimize token consumption and cognitive overhead.

### Proposed rule text for bundled Go guidelines (`docs/lang/Go.md` / AGENTS template):
> **Modular Sub-packages**: Place new features, substantive command logic, and tests in `internal/<feature>` sub-packages from day one. Keep `cmd/<app>` thin (only flag registration, help, and exit routing) to preserve Go package-level test caching.

## 3. Implementation & Verification Plan
1. Add the concise rule to `docs/lang/Go.md` under Project Layout / Best Practices.
2. Ensure `harnez init` and AGENTS template propagate this rule to managed Go repositories.
3. Verify that the rule remains brief (1–2 sentences) and contains no redundant prose.
