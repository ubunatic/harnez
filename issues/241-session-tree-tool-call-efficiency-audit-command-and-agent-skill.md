# 241 — Session-Tree Tool-Call Efficiency Audit Command and Agent Skill

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[040-agent-context-duplication-and-file-read-discipline]],
[[066-native-go-command-output-distillation]],
[[116-tool-telemetry-schema-and-storage-layer]],
[[120-harnez-stats-analytical-reporting]],
[[173-record-distillation-byte-savings-in-telemetry]],
[[176-structured-capped-subagent-completion-report-contract]],
[[204-sanitized-telemetry-and-token-export-for-datavis]],
[[212-taxonomic-classification-of-telemetry-tool-notes-for-safe-visual-analytics]]

---

## 1. Problem & Motivation

Agent sessions and their subagent sessions accumulate tool calls whose cost and quality are hard
to assess after the fact. The recurring problems are not limited to failed commands:

- successful calls can return far more text than the task needed, consuming avoidable context;
- one tool call can glue together several discovery and rendering steps (`awk` + `sed` + shell
  loops, for example), making intent opaque and results difficult to attribute or reuse;
- technically clever shell pipelines can bypass a clearer domain command or structured tool;
- repeated or weakly targeted searches can be ineffective even though each process exits zero;
- a parent-session review misses the same patterns inside its subagent descendants unless someone
  manually opens every transcript.

Existing telemetry records execution facts, ratings, output byte counts, distillation savings, and
activity categories. Existing distillation reduces some output after execution. Neither provides a
purpose-built review of a complete session tree that identifies avoidably verbose, ineffective,
hacky, over-technical, or tightly coupled multi-command calls and turns those observations into an
actionable improvement ticket.

Manual transcript browsing is also the wrong default: it is itself token-prone, risks ingesting
sensitive or irrelevant prose, and conflicts with the project's bounded log-exploration rule.

## 2. Proposed Direction

### 2.1 `harnez` command

Explore a command such as:

```text
harnez audit tool-calls [--session <id>|--current] [--include-subagents]
```

The final command name and hierarchy remain an implementation decision. Its job should be to:

1. Resolve one root session and its descendant/subagent sessions from locally available metadata.
2. Inspect structured telemetry first; read transcript fragments only when required and only with
   explicit per-call/output bounds.
3. Rank suspicious calls using explainable evidence, including where available:
   - raw and distilled output bytes or estimated token cost;
   - repeated commands/searches with low information gain;
   - non-zero exits and low `harnez rate` scores;
   - unusually long shell commands or multi-stage pipelines;
   - shell composition that duplicates an existing structured/domain command;
   - large output immediately followed by a narrower retry;
   - parent/subagent duplication of the same discovery work.
4. Emit a concise overview followed by details on demand. The default must never print entire raw
   transcripts, unbounded command output, secrets, or all low-confidence candidates.
5. Distinguish evidence from heuristic inference. A pipeline is not automatically bad, and a large
   output is not automatically wasteful when the task genuinely required it.
6. Support a machine-readable form (`--json`) suitable for an agent-facing skill and regression
   fixtures.

### 2.2 Agent skill/command

Add a thin skill/command that tells an agent to run the audit for its current session tree, inspect
the highest-signal candidates, and summarize concrete improvement opportunities. It should:

- include the current session and all available sub-sessions by default;
- avoid exhaustive transcript ingestion;
- identify the exact call/pattern, why it was costly or ineffective, and a simpler alternative;
- separate one-off agent judgment errors from missing `harnez` product capabilities;
- search the issue tracker for duplicates before proposing or filing a follow-up ticket;
- require user intent before automatically filing additional tickets unless ticket creation was
  explicitly requested.

The skill should orchestrate the command, not reimplement its parsing and scoring logic in prompt
text.

## 3. Discovery Questions

- Which agent runtimes expose a stable parent/child session relationship locally, and which require
  best-effort correlation by task ID, timestamps, working directory, or telemetry session ID?
- Does `tool_calls` currently retain enough command/output metadata to detect glued pipelines and
  low-information retries, or is a privacy-conscious schema extension required?
- How should the audit estimate token impact when only byte counts are available?
- Which findings can be deterministic, and which need agent interpretation? Prefer rules and
  explainable thresholds before any model-based classifier.
- Can transcript adapters share the existing agent-resolution work used by usage collection, or do
  Codex, Claude, AGY, and other runtimes need separate bounded readers?
- What retention and redaction rules prevent commands, paths, prompts, or output excerpts from
  leaking through JSON or follow-up issue text?
- Should the first version analyze completed sessions only, or can it safely inspect the active
  session without racing partially written transcript data?

## 4. Scope & Constraints

- Preserve `apply`/`init` separation; this is an analysis/reporting capability, not project setup.
- Reuse existing telemetry, distillation metrics, taxonomy, and privacy helpers where feasible.
- No automatic rewrite or execution of suggested replacement commands in the first version.
- No frontier-model-per-call classification. Any model-assisted review must be batched,
  opt-in or skill-driven, and grounded in a small bounded candidate set.
- Do not equate shell syntax complexity with failure. Findings must cite measurable cost,
  duplication, poor targeting, or a clearly simpler available primitive.
- Do not make raw transcripts part of durable telemetry or issue text by default.

## 5. Acceptance Criteria

- [ ] A canary documents which supported runtimes expose enough local data to reconstruct a root
  session and its subagent tree, including explicit gaps and fallbacks.
- [ ] A `harnez` command analyzes a selected/current session tree and returns a bounded ranked list
  of tool-call efficiency findings with evidence and suggested alternatives.
- [ ] Default output is concise and redacted; `--json` has a documented stable schema and contains
  no unbounded raw transcript/output fields.
- [ ] Detection covers regression fixtures for verbose output, ineffective retry chains, and an
  overly glued multi-command shell call, plus negative fixtures for justified pipelines and large
  outputs.
- [ ] At least one end-to-end fixture proves descendant subagent calls are included and attributed
  to the correct session.
- [ ] The companion skill/command delegates extraction to `harnez`, reviews only bounded findings,
  checks for duplicate issues, and produces an observation plus improvement proposal.
- [ ] Documentation explains heuristic limits, privacy behavior, runtime support, and how to drill
  into one finding without dumping a complete transcript.
- [ ] `go test ./...`, focused command tests, and a live canary against a real session tree pass.

## 6. Verification Guidance

Use synthetic transcript/telemetry fixtures for deterministic tests and one sanitized real-session
canary for integration proof. Assert byte/token budgets and maximum finding counts directly rather
than relying on visual inspection. Confirm that the command remains useful when transcript content
is unavailable but structured telemetry exists, and that unsupported runtimes fail with an
actionable diagnostic instead of silently reporting a clean audit.
