# 419 — Configurable mode switch for harnez read enforcement vs autonomous agent tool selection

**Status**: Closed — Works as expected
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

1. **System-Level & Declarative Configuration**:
   - Primary configuration resides in `~/.harnez/config.yaml` (e.g. `reading_discipline.enforce: true|false`).
   - `HARNEZ_READ_ENFORCE=0|1` is supported as an optional per-process override.
   - CLI commands (`harnez mode autonomous-read` / `harnez config`) provide an ergonomic, persistent interface to toggle the switch.

2. **Permanent Hook Registration & Traceability (Observer Mode)**:
   - All lifecycle hooks (`PreToolUse`, `PostToolUse`) remain permanently registered in `~/.claude/settings.json` and Antigravity hook configs.
   - Hooks are **not removed** when enforcement is turned off. Instead, they operate in **Observer / Passthrough Mode**:
     - Pre-tool hooks log the invocation to telemetry and immediately permit native reads without issuing denials or redirects.
     - Post-tool observation hooks continue capturing execution metrics, token footprints, and opportunity cost estimates.
   - When enforcement is turned ON, pre-tool hooks actively intercept and redirect large/unbounded reads.

3. **Hermetic Test Isolation & Dependency Injection**:
   - Hook options (`readHookOptions`, `agyHookOptions`) accept explicit configuration flags (`EnforceRead *bool`) so test suites remain fully isolated from ambient environment variables.

4. **Clean Separation of Prompt Recommendations vs Hard Rules**:
   - Audit and clarify `AGENTS.md` and `docs/practices/AgenticLoop.md` so that guidance establishes `harnez read -I` as the *recommended best practice for large/medium files* while making runtime enforcement conditional on the active mode switch.

5. **Comparative Evaluation & Benchmarking**:
   - Enable direct measurement of token consumption, tool frequency, error rates, and completion velocity between autonomous agent runs and enforced runs via `harnez stats`.

---

## 3. Implementation Milestones

### Milestone 1: Toggle Switch, Passthrough Logic & Config-First Integration
- Introduce `readEnforcementEnabled()` checking declarative config `~/.harnez/config.yaml` with fallback to `HARNEZ_READ_ENFORCE` env override and default-on behavior.
- Support dependency injection (`EnforceRead *bool`) in `readHookOptions` / `agyHookOptions` to ensure test suite isolation.
- When disabled, bypass deny decisions and allow native read tools immediately while logging telemetry.

**Delivered (2026-09-18):** `readEnforcementEnabled` now checks the optional
`HARNEZ_READ_ENFORCE` per-process override, then `~/.harnez/config.yaml` at
`reading_discipline.enforce`, with enforcement enabled by default. Hook options
accept `EnforceRead *bool` for hermetic tests and explicit callers. Claude and
Antigravity pre-tool hooks preserve passthrough and telemetry behavior.

### Milestone 2: Telemetry & Opportunity Cost Observer Preservation
- Decouple large read discipline detection from denial actions so that post-tool observation hooks (`runAgyPostToolHook`) and telemetry record candidate large-read opportunities even when enforcement is OFF, providing ground-truth comparison data.

**Delivered (2026-09-18):** Antigravity pre-tool telemetry independently evaluates
large native reads and marks allowed observer-mode calls with
`reading_discipline:opportunity`. Enforcement denials retain their existing
`reading_discipline:intercepted` marker, and post-tool hooks continue recording
actual tokens plus estimated savings for native reads in either mode.

### Milestone 3: Documentation, Prompts & Skills Decoupling
- Review and refine `AGENTS.md`, `docs/templates/AGENTS.md`, `docs/practices/AgenticLoop.md`, and skills to distinguish between visual context card recommendations and hook-level enforcement mechanics.

**Delivered (2026-09-18):** Agent guidance now presents `harnez read` as the
recommended context-efficient workflow while documenting native reads as valid
for targeted inspection. The template and AgenticLoop guidance identify hook
blocking as conditional on `reading_discipline.enforce` / `HARNEZ_READ_ENFORCE`,
separating prompt recommendations from runtime policy.

### Milestone 4: Verification & CLI Mode Switch Control
- Add unit and integration tests verifying both enabled and disabled switch states across Claude Code and Antigravity hooks under isolated test conditions.
- Provide a convenient CLI command (`harnez mode [enforce-read|autonomous-read]` or `harnez config`) to inspect and toggle the mode.

**Delivered (2026-09-18):** Added persistent `harnez mode enforce-read` and
`harnez mode autonomous-read` commands backed by
`~/.harnez/config.yaml`. Synchronized the embedded config template and root
guidance wording, and verified the complete suite under the autonomous-mode
test environment.

### Milestone 5: Final Hardening

**Delivered (2026-09-18):** Added hermetic CLI tests covering both persistent
read-mode switches, confirmed the configuration output, and re-ran the full
quota-1 suite successfully.

---

## 4. Verification & Acceptance Criteria
- `go test ./...` passes cleanly regardless of whether `HARNEZ_READ_ENFORCE=0` or `1` is present in the shell.
- Telemetry properly records invocations and opportunity cost metrics regardless of mode switch state.
- `harnez mode enforce-read` and `harnez mode autonomous-read` persist the selected mode.
