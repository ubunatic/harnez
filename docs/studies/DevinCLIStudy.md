# Devin CLI Feature Analysis, Binary Capabilities, and Harnez Comparison

**Date**: 2026-10-31  
**Issue**: [#1063](file:///home/uwe/projects/harnez/issues/1063-study-devin-cli-features-binary-capabilities-and-multi-agent-workflows-vs-harnez.md)  
**Binary Version**: `3000.11.234` (commit `92c83011d113`, distribution `curl-bash`)  
**Primary Sources**:
- Ecosystem and web research via `claude-haiku-5-5` (`docs/studies/scratch-devin-web-research.md`)
- Static ELF inspection of `devin` binary (`217.5 MB`, stripped static-PIE Rust) and 191 Clap-generated man pages via `claude-opus` (`docs/studies/scratch-devin-binary-analysis.md`)

---

## Executive Summary

The official Cognition Devin CLI is a terminal client and autonomous coding agent built on a Rust codebase (internal workspace `chisel`, incorporating Codeium/Windsurf lineage). It operates in two core modalities:
1. **Local Agent**: Runs locally in the terminal with OS-level sandboxing (`bwrap` on Linux, seatbelt on macOS).
2. **Cloud Bridge**: Connects to remote Devin cloud sandboxes via WebSockets, SSH passthrough (`devin ssh`), port forwarding (`devin forward`), and Declarative Repo Setup (`devin cloud drs`).

Its multi-agent capability spans two tiers: **Subagents** (modular, hierarchical workers like `subagent_explore` and `subagent_general`) and **Devin Fusion** (a dual-model pairing pattern coupling an expensive frontier "Lead" planner with a fast "Sidekick" mechanical implementer).

---

## 1. Complete Catalog of Devin CLI Features

### 1.1 Global Invocations & Flags

| Flag | Short | Environment Variable | Functionality |
| :--- | :--- | :--- | :--- |
| `--model <MODEL>` | | `DEVIN_MODEL` | Sets provider model (e.g. `claude-sonnet-4`, `claude-opus-4.6`, `codex`, `opus`). Resumed sessions can switch models. |
| `--permission-mode <MODE>` | | `DEVIN_PERMISSION_MODE` | Governs auto-approval: `auto` (read-only auto-approved), `accept-edits` (workspace writes auto-approved), `smart` (fast auxiliary model judges safety), `dangerous` / `bypass` (auto-approves all). |
| `--sandbox` | | `DEVIN_SANDBOX` | Enforces OS process isolation (`bwrap` + seccomp on Linux). Forces `autonomous` permission mode. |
| `--cloud` | | | Connects REPL to Devin Cloud VM instead of local agent. |
| `--continue` | `-c` | | Continues the most recent conversation in the current working directory. |
| `--resume [<ID>]` | `-r` | | Resumes a conversation by session ID or interactive picker; accepts cloud URLs with `--cloud`. |
| `-p, --print [<PROMPT>]` | | | Non-interactive execution; prints response to stdout and exits. |
| `--prompt-file <FILE>` | | | Reads initial user prompt from file. |
| `--config <PATH>` | | | Custom configuration file (overriding `~/.config/devin/config.json`). |
| `--export [<PATH>]` | | | Serializes conversation turns (ATIF format) after every step. |
| `--respect-workspace-trust` | | | Enforces directory trust check; default `true` (non-interactive fails if untrusted). |
| `--verbosity <0-3>` | | `DEVIN_VERBOSITY` | Output detail level (`0` quiet, `1` compact, `2` detailed, `3` verbose). |

---

### 1.2 Subcommand Tree

```text
devin
├── auth         login [--force-manual-token-flow] | logout | status
├── mcp          add | list | get | remove | login | logout | enable | disable
├── models       list [--format text|json]
├── doctor       [--json]
├── rules        list [--provider cursor|windsurf] | show <NAME> | paths
├── skills       list [--trigger user|model] [--json] | show <NAME> | paths
├── plugins      install <SRC> [-y] [--local] | list | info | update [NAME] | remove | prune
├── cloud        REPL on cloud sessions (same as --cloud)
│   └── drs      Declarative Repo Setup: whoami, sandbox-create, run, blueprint-*, secret-create
├── desktop      Launch or attach to Devin Desktop GUI
├── list / ls    List local/cloud sessions [--format interactive|json|csv]
├── rm           Delete session by ID or name [--force]
├── ssh          SSH directly into a cloud session VM [passthrough flags, -L, scp]
├── forward      Port forward from cloud session VM to localhost
├── update       In-place self-updater
├── version      Print version and commit hash
├── migrate      Import hooks or workflows from other harnesses (Cursor, Windsurf, Claude)
├── sandbox      setup (checks prerequisites: bwrap, socat)
├── setup        Interactive setup and auth wizard
├── uninstall    Clean removal of binaries, versions, and configuration
└── acp          Run as Agent Client Protocol (JSON-RPC) server over stdio
```

---

### 1.3 Multi-Agent Cooperation & Pairing Workflows

#### Devin Fusion (Frontier / Sidekick Pairing)
* **Concept**: Often referenced colloquially as "agent pairing" or "DevInfusion", **Devin Fusion** is Cognition's dual-model architecture designed to maximize reasoning while curbing token expenditure.
* **Roles**:
  * **Lead Model**: Frontier model (e.g. Claude Opus 5.5). Focuses exclusively on system decomposition, planning, milestone breakdown, and architectural review.
  * **Sidekick Model**: High-speed, lower-cost model (e.g. GPT-6 Luna, Fable 5.1). Executes mechanical edits, file searching, syntax checks, and test runs.
* **Context Isolation**: The Lead and Sidekick do not share full conversational transcripts. Instead, the Lead generates a compact brief/specification, the Sidekick executes and produces a verified result diff/summary, and the Lead reviews the outcome. This protects the Lead's context window and preserves prompt cache locality.

#### Subagent Profiles
* **Built-in Profiles**:
  * `subagent_explore`: Read-only scout restricted from modifying files or calling arbitrary network URLs; designed for fast codebase discovery.
  * `subagent_general`: General worker with tool access matching parent permissions.
* **Concurrency & Management**:
  * Subagents run in either foreground (parent pauses until worker finishes) or background (parallel background execution).
  * `Ctrl+B` pushes a foreground subagent into the background; `f` foregrounds, `x` terminates.
  * Park-and-Resume: Interrupting the parent conversation parks running subagents without losing state; they resume on subsequent turns.
* **Custom Profiles**: Authored in `.devin/agents/<name>.md` with YAML frontmatter specifying `allowed-tools`, `model`, and `max-nesting`.

---

## 2. Feature-by-Feature Explanatory Breakdown

### Sandboxing & Process Isolation (`--sandbox`)
* **How it works**: Uses Bubblewrap (`bwrap`) on Linux and Seatbelt profiles on macOS. The root filesystem (`/`) is mounted read-only (`--ro-bind / /`), `/tmp` is placed on `tmpfs`, and write permissions are confined exclusively to the active workspace directory and explicitly requested `Write(...)` scopes.
* **Dynamic Scope Expansion**: When a command attempts to write outside the workspace (e.g. `/var/log` or a global cache), the agent calls an internal `request_scope` tool to prompt the user for permission.
* **Seccomp & Network Proxying**: The binary includes hooks for `apply-seccomp` and local HTTP/SOCKS proxies (`CLAUDE_CODE_HOST_HTTP_PROXY_PORT`), routing traffic through an unshared network namespace (`--unshare-net`) with loopback/domain filtering (`sandbox.allowed_domains`).

### Agent Client Protocol (ACP) (`devin acp`)
* **How it works**: Devin implements the standard Agent Client Protocol (ACP) over stdio using JSON-RPC 2.0. Rather than coupling tightly to a specific IDE, editors like Zed launch `devin acp` as an external subprocess.
* **Internal Architecture**: The interactive terminal UI itself (`devin`) internally launches `devin acp` as a subprocess and acts as its local client. When `--cloud` is passed, `devin acp` bridges JSON-RPC frames over a secure WebSocket to `api.devin.ai`.

### Declarative Repo Setup (DRS) & Blueprints (`devin cloud drs`)
* **How it works**: Defines declarative container/VM setup configurations via `environment.yaml` blueprints. A blueprint describes base packages, system dependencies, secrets, and setup scripts required to build a cloud dev environment.
* **Build & Cache Pipeline**: Blueprints are compiled via `devin cloud drs build-start` into cached snapshot images. Subsequent cloud runs start instantly from the cached VM image.

### Cloud Bridge, SSH, and Forwarding (`devin ssh`, `devin forward`)
* **How it works**: Every cloud Devin session runs inside a dedicated remote microVM. `devin ssh` negotiates SSH credentials and establishes an authenticated tunnel into the cloud container, supporting interactive shells, `scp`, and arbitrary SSH options.
* **Port Tunneling**: `devin forward <session> 8080:3000` sets up multiplexed local port tunnels to preview web servers or databases running inside the remote Devin VM.

### Extensibility: Rules, Skills, Plugins, and Cross-Tool Importer
* **Hierarchical Rules**: Evaluates `.devin/config.json`, `AGENTS.md`, and project `.devin/rules/*.md`.
* **Cross-Tool Importer (`read_config_from`)**: Automatically detects and parses existing configuration from sibling tools:
  * Reads `.cursor/rules/*.md[c]`
  * Reads `.windsurf/rules/*.md`
  * Reads `.claude/` (skills, hooks, subagents)
  * Reads `.github/skills/`
  * Imports MCP server definitions from `opencode.json` and `.zed/settings.json`.

---

## 3. Detailed Comparison: Devin CLI vs Harnez

| Dimension | Devin CLI (`3000.11.234`) | Harnez (`ubunatic/harnez`) | Architectural Trade-offs & Notes |
| :--- | :--- | :--- | :--- |
| **Primary Architecture** | Monolithic Rust binary (`chisel`, 217 MB static ELF). Integrates terminal UI, ACP client, local runtime, and cloud bridge. | Modular Go CLI (`harnez`) + multi-vendor agent harness. Manages sessions, Git worktrees, hooks, and external providers. | Devin packages an entire agent engine; Harnez acts as an orchestrator and governance layer over multiple frontier agents (Claude, Codex, AGY, Pi). |
| **Sandboxing & Isolation** | Built-in `bwrap` + seccomp on Linux, seatbelt on macOS. Enforces `Write(...)` scopes and read denies. | `harnez exec` with Quota-1 `bwrap` sandbox (`quota1SandboxArgs`), read-only root, tmpfs runtime dir, and project bind mounts. | Both rely on Linux `bwrap` namespaces. Devin integrates dynamic runtime scope prompts; Harnez enforces strict single-test execution boundaries (Quota-1). |
| **Multi-Agent Orchestration** | **Devin Fusion** (Lead/Sidekick model pairing) + built-in subagents (`subagent_explore`, `subagent_general`). | **5-Phase Agentic Sprint** (`AgenticLoop.md`), Sequential Advisor, Reusable Developer, Independent Reviewer Gate, and `harnez agent start`. | Devin optimizes pairing for prompt-cache token savings (two models in one session). Harnez enforces multi-vendor checks and structured review gates across independent subagent lifecycles. |
| **Client / IDE Protocol** | **ACP (Agent Client Protocol)** over stdio JSON-RPC. First-class Zed and Devin Desktop support. | Unix CLI, interactive TUI (`harnez usage`), stdout streaming, and MCP Server (`harnez_spawn_agent`, `harnez_find`). | Devin acts as an ACP server for IDEs; Harnez acts as an MCP server and harness provider for orchestrators. |
| **Cloud Infrastructure** | Devin Cloud VMs, Declarative Repo Setup (`drs` blueprints), cloud SSH tunnels, port forwarding. | Local-first; interacts with standard Git remotes (GitHub, Codeberg), container testing (`podman`), and local LLM runners (`lmcoder`). | Devin provides managed proprietary cloud compute; Harnez prioritizes local developer machine sovereignty and rootless containers. |
| **Ecosystem Compatibility** | Built-in importers for Cursor, Windsurf, Claude Code, OpenCode, and Zed. | Universal rule synchronizer (`harnez init`/`apply`) syncing `AGENTS.md`, `.harnez/rules/`, and `.claude/settings.json`. | Devin imports sibling tool configs; Harnez provides single-source-of-truth doc/rule deployment across repositories. |
| **Telemetry & Observability** | OpenTelemetry (`chisel-otel`) and Sentry instrumentation reporting to Cognition servers. | Local SQLite database (`~/.harnez/tool_stats.db`) recording exact tool execution, exit codes, durations, and quota metrics. | Harnez guarantees zero data exfiltration by storing telemetry in local SQLite tables. |

---

## 4. Key Architectural Takeaways for Harnez

1. **Formalize a "Lead / Sidekick" Dual-Model Runner**:
   Devin Fusion's primary insight is that passing full conversational transcripts between models destroys cache efficiency and wastes tokens. Harnez's `/sprint` and `factory-sprint` workflows can adopt this structured brief-and-diff exchange protocol: a frontier planner (Opus) generates a milestone specification, a lower-tier developer (Luna/Haiku) executes the diff, and the frontier model verifies the patch without re-reading intermediate execution churn.
2. **First-Class `harnez wrap` Command**:
   Devin's use of `bwrap` with `--bind`, `--ro-bind`, and proxy environment variables matches Harnez's existing `quota1SandboxArgs`. Exposing this cleanly as `harnez wrap [--net=none|proxy] -- <command>` gives developers a native way to run untrusted third-party CLIs safely on the host without full container overhead.
3. **In-CLI Secret Masking (`secret-create`)**:
   Devin's `drs secret-create` ingests credentials via interactive prompts or direct stdin pipes, ensuring secrets never appear in `argv`, shell history, or process listings (`ps`). Harnez can adopt similar ingestion mechanics for API tokens and sensitive configs.

---

## 5. Devin Fusion Deep Dive: Architecture, Guardrails, and Protocol

Based on the static inspection of `chisel-agent/src/local_fusion/` in the `devin` binary (issue [#1063](file:///home/uwe/projects/harnez/issues/1063-study-devin-cli-features-binary-capabilities-and-multi-agent-workflows-vs-harnez.md)):

### 5.1 In-Process Dispatch Mechanics
* **Tool Interface**: The Lead dispatches work through an internal tool named `sidekick` with two arguments: `message` (mandatory brief) and `block` (default `true`).
* **Session Persistence**: There is exactly one persistent Sidekick per session running in the same process against the shared workspace filesystem. Its state is tracked in SQLite via `V17__subagent_heads(session_id, agent_id, chain_node_id, updated_at)`.
* **Model Routing**: The Lead acts as a model router. The backend returns the paired model via `/exa.api_server_pb.ApiServerService/AssignModel` (Connect RPC).
* **Execution Flow & Interrupts**:
  * If the Sidekick is running and the Lead issues a new call, it is delivered as an interrupt (`<lead_update>`) rather than spawning a duplicate worker.
  * If a blocking wait expires, the Sidekick transitions into background execution; upon completion, it pushes a `<subagent_completion_notification>`.

### 5.2 Checks and Validations Between Agents
* **Diff Review as Landing Point**: The Lead's system prompt dictates: *"A handoff report about code it wrote or committed is itself a landing point: read the full diff and give it a verdict before your next action."* The report prose is treated strictly as an unverified claim until grounded in artifacts.
* **Prohibition of Test Alteration**: The Sidekick prompt explicitly forbids editing test assertions to make failing suites pass.
* **Explicit Declaration of Unverified Work**: If a test run cannot be executed, the Sidekick must explicitly state: *"implementation done, cannot verify because X"*.
* **Code Attribution Checks**: Edits matching non-permissive public code trigger a hard rollback (`"Attribution check failed; reverting edit"`).
* **No Code-Enforced Diff Gate**: No AST parser, test status validator, or compiler gate was found in the Rust binary between the Sidekick and Lead; the quality gate is enforced entirely via prompt guidance and the Lead's review turn.

### 5.3 Guardrails and Isolation Boundaries
* **Authority Asymmetry**: The Sidekick is banned from communicating with the user, creating Git commits, pushing to remotes, opening PRs, or running destructive Git commands (`reset --hard`, `clean -fd`, `checkout --`, `stash drop`, `sudo git`). The workspace is left dirty for the Lead to inspect and commit.
* **Code-Enforced Background Permissions**: When running in the background, any tool that would prompt the user for approval is automatically denied by the runtime (`permission_mask`).
* **Runaway Command Termination**: A `memory_guard` monitors cgroup limits (`DEVIN_MEMORY_GUARD_PERCENT`) and terminates the process group of any command exceeding safety thresholds.
* **Compaction and Cache Keep-Alive**:
  * `agent-ext/src/compactor/handoff_history.rs` preserves Lead briefs verbatim across conversation compaction (`HandoffHistoryMetadata`).
  * While the Sidekick is executing, the Lead's prompt cache is kept warm via an active keepalive (`[CACHE_KEEPALIVE] Lead idle but subagent mid-run; leaving keepalive alone`).
* **Runtime Reminder Injections**: Code dynamically injects system prompts when rules are violated (e.g. `<system_guidance> You made a direct edit yourself instead of delegating to Sidekick...` when the Lead edits files directly).

### 5.4 Data Storage & Protocol
* **Message Forest**: Conversations are modeled as immutable directed acyclic graphs in SQLite (`message_nodes`). Branches and subagent chains represent separate paths within this forest.
* **Wire Protocol**: No JSON-RPC or network protocol is used between Lead and Sidekick; communication is entirely in-process using Rust async channels and tool invocation / result messages. External protocols (ACP for IDEs, Connect RPC Protobuf for the Cognition API) are used exclusively for external boundaries.

---

## 6. Actionable Adoption Proposals for Harnez Multi-Agent Flows

### Priority 1: Text & Prompt Enhancements (Zero Code Changes)
1. **Report as a Mandatory Landing Point**:
   Update `AgenticLoop.md` and role specifications to treat a worker subagent's summary strictly as an unverified claim. The orchestrator/host must inspect the raw `git diff` and execution logs and render an explicit verdict before proceeding.
2. **Standardized Verification Schema**:
   Standardize the `developer` role handoff to require:
   - What was modified (file paths and line ranges).
   - Verbatim verification commands run and their exact exit codes.
   - Explicit declaration: *"done, but unverified because <reason>"* when tests cannot be executed.
3. **Carry Settled State in Briefs**:
   Include settled architectural decisions, running server ports, and completed checks in worker briefs to prevent redundant re-discovery.

### Priority 2: Runtime Code Guardrails
4. **Dynamic Post-Turn Reminders**:
   Implement an event-driven reminder hook in `harnez exec` or `harnez agent`: when a worker finishes with uncommitted working tree modifications, inject an orchestrator reminder to inspect the diff before dispatching subsequent tasks.
5. **Delegation Ineligibility List**:
   Define strict boundaries in `spec/agent.yaml` preventing the delegation of correctness-critical components (such as benchmark graders, verification thresholds, and production database queries) to non-frontier models where plausible but erroneous output cannot be caught by compilers.

### Priority 3: Empirical Experiments
6. **Persistent Worker vs Fresh Worker Evaluation**:
   Compare token costs and regression rates using `/jam`: evaluate whether a persistent developer subagent (retaining session context across milestones) outperforms fresh-handoff developers (`factory-sprint`) across multi-step tickets.

