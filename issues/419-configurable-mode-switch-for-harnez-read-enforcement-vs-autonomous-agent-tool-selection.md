# 419 — Configurable mode switch for harnez read enforcement vs autonomous agent tool selection

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Medium
**Category**: Architecture / Context Optimization / Developer Experience
**Related**: #405, #416, #418, `docs/practices/AgenticLoop.md`, `config.yaml`

---

## 1. Problem Statement & Motivation

`harnez read` provides high-density, token-efficient visual context cards (via `-I`) and line-bounded reads (`-L`). Currently, `harnez` enforces file reading discipline strictly through PreToolUse lifecycle hooks (blocking native `view_file`, `View`, `Read`, `ReadMultipleFiles` on files $\ge 100$ lines with denial errors and redirection commands like `harnez read --auto`).

While strict enforcement guarantees low context token usage during high-density operations, it prevents empirical benchmarking and head-to-head comparison of different operational paradigms:
1. **Enforced Harness Mode**: Hard intercepts and redirects via PreToolUse lifecycle hooks.
2. **Autonomous / Agent-Native Mode**: Agents freely decide when to invoke native tools (e.g. `view_file` for targeted inspection) vs `harnez read -I <file>` to generate dense visual PNG cards when ingesting medium/large documentation or source files (without `--auto` redirects forcing behavior).

Furthermore, promotion of `harnez read` is currently embedded across multiple configuration files, prompt templates (`AGENTS.md`, `AgenticLoop.md`), skills, and hook implementations. To allow toggling between modes seamlessly, we need a clean separation between **active hook enforcement** and **soft recommendations for large files**.

---

## 2. Requirements & Design Decisions

1. **Simple Toggle Switch**:
   - Provide a clear, persistent switch (e.g. `harnez mode [enforce-read|autonomous-read]` or configuration setting in `~/.harnez/config.yaml` / environment variable `HARNEZ_READ_ENFORCE=0|1` / hook setting) to toggle reading discipline enforcement.
   - When **Enforcement is OFF**:
     - PreToolUse hooks (`harnez hook agy`, `harnez hook read`) allow native read tools (`view_file`, `View`, `Read`, `ReadMultipleFiles`) without intercepting or denying.
     - Telemetry continues to observe and record post-tool execution and opportunity cost metrics (`harnez hook agy-post`) for analytical comparison.
     - Agents retain discretion to explicitly run `harnez read -I <file>` when desired for dense visual context cards.
     - Prohibit `--auto` redirects when enforcement is disabled.
   - When **Enforcement is ON**:
     - Strict PreToolUse interception and redirection rules apply to large/unbounded reads.

2. **Clean Separation of Prompt Recommendations vs Hard Rules**:
   - Audit and clarify `AGENTS.md` and `docs/practices/AgenticLoop.md` so that the guidance establishes `harnez read -I` as the *recommended best practice for large/medium files* while making runtime enforcement conditional on the active mode switch.
   - Ensure skills and command definitions do not break or fail when enforcement is toggled off.

3. **Comparative Evaluation & Benchmarking**:
   - Enable direct measurement of token consumption, tool frequency, error rates, and completion velocity between autonomous agent runs and enforced runs via `harnez stats`.

---

## 3. Implementation Milestones

### Milestone 1: Toggle Switch & Hook Passthrough Logic
- Introduce a configuration option / environment flag (`HARNEZ_READ_ENFORCE` / `~/.harnez/config.yaml` setting) checked inside `evaluateReadToolDiscipline` and `runClaudeReadHook` / `runAgyToolHook`.
- When disabled, bypass deny decisions and allow native read tools immediately.

### Milestone 2: Telemetry & Opportunity Cost Observer Preservation
- Ensure post-tool observation hooks (`runAgyPostToolHook`) continue capturing output metrics and estimating opportunity savings even when enforcement is OFF, providing ground-truth comparison data.

### Milestone 3: Documentation, Prompts & Skills Decoupling
- Review and refine `AGENTS.md`, `docs/templates/AGENTS.md`, `docs/practices/AgenticLoop.md`, and skills to distinguish between the visual context card recommendation and hook-level enforcement mechanics.

### Milestone 4: Verification & CLI Mode Switch Control
- Add unit and integration tests verifying both enabled and disabled switch states across Claude Code and Antigravity hooks.
- Provide a convenient CLI command or flag to inspect and set the mode.

---

## 4. Verification & Acceptance Criteria
- `go test ./...` verifies hook behavior under both `HARNEZ_READ_ENFORCE=1` (deny large reads) and `HARNEZ_READ_ENFORCE=0` (allow native reads).
- Telemetry properly records invocations regardless of mode switch state.
