# Agentic Feedback Report: Subagent Domain Extraction Blindspots & Harness Tooling Gaps

- **Date:** 2026-08-18
- **Context:** Retrospective on the extraction of `ubunatic/voxi` from `harnez` (Issue 029).
- **Target:** Potential new features, rules, and commands for `harnez` itself.

---

## 1. Executive Summary

During the architectural extraction of the voice input engine from `harnez` to `voxi`, the delegated coding subagent exhibited **code-centric tunnel vision**:
- It successfully migrated Go source packages and made tests pass.
- It completely missed operational, documentation, and backlog artifacts (`scripts/canary_nested/`, `issues/021-028`, `docs/studies/2026-08-18-*.md`, and `spec/tools/voice-input.yaml`).
- It left a vestigial wrapper command in the parent CLI instead of cleanly decoupling the codebase.
- The human user had to manually nudge the agent across 4 separate review rounds to achieve a comprehensive extraction.

This report analyzes why current agent prompts and harness structures failed to prevent this, and proposes concrete features and commands that `harnez` can implement to make agent extractions and refactorings rigorous and autonomous.

---

## 2. Root Cause Analysis of Agent Blindspots

| Failure Mode | Agent Behavior | Underlying Reason / Harness Gap |
| :--- | :--- | :--- |
| **1. Source-Code Bias** | Moved `internal/tools/voice_*.go` and `cmd/` only; left `scripts/canary_nested` behind. | Agents treat "code migration" as resolving Go module imports and compiler errors rather than whole-repo domain boundaries. |
| **2. Issue & Backlog Orphanage** | Migrated code but left 7 active/research issues in `harnez/issues/`. | No tooling or harness convention exists in Harnez to audit issue ownership across repository extractions. |
| **3. Case Study & ADR Abandonment** | Left technical studies and architecture decisions in the old repo's `docs/studies/`. | Docs are treated as static baggage rather than living domain assets tied to the migrated subsystem. |
| **4. Incomplete Decoupling (Wrapper Trap)** | Built a wrapper `harnez tools voice-input` delegating to `voxi` instead of removing `tools`. | Subagents default to "minimal edit distance" and non-breaking wrappers to satisfy prompts, rather than bold architectural pruning. |
| **5. False "Done" Criteria** | Reported task complete as soon as `go build` and `go test` succeeded in the new repo. | Lack of a domain manifest verification step (e.g. verifying 0 domain keyword references remain in active code). |

---

## 3. Potential Harnez Features & Harness Innovations

The following feature concepts can be turned into concrete Harnez issues:

### Concept A: `harnez extract` / `harnez split` (Domain Extraction Planner)
A command that automates domain boundary discovery across a multi-repo workspace:
- **Keyword & AST Scanning**: Given a domain term (e.g. `voice`), scans the entire repository (`cmd/`, `internal/`, `scripts/`, `docs/`, `issues/`, `spec/`, `website/`) and outputs an exhaustive **Domain Manifest**.
- **Cross-Repo Migration**: Automatically moves matched files, rewrites Go package imports to the target module, updates issue indices, and verifies that the source repo has zero dangling references.
- **Verification Gate**: Fails if any unarchived reference to the extracted domain remains in the source project.

### Concept B: Domain Manifest & Extraction Checklist in `AGENTS.md`
Standardize instructions in `harnez init` for domain migrations:
- Require agents to produce and display a **Full Repository Artifact Manifest** (Code, Tests, Scripts, Canaries, Issues, Studies, ADRs, Docs, Systemd Units) *before* making changes.
- Add an explicit rule: *"When extracting or pruning a capability, wrappers are forbidden unless explicitly requested. Prefer complete removal and clean interfaces."*

### Concept C: Multi-Repo Workspace Issue & Doc Auditor (`harnez audit`)
- Check for orphaned issues referencing removed code or extracted submodules.
- Check for broken links between `docs/README.md` and deleted case studies/ADRs.
- Verify that issues in `issues/archive/` link to their destination if moved.

---

## 4. Action Items & Derived Issues

1. **Issue: Add Domain Extraction & Pruning Rules to `AGENTS.md` Template**
   - Update `internal/claude/config.go` and bundled templates to include explicit extraction checklist rules for coding agents.
2. **Issue: Pre-Extraction Discovery Protocol in Canary/Spec Docs**
   - Update `docs/other/Spec.md` and `docs/other/Canary.md` with guidelines on whole-repository inventory before refactoring.
3. **Issue: Explore `harnez extract` command for workspace repository slicing**
   - Plan a declarative extraction mechanism for splitting monolithic utilities into standalone tools managed under `.uman.toml`.
