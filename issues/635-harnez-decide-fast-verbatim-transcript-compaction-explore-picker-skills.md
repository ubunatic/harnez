# 635 — harnez decide: fast verbatim transcript compaction & explore/picker skills

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/633, issues/631, docs/practices/AgenticLoop.md, docs/Search.md

---

## 1. Problem & Motivation
Standard context compaction in agent harnesses (e.g. Claude Code built-in summarization) suffers from two major drawbacks:
1. **High latency & cost**: Summarizing 50-100k tokens using general LLMs takes 15-30+ seconds and consumes substantial tokens.
2. **Lossy summarization**: Summarizing loses vital exact details: file paths, line ranges, error messages, compiler warnings, or exact commands needed later in the session.

Inspired by `tamaratran/fast-jev-compaction` and `disler/ten-levels-of-jev` (Levels 7, 8, 9, 10):
- Verbatim compaction scores every past tool call and result. Unnecessary outputs are pruned or truncated while preserving messages verbatim.
- In addition, agent navigation can be dramatically accelerated using decision-based ranking:
  - **Fast Explore**: Keyword search generates candidate files; the decision model ranks 20-40 files in parallel in ~1.5s, allowing the agent to read only top-ranked files.
  - **Skill Picker**: Evaluates available skills against user prompt to recommend or select exactly one relevant skill (or none), reducing context noise from unused skill prompts.
  - **Browser & Action Picker**: Given interactive UI elements or workflow branches, selects the next targeted interaction.

## 2. Technical Specification / Findings

### 1. Verbatim Transcript Compaction (`harnez compact`)
Directly integrates or adapts the `fast-jev-compaction` strategy:
- Pin recent $N$ messages and initial user instructions.
- Construct conversation state (messages + abridged tool calls).
- For each older tool call, evaluate via `harnez decide`:
  1. `keep_call`: Does the context need to know this tool was run with these arguments?
  2. `keep_result`: Does the verbatim output still matter?
- Gating policy:
  - Both true -> keep intact.
  - `keep_call` true, `keep_result` false -> truncate result to head (e.g. 200 chars) + note.
  - Both false -> excise both call and result.
- Rebuild transcript verbatim without lossy paraphrasing.
- Latency: < 1-2 seconds across parallel batch requests.
- Expose via CLI `harnez compact --transcript <file> --model <model>` and optional Claude Code compaction hook plugin.

### 2. Fast Explore & Ranking Skill (`harnez find --rank`)
- Augments `harnez find code` / `harnez find docs`:
  - When a query yields 15-50 candidate files, instead of feeding all file snippets to the agent:
  - Batch score files using `harnez decide --model <model>` on relevance to the user's intent.
  - Returns the top 3-5 ranked files with confidence scores.

### 3. Skill & Agent Picker
- Given the user's initial prompt and installed skills metadata (`name` + `description` from registry, issue 631):
- Issues a single `choice` question to the decision model: "Which skill is best suited for this task? (Options: [skill1, skill2, ... none])".
- Provides zero-overhead skill activation without bloating the main system prompt with full skill texts.

## 3. Implementation & Verification Plan
1. **Compaction Engine**:
   - Implement transcript parser & rebuild logic in `internal/decide/compact.go`.
   - Wire `harnez compact` CLI command.
   - Claude Code compaction hook plugin template.
2. **Search / Explore Ranking**:
   - Add `--rank` flag to `harnez find` utilizing `internal/decide`.
3. **Skill Picker Hook/Skill**:
   - Implement skill selection helper in `internal/skillreg` or standalone helper script.
4. **Verification**:
   - Test compaction against realistic transcript sessions (verify zero loss of pinned items, valid JSON structure, expected truncation).
   - Test ranking accuracy against test fixtures with known relevant files.
