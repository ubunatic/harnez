# 582 — find: pluggable code and docs finders, neus first

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [Finder plugin design](../docs/studies/2026-09-25-finder-plugin-design.md)

---

## 1. Problem & Motivation

`harnez find` currently searches issues only. Add code and documentation discovery through external finder commands, with `neus` as the first external finder and useful fallbacks when it is unavailable.

## 2. Goal and Acceptance Criteria

Implement the design in [the finder plugin study](../docs/studies/2026-09-25-finder-plugin-design.md):

- Add `code` and `docs` find targets, backed by registered external finder commands configured in `~/.harnez/config.yaml` with project overrides (name, scope, command template, timeout).
- Run matching finders concurrently, merge and rank their results, and deduplicate by path plus line.
- Support `--via <name>` and JSON array or JSON Lines records with `path`, `line`, `title`, `snippet`, `score`, and `kind`.
- Expose the same search behavior through one MCP search tool.
- Make existing fuzzy issue search the first built-in finder and docs fallback; use `rg` as the code fallback when `neus` is missing.
- Adapt `neus search --json -k N <query>` and account for its pending repo-root, kind, and embedding-endpoint requirements.
- Do not add a harnez-managed index or cache.

## 3. Implementation & Verification Plan

Implement the registry, execution, result normalization/ranking, CLI targets, fallbacks, and MCP surface. Add focused tests for configuration overrides, concurrent execution, timeout/failure isolation, deduplication, ranking, and output formats. Verify CLI and MCP behavior, then update relevant documentation.
