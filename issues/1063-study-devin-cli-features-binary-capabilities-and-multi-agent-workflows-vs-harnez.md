# 1063 — Study Devin CLI features, binary capabilities, and multi-agent workflows vs harnez

**Status**: Closed — completed study in docs/studies/DevinCLIStudy.md via Haiku web search and Opus binary analysis
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Study
**Related**: 763 (Harness study vs Pi), 473 (MiniMax CLI study), 263 (Prototyping & isolation)

---

## 1. Problem & Motivation

Proprietary coding agent CLIs such as Cognition's Devin CLI provide an integrated developer experience that combines local execution with remote/cloud workflows, built-in process sandboxing, Agent Client Protocol (ACP) support, and emerging multi-agent pairing/orchestration (e.g. DevInfusion).

To inform Harnez's own harness architecture, sandbox isolation (`harnez exec` / `wrap`), and agent coordination capabilities, we need a comprehensive study cataloging all Devin CLI features, analyzing its compiled Linux binary and protocols, and comparing its capabilities against Harnez.

## 2. Technical Scope & Methodology

1. **Broad Web & Ecosystem Research**:
   * Gather external documentation, release notes, and discussions on Devin CLI capabilities.
   * Investigate multi-agent workflows, pairing patterns, and DevInfusion cooperation setups.
   * Model: `haiku:low` workers for bounded web searches and documentation summarization.

2. **Binary & Protocol Analysis**:
   * Inspect the installed ELF binary (x86_64 Rust executable from `cli.devin.ai`).
   * Extract and document:
     - Subcommands (`auth`, `mcp`, `models`, `doctor`, `rules`, `skills`, `plugins`, `cloud`, `desktop`, `sandbox`, `acp`, etc.).
     - Sandboxing mechanism: Linux `bwrap` + `seccomp` and macOS seatbelt profiles used by the exec tool.
     - Protocol interfaces: ACP (Agent Client Protocol) over stdio and cloud WebSocket bridges.
     - Configuration schemas, permission modes (`auto`, `accept-edits`, `smart`, `dangerous`), and local cache structures (`~/.local/share/devin`).
   * Model: `opus:medium` for deep decompilation/strings/symbol inspection and structural synthesis.

3. **Comparative Analysis & Feature Explanation**:
   * Compile a comprehensive study at `docs/studies/DevinCLIStudy.md`.
   * For each identified Devin CLI feature, provide:
     - An explanation of what the feature does and how it functions under the hood.
     - Direct comparison with existing `harnez` counterparts (or gap analysis where Harnez lacks the feature).
     - Potential recommendations or architectural patterns for Harnez adoption.

## 3. Goal & Execution Plan

**/goal Produce a comprehensive, verified study in `docs/studies/DevinCLIStudy.md` detailing Devin CLI features, multi-agent workflows, and binary analysis compared with Harnez, or stop and report if external binary execution or required documentation access is blocked.**

### Milestones

* **M1 — Web & Ecosystem Research**: Run targeted research via `haiku:low` on Devin CLI features, multi-agent pairing, DevInfusion, and ACP.
* **M2 — Deep Binary Analysis**: Run deep binary inspection via `opus:medium` on the `devin` binary (strings, symbols, ACP JSON-RPC methods, sandboxing arguments, config models).
* **M3 — Synthesis & Comparative Study**: Author `docs/studies/DevinCLIStudy.md` containing the complete feature catalog, feature-by-feature explanatory breakdowns, and the comparative matrix vs `harnez`.
