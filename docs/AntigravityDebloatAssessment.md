# Antigravity Context & Debloat Assessment

**Scope**: Assessment of Antigravity (`agy`) context usage breakdown, customization surfaces, per-tool upfront token costs, and how the `harnez apply --debloat` feature can be ported and adapted from Claude Code to Antigravity setups.

---

## 1. Measured Antigravity Context Baseline

Measured baseline context consumption on a fresh session with `Gemini 3.7 Flash` (1.0M context window):

| Category | Tokens | Share | Description & Contents |
|---|---|---|---|
| **System Tools** | **13.9k** | **1.3%** | JSON Schema definitions for built-in client tools (`run_command`, `replace_file_content`, `view_file`, `write_to_file`, `grep_search`, `find_by_name`, `list_dir`, `manage_task`, `invoke_subagent`, `define_subagent`, `manage_subagents`, `send_message`, `schedule`, `ask_question`, `read_url_content`, `search_web`, `generate_image`). |
| **System Prompt** | **6.7k** | **0.6%** | Core instructions, workspace rules (`AGENTS.md` / `GEMINI.md`), artifact conventions, communication guidelines, and slash command mappings (`/goal`, `/schedule`, `/browser`, `/plan`, `/grill-me`, `/teamwork-preview`, `/learn`, `/boost`). |
| **Skills** | **691** | **<0.1%** | Frontmatter catalog for registered/discovered skills (e.g. `antigravity-guide`, `agy-customizations`). Full skill bodies are loaded on-demand via progressive disclosure. |
| **Subagents** | **653** | **<0.1%** | Declarations and parameter specs for built-in subagent roles (`self`, `research`). |
| **User & Agent History** | **~0.1k** | **<0.1%** | Initial turn messages. |
| **Free Context Space** | **~1.0M** | **97.9%** | Remaining headroom in the active context window. |

---

## 2. Tool Surface & Upfront Pre-Use Token Costs

The system tools block injects **13.9k tokens across 17 tool definitions**. The upfront cost is driven by JSON schema type structures, parameter descriptions, instructions, and in-schema usage examples.

### Per-Tool Upfront Token Footprint

| Tool Name | Approx. Upfront Tokens | Cost Profile & Complexity Driver |
|---|---|---|
| **`schedule`** | **~1,450** | Extensive multi-mode docs (one-shot timers, 5-field cron, early termination conditions, sender matching). |
| **`replace_file_content`** | **~1,250** | Strict replacement instructions, line range rules, lint error feedback mappings. |
| **`run_command`** | **~1,200** | Persistent terminal session IDs, background execution rules, `WaitMsBeforeAsync` params. |
| **`generate_image`** | **~1,100** | Multi-image references, aspect ratio enums, artifact generation instructions. |
| **`invoke_subagent`** | **~1,050** | Array schema with model tiers (`flash`, `pro`, `inherit`), workspace modes (`share`, `branch`, `inherit`). |
| **`define_subagent`** | **~1,000** | Dynamic agent registration schema with tool capability toggles. |
| **`write_to_file`** | **~950** | Artifact metadata spec (`RequestFeedback`, `Summary`, `UserFacing`), overwrite flags. |
| **`manage_subagents`** | **~900** | Subagent lifecycle schema (`waiting_for_dependents`, `running`, `errored`). |
| **`ask_question`** | **~850** | Interactive modal questionnaire schema, option formatting, multi-select flags. |
| **`manage_task`** | **~800** | Task management schema (`list`, `kill`, `status`, `send_input`). |
| **`grep_search`** | **~750** | Ripgrep parameters (regex, glob filters, line matching, path bounds). |
| **`view_file`** | **~700** | Slice notation, byte offsets, binary handling rules. |
| **`find_by_name`** | **~650** | File search filters, extension lists, depth limits, glob patterns. |
| **`send_message`** | **~600** | Inter-agent messaging schema. |
| **`read_url_content`** | **~550** | HTTP fetch parameters & HTML-to-markdown conversion documentation. |
| **`search_web`** | **~550** | Web search query & domain filter schema. |
| **`list_dir`** | **~500** | Directory listing schema. |
| **Total** | **~13,900** | |

---

## 3. Tool Triage & Debloat Worthiness

Following `harnez`'s debloat philosophy (preserving core TDD/coding mechanics while pruning unused or high-overhead peripheral tools):

### Tier 1: Prime Debloat Candidates (Safe for Standard Coding)
These tools provide non-core peripheral capabilities and can be removed with zero friction during normal programming workflows:
* **`schedule` (~1,450 tokens)**: Cron triggers and one-shot timer callbacks. Rarely used during active interactive pairing.
* **`generate_image` (~1,100 tokens)**: UI mockups and asset generation. Irrelevant for backend/CLI/systems development.
* **`read_url_content` + `search_web` (~1,100 tokens combined)**: External web access. Unnecessary in local codebase maintenance or air-gapped environments.
* **Immediate Savings**: **~3,650 tokens (~26% of tool surface)**.

### Tier 2: Dynamic Subagent & Interactive Tools (Aggressive Preset)
These tools support advanced orchestration and UI modals, but standard single-agent TDD or static subagents (`self`, `research`) function cleanly without them:
* **`define_subagent` + `manage_subagents` (~1,900 tokens combined)**: Unnecessary if dynamic agent synthesis is not used.
* **`ask_question` (~850 tokens)**: In terminal-first or automated runs, text questions in chat replace the UI modal.
* **Additional Savings**: **~2,750 tokens**.
* **Cumulative Savings (Tier 1 + 2)**: **~6,400 tokens (~46% of tool surface)**.

### Tier 3: Retained Core Tools (Protected)
Must be preserved to maintain full coding capability:
* **File Operations**: `view_file`, `replace_file_content`, `write_to_file`.
* **Search & Codebase Exploration**: `grep_search`, `find_by_name`, `list_dir`.
* **Execution & Lifecycle**: `run_command`, `manage_task`.
* **Static Delegation**: `invoke_subagent`, `send_message`.

---

## 4. Comparison: Claude Code vs. Antigravity Mechanics

| Mechanism / Surface | Claude Code (`~/.claude`) | Antigravity (`~/.gemini/antigravity-cli`) |
|---|---|---|
| **Tool Control** | `permissions.deny: [...]` in `settings.json` strips tool schemas directly from prompt. Live mid-session reload supported. | Tool schemas injected as `default_api:*`. Configured via `settings.json` and agent tool-binding profiles. |
| **Skill Management** | Bundled skills catalog injected at startup (~2.1k tokens). Toggleable via `disableBundledSkills: true` or skill frontmatter overrides (`user-invocable-only`). | Progressive disclosure: only names and descriptions (~691 tokens) loaded initially. Full `SKILL.md` loaded on demand. |
| **Subagent Roles** | Spawned via `Agent` or `SendMessage` tool; subagent templates declared dynamically or via hooks/settings. | Built-in subagents (`self`, `research`) defined in system prompt, extended dynamically via `define_subagent` and `invoke_subagent`. |
| **Interactive Tools** | `AskUserQuestion`, `EnterPlanMode`, `ExitPlanMode`, `ReportFindings`. | `ask_question`, `generate_image`, `read_url_content`, `search_web`. |
| **Async / Task Tools** | `ScheduleWakeup`, `CronCreate`, `CronDelete`, `CronList`. | `schedule`, `manage_task`. |

---

## 5. Proposed `harnez apply --debloat` Architecture for AGY

To support Antigravity within `harnez apply --debloat`:

### 1. Unified Configuration in `config.yaml`
Extend the `debloat:` section in `config.yaml` with an `agy:` subsection:

```yaml
debloat:
  agy:
    minimal_deny:
      - generate_image
      - schedule
    aggressive_extra_deny:
      - ask_question
      - read_url_content
      - search_web
      - define_subagent
      - manage_subagents
    optional_toggles:
      disable_builtin_skills: true
```

### 2. Multi-Target Debloat Pipeline
Update `internal/claude/debloat.go` (or factor out shared logic to `internal/debloat/`):
* Detect target environment: Claude Code (`~/.claude`), Antigravity CLI (`~/.gemini/antigravity-cli`), or custom via `-t <dir>`.
* Persist pre-debloat settings in `<target>/.harnez-debloat.json` for rollback fidelity with `harnez revert --debloat`.
* Status command reporting (`harnez status --debloat`) displays active and denied tool counts per target platform.
