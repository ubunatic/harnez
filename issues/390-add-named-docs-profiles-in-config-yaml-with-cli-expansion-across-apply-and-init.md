# 390 — Add named docs_profiles in config.yaml with CLI expansion across apply and init

**Status**: Closed — added docs_profiles (core, dev, full) with CLI expansion
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[389-make-global-doc-installation-in-harnez-apply-opt-in-via-flag-with-default-zero-global-docs]], [[388-unified-cross-harness-skill-installation-in-harnez-apply-for-claude-code-agy-codex-and-prime]]

---

## 1. Problem & Motivation

Following Issue 389, `harnez apply` defaults to zero global docs (`docs: []`), and allows opt-in installation of specific documentation files via `--docs` / `-d`.

To make it effortless for users to selectively install cohesive subsets of documentation without listing individual files manually, `config.yaml` should support a `docs_profiles:` mapping (e.g. `core`, `dev`, `full`). Both `harnez apply --docs <profile>` and `harnez init --docs <profile>` should expand profile names to their underlying documentation lists deterministically.

---

## 2. Technical Specification

1. **`config.yaml`**:
   - Add `docs_profiles:` mapping with:
     - `core`: `[agentic-loop, issue-tracking]`
     - `dev`: `[agentic-loop, issue-tracking, bash, git]`
     - `full`: (all language and workflow docs)
2. **`internal/claude`**:
   - Update `Config` struct in `config.go` with `DocsProfiles map[string][]string`.
   - Update `expandDocNames(cfg, names)` to expand profile names into their constituent doc list, supporting combinations like `--docs core,golang`.
   - Update `validateDocNames` to recognize profile names as valid inputs.
3. **Tests & Verification**:
   - Add unit tests verifying profile expansions in `internal/claude`.
   - Verify `make test-q1`.

---

## 3. Acceptance Criteria

- [ ] `docs_profiles` is declared in `config.yaml` (`core`, `dev`, `full`).
- [ ] `expandDocNames()` resolves profile names deterministically.
- [ ] Both `apply --docs <profile>` and `init --docs <profile>` accept profile names and expand them properly.
- [ ] Unit tests pass under `make test-q1`.
