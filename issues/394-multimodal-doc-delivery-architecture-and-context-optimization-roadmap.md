# 394 — Multimodal doc delivery architecture and context optimization roadmap

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Low
**Category**: Architecture & Design
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[388-unified-cross-harness-skill-installation-in-harnez-apply-for-claude-code-agy-codex-and-prime]], [[389-make-global-doc-installation-in-harnez-apply-opt-in-via-flag-with-default-zero-global-docs]], [[390-add-named-docs-profiles-in-config-yaml-with-cli-expansion-across-apply-and-init]]

---

## 1. Problem & Motivation

With global skills unified across all four agent harnesses (`claude`, `agy`, `codex`, `prime`), global docs made opt-in with zero default global payload, and visual cheatsheets proven to offer 2x–7x token compression, Harnez needs a formalized architecture roadmap for context delivery:
1. **Tier 1: Global Behavioral Glue**: Flat `CLAUDE.md` / `AGENTS.md` with zero doc inlining, relying on prompt prefix caching.
2. **Tier 2: Universal Autonomous Capabilities**: Dynamic Skills (`~/.<harness>/skills/`) with progressive disclosure metadata.
3. **Tier 3: Bounded Visual Cheatsheets**: High-density rendered image cards for subagent dispatch and on-demand reference.
4. **Tier 4: Project-Local Evergreen Rules**: Repository `./AGENTS.md` and `./docs/` via `harnez init`.

---

## 2. Acceptance Criteria

- [ ] Architectural synthesis doc in `docs/` summarizing multi-tier context distribution.
- [ ] Integration recommendations for future `harnez mode` and subagent dispatch flags.
