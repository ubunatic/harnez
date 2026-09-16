# 366 — Automated doc compression command/skill with LLM canary evaluation loop

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Templates / Docs / Token Efficiency
**Related**: [[065-concisemode-caveman-skill-and-output-distillation]], [[359-pilot-agenticloop-lite-md-behavioral-canary-gate]], [[362-llm-invocation-canary-harness-to-automate-lite-doc-behavioral-scoring]], [[363-dense-dos-donts-only-rewrite-of-lite-docs-validated-by-canary-lite-doc]], `docs/practices/ConciseMode.md`, `scripts/canary-lite-doc/`

---

## 1. Problem & Motivation

Documentation provided to LLMs in system prompts or agent context windows consumes precious tokens and KV-cache budget. While "lite" docs (e.g. `Bash.lite.md`, `Make.lite.md`, `AgenticLoop.lite.md`, `IssueTracking.lite.md`) achieve 50–70% token savings compared to full explanatory docs, authoring and maintaining them manually is tedious.

Techniques from the open-source **"Caveman"** project demonstrate that compressing text to high-density telegraphic fragments, symbolic rules (`->`, `|`, `x = y`), and concise dos/don'ts drastically reduces token usage without losing semantic meaning. However, naive compression risks dropping nuanced constraints or degrading model adherence.

We need a dedicated workflow (command/skill, e.g. `/doc-compress` or `harnez doc-compress`) that implements an automated end-to-end loop:
1. **Read & Analyze**: Ingest the source documentation and extract core rules, invariants, and structural sections.
2. **Compress / Clean / Aggregate / Rewrite**: Transform the doc into a dense, token-efficient format (applying telegraphic/caveman-style compaction, dos/don'ts tables, and removing conversational filler/redundant prose) while respecting structural headings (`TestLiteDocStructuralGate`).
3. **Trial Run**: Automatically generate or select small test fixtures and dispatch real isolated LLM runs (`claude -p` / subagent) using *only* the compressed doc as context.
4. **Evaluate & Lint**: Assess the model's generated output against mechanical rules via `harnez lint` / `internal/lint` (and structural gates) to guarantee zero regression in rule adherence.

---

## 2. Technical Specification

### 2.1 Workflow Pipeline

```
┌──────────────┐     ┌────────────────────────┐     ┌───────────────────────┐     ┌────────────────────────┐
│  Source Doc  │ ──> │ Compress & Rewrite     │ ──> │ Isolated LLM Trial    │ ──> │ Mechanical Evaluation  │
│  (Full text) │     │ (Caveman/Telegraphic)  │     │ (Task + Compressed)   │     │ (harnez lint / checks) │
└──────────────┘     └────────────────────────┘     └───────────────────────┘     └────────────────────────┘
```

1. **Input Phase**:
   - Accepts a target doc (e.g. `docs/lang/Go.md` or a custom user doc).
   - Identifies existing headings, rules, and known verification criteria.

2. **Compression Phase**:
   - Applies telegraphic compression rules inspired by Caveman / `ConciseMode.md`:
     - Strip articles, filler phrases, and speculative narrative.
     - Convert paragraphs to bulleted rules and structured `DO` / `DON'T` examples.
     - Preserve verbatim syntax for code snippets, commands, regexes, and file paths.
     - Retain top-level markdown headings (`##`) to satisfy structural gates.

3. **Trial Run Execution**:
   - Leverages the existing Go harness `scripts/canary-lite-doc` (or an integrated internal package).
   - Runs an isolated agent session on one or more fixture tasks designed to exercise the doc's specific rules.

4. **Evaluation & Feedback Gate**:
   - Runs `harnez lint` on the agent's output.
   - If findings > 0 or tasks fail: feeds the failure trace back into the compression loop to reinforce the failing rule.
   - Outputs a diff + token reduction stats (e.g., `-55% bytes, 0 lint findings, 100% pass`).

---

## 3. Implementation & Verification Plan

1. **Design & Command Interface**:
   - Evaluate whether this lives as a `harnez` CLI subcommand (`harnez doc compress ...`), a dedicated skill (`.claude/skills/compress-doc/SKILL.md`), or a script workflow.
2. **Harness Integration**:
   - Generalize `scripts/canary-lite-doc/` / `internal/lint` so custom docs and fixture tasks can be dynamically paired and evaluated.
3. **Canary Fixtures**:
   - Provide standard evaluation fixtures for remaining full docs (e.g., Go, Markdown, Git, Release).
4. **Verification**:
   - Run the workflow on a full doc (e.g. `docs/lang/Go.md` -> `docs/lang/Go.lite.md`).
   - Confirm byte reduction >= 30%, structural heading parity, and 0 lint errors on LLM generated code.
