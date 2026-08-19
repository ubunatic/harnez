# 040 — Agent Context Ingestion Duplication & File Read Discipline

**Status**: Closed  
**Category**: Agentic Ergonomics / Context Optimization / Token Efficiency  
**Related**: [018 — Mark Bundled Docs in Frontmatter](018-mark-bundled-docs-in-frontmatter.md), [039 — Agentic Loop Practices](039-agentic-loop-practices-and-sprint-command.md), `AGENTS.md`

---

## 1. Problem & Motivation

When AI coding agents (such as Antigravity, Claude Code, or Codex) operate on a repository managed by `harnez`, several subtle context-window inefficiencies and duplicate file-reading behaviors occur:

1. **System Prompt Duplication (`AGENTS.md`)**:
   - `AGENTS.md` is automatically loaded into the LLM system prompt / system rules block (e.g. `<RULE[...]>` or preamble) at session startup.
   - Agents unfamiliar with their own harness frequently execute `view_file` on `AGENTS.md` when asked to explore the repository, needlessly duplicating 5+ KB of instructions directly into the conversation history.

2. **Bundled Doc Ingestion Overlap**:
   - `AGENTS.md` contains high-density summaries of bundled docs (e.g. `docs/Go.md`, `docs/Make.md`, `docs/Spec.md` via `harnez:bundled` blocks).
   - Agents often load the complete underlying doc files even when performing simple tasks where the summary in `AGENTS.md` already provides sufficient constraints.

3. **Tool Ingestion Mechanics vs. In-File Warning Headers**:
   - File inspection tools (`view_file`, `cat`, etc.) return up to 800 lines (or 46 KB) synchronously in a single tool response.
   - Once a tool response is returned, the entire output immediately and permanently becomes part of the conversation transcript and token context for all subsequent turns.
   - Placing a warning header *inside* a target doc ("If you already know this, stop reading") does **not** prevent token consumption if the tool returns the entire file at once. The warning arrives too late.

4. **Lack of Index-Level Scope Gates**:
   - Index files (`docs/README.md`) list document titles and links, but do not explicitly annotate which documents are already summarized in `AGENTS.md` or provide guidance on when *not* to read them.

---

## 2. Technical Analysis & Findings

### 2.1 Tool Execution & Context Lifetime
- **Context Injection**: Every tool call result is appended to the message history. An LLM cannot "un-see" or discard tokens returned in a previous tool step.
- **Whole-File vs. Peeking**:
  - Unbounded reads (`view_file` without line constraints) ingest full files.
  - Range-bounded peeking (`StartLine: 1, EndLine: 30`) or `grep_search` ingests only requested snippets.

### 2.2 Where Guidance Must Live
Because in-file warning banners are ineffective against full-file tool reads, deterrence must occur **before** the tool call is dispatched:
1. **In `AGENTS.md`**: Direct instruction not to re-read files that are already part of the active system prompt.
2. **In `docs/README.md` (Index/Hub)**: Annotating which documents are bundled/summarized in `AGENTS.md` vs. which contain deep reference material requiring on-demand lookup.

---

## 3. Implementation & Resolution

1. **`AGENTS.md` Directives**: Added `## Context Discipline & Token Efficiency` preventing whole-file reads on files already present in active system rules, advising `grep_search` and range-bounded reads.
2. **Global Rules Configuration**: Added Context Discipline directive in `config.yaml` (`agents_md.global.sections`).
3. **Index Annotations (`docs/README.md`)**: Added clear consultation triggers and annotated bundled language and practice docs to prevent redundant ingestion.
4. **Agentic Loop Invariants & Anti-Patterns**: Updated `docs/practices/AgenticLoop.md` with Invariant #5 (*Context Discipline & Range-Bounded Ingestion*) and Anti-Pattern ❌ *Unbounded Doc Ingestion*.
5. **Sprint Command Guidance**: Updated `commands/sprint.md` Phase 1 instructions to guide advisor subagents to use targeted grep and bounded reads.
