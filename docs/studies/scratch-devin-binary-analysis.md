# Devin CLI binary analysis (scratch, issue 1063 M2)

Input for `docs/studies/DevinCLIStudy.md`. This is a static analysis only: the binary was never
executed. The tools used were `strings`, `readelf`, `objdump`, `grep` and two small read-only scan
scripts, plus the bundled man pages and `.mdx` docs.

Each claim carries its evidence level:

- **[man]**: bundled man page. **[docs]**: bundled `.mdx`.
- **[str]**: string in the binary. Rust strings are not NUL-terminated, so neighbouring strings run
  together in the dump and some names below had to be split by hand.
- **[asm]**: decoded from `.text`. LLVM inlines strings of 16 bytes or fewer as immediates, so most
  `bwrap` flags are invisible to `strings`.
- **[inferred]**: my reading of the evidence, not directly proven.

## 1. Artifact

| Property | Value |
| --- | --- |
| Version | `3000.11.234`, commit `92c83011d113`, distribution `curl-bash` [man, file] |
| Format | ELF x86-64, static-pie, stripped, 217,576,808 bytes |
| SHA-256 | `9be67fc1ec19bc29e95f7329b21dc3b678ddc595bc943fd11ee05d0cd94d2d82` |
| Build ID | `7cb34e85f2e7daadda68dae3804a946c97ce1e97` |
| Toolchain | `rustc 1.99.0 (b940084d7 2026-09-28)`, plus GCC for C dependencies |
| Bundle | `bin/devin` (identical to the top-level copy), 42 doc files, 191 man pages generated from clap |

Internal product name: **chisel** (`chisel/src/bin/devin/main.rs`). Workspace crates seen in panic
and tracing paths [str]: `chisel`, `chisel-agent`, `chisel-core`, `chisel-api`, `chisel-server`,
`chisel-ui`, `chisel-otel`, `chisel-mcp`, `chisel-commands`, `chisel-cloud-bridge`,
`chisel-acp-relay`, `chisel-acp-client`, `chisel-proxy-config`, `toolbox`, `toolbox-core`,
`agent-ext`, `local-agent`, `local-tools-client`, `sandbox-runtime`, `plugin`, `plugin-host`,
`config-importers`, `user-config`, `windsurf-api-client`, `connect-rpc`, `inference`,
`message-forest`, `browser-preview`, `affogato`, `barista`, `scrollback`.

Key third-party crates (297 registry paths, from the private registry `pkg.cognition.build`) [str]:
`agent-client-protocol-schema-1.1.0`, `tokio`, `axum`, `reqwest`, `tokio-tungstenite`, `rusqlite`,
`rustls`, `tree-sitter`, `oauth2`, `opentelemetry`, `sentry`, `schemars`, `rama-socks5`, `wezterm-*`.

The Windsurf/Codeium heritage is still visible: `exa.*_pb` RPC services, `server.codeium.com`,
`WINDSURF_API_KEY`, `.codeiumignore`, and migration from `~/.codeium/`.

## 2. Subcommand tree [man]

Top-level flags: `--prompt-file`, `--config`, `--permission-mode` (default `auto`), `--sandbox`,
`--cloud`, `--model`, `--verbosity 0-3`, `-p/--print`, `--export [PATH]`, `-c/--continue`,
`-r/--resume [ID]`, `--respect-workspace-trust`. Positionals are `PATH` (opens Desktop) and `PROMPT`
(after `--`). Env overrides: `DEVIN_PERMISSION_MODE`, `DEVIN_SANDBOX`, `DEVIN_MODEL`,
`DEVIN_VERBOSITY`, `DEVIN_REFUSAL_FALLBACK`.

```text
devin
├── auth         login [--force-manual-token-flow] | logout | status
├── mcp          add | list | get | remove | login | logout | enable | disable
│                add: -t stdio|sse|http, -s local|project|user, --url, --command, -e, -H,
│                     --scopes, --oauth-client-id/-secret, --oauth-resource
├── models       list [--format text|json]
├── doctor       [--json]
├── rules        list [--provider cursor|windsurf] | show <NAME> | paths
├── skills       list [--trigger user|model] [--json] | show <NAME> | paths
├── plugins      install <SRC> [-y] [--local] | list | info | update [NAME]
│                | remove [--force] [--local] [-y] | prune
├── cloud        (no subcommand: REPL on cloud sessions, same as --cloud)
│   └── drs      whoami | sandbox-create --repo [--prompt] [--secret K=V]
│                | run --devin-id --command [--timeout 600]
│                | blueprint-list | blueprint-create [--repo] [--from-file]
│                | blueprint-write --blueprint-id --from-file
│                | build | build-start | build-wait --build-job-id | build-logs --build-job-id
│                | secret-create [--key] [--from-env|--from-file|--from-dotenv|--stdin]
│                                [--only] [--exclude] [--dry-run]
├── desktop      [ARGS] (forwarded to the launcher, channel-matched)
├── list         [--format interactive|json|csv]
├── rm           <TARGET> [--force]
├── ssh          [SESSION] [--gateway] [SSH_ARGS]
├── forward      <SESSION> <PORTS> [--gateway]   (port or local:port; runs until Ctrl-C)
├── update       [--force]
├── version
├── migrate      hooks | workflows [--scope all|workspace|global]
├── sandbox      setup   (only prints prerequisites)
├── setup        [--force-manual-token-flow]
├── uninstall    [--clean] [--force]
└── acp          [--agent-type summarizer|review] [--model] [--refusal-fallback] [--cloud]
```

Notes:

- DRS means "Declarative Repo Setup". Blueprints are `environment.yaml` files that a snapshot-setup
  build turns into a cloud environment. `build-logs` returns NDJSON.
- `secret-create` reads values inside the CLI, so they never reach argv, shell history or a
  transcript. This is an agent-safety pattern worth comparing with harnez.
- `plugins install` accepts `owner/repo`, a git URL or a local path, with `#subpath`. A repo with no
  manifest but with `skills/<name>/SKILL.md` installs as a skills-only plugin.
- `--refusal-fallback` switches model when a provider refuses a request under its usage policy, and
  tries each candidate at most once per session.
- Not in the man pages [str]: a remote worker with `~/.devin/worker/cache`, `DEVIN_WORKER_TOKEN`,
  `DEVIN_OUTPOST_GATEWAY_URL`, `DEVIN_OUTPOSTS_TOKEN` and `devin-remote` binaries at
  `static.devin.ai/devin-rs/remote` ("Outposts"). The man tree may omit other hidden commands.

## 3. Sandboxing

### 3.1 Scope and policy

- Only the **exec tool** is sandboxed. The system prompt says so: "Your other tools (read, edit,
  write, etc.) are not affected by the sandbox." Those tools go through the permission system [str].
- Writes are limited to the workspace plus granted `Write(...)` scopes. Everything is readable
  except `Deny(Read(...))` paths, which stay hidden for the whole session [man, docs].
- A `request_scope` tool lets the model ask for more access. A mid-session `Write` grant applies to
  later commands, and deny rules cannot be lifted [str, docs].
- Failure is closed: if resolution fails the CLI refuses to start. Windows is unsupported except
  through WSL 2 [docs].
- Team settings can force the sandbox on ("Required") [docs].
- `sandbox.excluded.{allow,ask,deny}` holds `Exec(...)` rules for commands that run outside the
  sandbox. The most specific rule wins within a source, and `deny > ask > allow` across sources.
  Unparseable commands stay inside, and persistent PTY shells never get excluded [docs].

### 3.2 Linux implementation

The code lives in a crate named `sandbox-runtime` (`src/{linux,manager,http_proxy,socks_proxy}.rs`)
[str]. It appears to be a Rust port or derivative of Anthropic's open-source `sandbox-runtime`
[inferred]. The evidence is leftovers such as `CLAUDE_CODE_HOST_HTTP_PROXY_PORT`,
`CLAUDE_CODE_HOST_SOCKS_PROXY_PORT`, `CLAUDE_TMPDIR`, `claude-empty-`, `/.claude/debug`,
`/private/tmp/claude` and the `vendor/seccomp/{x64,arm64}/apply-seccomp` layout. Any match with the
upstream project is from my memory and was not checked here.

`bwrap` arguments recovered from the code [asm, str]. The order is by code address, so it is not
proven to be runtime order:

```text
bwrap --new-session --die-with-parent
      --unshare-net                  # guarded by a flag test at 0x6cd06c4
      --bind <uds-sock> <uds-sock>   # host proxy bridge sockets [inferred]
      --setenv <K> <V> ...
      --ro-bind / /                  # base: whole filesystem read-only
      --bind <dir> <dir>             # workspace and each Write scope
      --ro-bind /dev/null <file>     # hide a denied file [inferred]
      --tmpfs <dir>                  # hide a denied directory [inferred]
      --dev /dev
      --unshare-pid
      --proc /proc
      -- <shell> -c '<proxy bootstrap>; eval <command>'
```

- `--unshar` occurs exactly twice in the binary (`--unshare-net`, `--unshare-pid`). There is no
  `--unshare-all`, `--unshare-ipc`, `--unshare-uts`, `--cap-drop` or `--share-net` [asm].
- The builder is at `0x6cd0540..0x6cd2f50`. The filesystem mounts are in `0x5fb635..0x5fe00c`.
- Protected paths are re-mounted read-only even inside the workspace [str]: `.git/hooks`,
  `.git/config`, `.gitconfig`, `.gitmodules`, `.bashrc`, `.bash_profile`, `.zshrc`, `.zprofile`,
  `.profile`, `.mcp.json`, `.ripgreprc`, `.claude/commands`, `.vscode`, `.idea`. Bare repos are
  handled via `/hooks` and `/config`. This list closes the "write a hook or rc file, escape on next
  use" route [inferred].
- `/etc/ssh/ssh_config.d` and `/.npm/_logs` also get special handling [str]. The exact treatment was
  not traced.
- Rules with globs are not enforced at OS level: "permission rule(s) use glob patterns, which the
  Linux sandbox cannot apply to sandboxed commands (it mounts concrete paths only)" [str].
- `bwrap` needs mount-point stubs on the host for missing paths. The runtime tracks them with
  lockfiles under `chisel/sandbox-locks`, using a `.coord` flock, `.staging-`/`.lockfile-` files,
  bwrap child PIDs and dead-peer scans. It runs an flock probe and **refuses to enable the sandbox**
  when flock is not enforced, for example on NFS without lockd [str].
- `SANDBOX_RUNTIME=1` and a private `TMPDIR` are set inside the sandbox [str].

### 3.3 Network path

With `--unshare-net` the child has no route out, so traffic goes through a bridge [str]:

```text
child → socat TCP-LISTEN:3128 → UNIX socket → host socat UNIX-LISTEN → host HTTP proxy
child → socat TCP-LISTEN:1080 → UNIX socket → host socat UNIX-LISTEN → host SOCKS5 proxy
```

- Bootstrap waits for both ports, up to 300 tries with a 10 ms sleep (about 3 s), and kills the
  bridges through `trap ... EXIT`. Missing or dead bridge sockets are errors.
- Proxy env set in the child: `HTTP(S)_PROXY`, `http(s)_proxy`, `ALL_PROXY`/`all_proxy`
  (`socks5h://`), `FTP_PROXY`, `GRPC_PROXY`, `RSYNC_PROXY`, `DOCKER_HTTP(S)_PROXY`,
  `CLOUDSDK_PROXY_*`, and `GIT_SSH_COMMAND` with a `socat PROXY:` ProxyCommand.
- `NO_PROXY` covers loopback, `*.local`, `169.254.0.0/16` and the RFC 1918 ranges. A sandboxed
  process can therefore address LAN hosts by intent, though `--unshare-net` still gives it no route
  [inferred].
- Config: `sandbox.allowed_domains`, `denied_domains`, `network_mode` (`full`, or `limited` for
  GET/HEAD/OPTIONS only). Patterns: `a.com`, `*.a.com`, `**.a.com` [docs]. An admin allowlist
  replaces the user's list, and denylists merge [docs].
- Unlisted domains trigger a user prompt instead of a silent block, and "allow permanently" saves to
  config. The docs still mark network filtering as unstable [str, docs].

### 3.4 Seccomp: claimed, but not found in this bundle

- The man page says `bwrap+seccomp`. The runtime expects external files `apply-seccomp` and
  `unix-block.bpf` under `vendor/seccomp/{x64,arm64}/` [str, asm: path built at `0x6cec597`,
  `0x6cec76b`].
- The extracted tree has no such files. The binary has no ELF magic other than its own header and no
  zstd or gzip frames, so the helper is not embedded in those forms. xz and brotli were not checked.
- Two strings point at different outcomes: "apply-seccomp binary not found..." (error) and
  "seccomp not available - unix socket access not restricted" (warning). The docs list only `bwrap`
  and `socat` as Linux prerequisites.
- **Likely result [inferred, not tested]:** on this install, seccomp does not run and AF_UNIX sockets
  are not blocked inside the sandbox. The filter name suggests it blocks only unix sockets, not a
  general syscall policy. Confirming this needs a run of `devin --sandbox` under `strace` in
  `Containerfile.sandbox`. That run needs a Devin login and network access and is out of scope here.

### 3.5 macOS seatbelt

This could not be analysed. The Linux binary has no `sandbox-exec` string and no SBPL profile text
such as `(version 1)` or `deny default`. The code is gated per platform, and `seatbelt` appears only
in help text and a word list. Only the docs' claim remains: it "works out of the box". A macOS build
is required to go further.

## 4. ACP (Agent Client Protocol)

### 4.1 Architecture

- Schema crate: `agent-client-protocol-schema-1.1.0`. JSON-RPC over newline-delimited stdio.
- **The REPL is itself an ACP client.** It spawns a `devin acp` child that owns the agent and the
  sessions database [man, str: `chisel/src/acp_repl/*`, "failed to start the ACP agent that owns the
  sessions database"]. So `devin`, `devin -p` and IDE hosts all share one agent code path.
- `devin acp --cloud` relays stdio to the cloud WebSocket (`chisel-acp-relay`: `connection`,
  `jsonrpc`, `sessions`, `state`). The URL path is `/acp/live?token=...`, so the token is in the
  query string [str]. Schemes `ws`/`wss`, with a hand-checked Upgrade handshake.
- `chisel-cloud-bridge` does handoff to cloud: it gathers a git diff (`gather_git_diff`, using a
  temporary `GIT_INDEX_FILE` and `add -N`), creates a session, summarizes history, then streams.
- Cloud API paths include `/v3/organizations/...` [str].
- No flag in the man pages lets the local CLI **serve** WebSocket. `chisel-server` and `axum` exist
  in the binary, and their role was not traced.
- Debug: `CHISEL_ACP_WIRE_LOG`, `CHISEL_PURE_ACP_WIRE_LOG`, `CHISEL_PURE_ACP_STDERR` [str].

### 4.2 Standard methods

Presence is by string match [str]. Direction is taken from the ACP model [inferred].

| Direction | Methods |
| --- | --- |
| Client → agent | `initialize`, `authenticate`, `logout`, `session/new`, `session/load`, `session/list`, `session/resume`, `session/prompt`, `session/cancel` (notification), `session/set_mode`, `session/set_config_option`, `session/delete`, `session/close` |
| Agent → client | `session/update` (notification), `session/request_permission`, `fs/read_text_file`, `fs/write_text_file`, `terminal/create`, `terminal/output`, `terminal/wait_for_exit`, `terminal/kill`, `terminal/release` |

Not found: `session/fork`, `session/set_model`. Model selection goes through config options
(`model`, `model_config`, `thought_level`).

Schema types [str]: `InitializeResponse{agentCapabilities, authMethods, agentInfo}`;
`SessionInfo{sessionId, cwd, additionalDirectories, title, updatedAt, _meta}`; `SessionModeState`;
`SessionConfigOption` with `select` kind and option groups; `Usage{totalTokens, inputTokens,
outputTokens, thoughtTokens, cachedWriteTokens, cachedReadTokens}`; capability structs for session
list, delete, additional directories, resume, close and logout, plus elicitation.

`session/update` kinds: `user_message_chunk`, `agent_message_chunk`, `agent_thought_chunk`,
`tool_call`, `tool_call_update`, `available_commands_update`, `current_mode_update`,
`config_option_update`, `session_info_update`, `usage_update`.

### 4.3 Extensions (`cognition.ai/*`)

Handler log lines name the methods; the namespace comes from strings such as
`cognition.ai/hooks/list`, `cognition.ai/plans/list` and `cognition.ai/reads/promptHistory` [str].
Not every wire name was confirmed individually.

| Area | Methods |
| --- | --- |
| Session | `session/end`, `session/share`, `session/rename`, `session/defaultConfigOptions`, `command/revise` |
| MCP | `mcp/listServers`, `listTools`, `installServer`, `removeServer`, `connectServer`, `disconnectServer`, `toggleServer`, `toggleTool` |
| Customization | `plugins/{list,assets,install,remove,refresh}`, `rules/list`, `skills/list`, `hooks/list`, `hooks/migrate`, `hooks/migrate/list`, `workflows/{list,migrate}`, `subagents/list`, `plans/list` |
| Goals and config | `goal/get`, `goal/set`, `goal/clear`, `config/read`, `config/write` |
| Revert | `revert/execute`, `revert/resume`, `revert/forkFromStep`; events `revert/stepsUpdated`, `revert/historyRewound` |
| Editor context | `document/didOpen`, `didClose`, `didFocus`, `didBlur`, `didChangeDirty`, `document/buffer` |
| Reads and terminal | `reads/promptHistory`, `reads/recentModelUids`, `terminal/killBackgroundShell` |
| Others | `browserPreview/capture`, `browserPreview/opened`, `clipboard/write`, `secret/create`, `snapshot-setup/update-blueprint-contents`, `integrations/github/connect`, `mcp/authorizationPending`, `mcp/authorizationFinished`, plus prefixes `diagnostics/`, `local-tools/`, `subagent/`, `repos/` |

Metadata also travels in `_meta` under `cognition.ai/*` and `chisel/*` keys, for example
`cognition.ai/errorKind`, `retryable`, `retryAfterSeconds`, `chisel/client-message-id`,
`chisel/fusion_lead_model_uid`, `chisel/goal-continuation`, `chisel/tool_call_timing`,
`chisel/approved_by`. There are hundreds of `cognition.ai/*` fields, many belonging to the cloud
session event model (Slack, PRs, ACU billing, playbooks).

ACP errors carry `errorKind` values `unavailable`, `resource_exhausted`, `deadline_exceeded`,
`content_filter`, `unknown` [str].

### 4.4 Permission options in `session/request_permission`

[str] `allow_once`, session-level allow ("Yes, allow ... this session"), `allow_always_global`,
`allow_all_fetches`, `allow_server_session`, `allow_server_always`, `allow_sandbox_excluded`,
`switch_accept_edits`, `switch_bypass`, `reject_once`, and for plans `plan_normal`,
`plan_accept_edits`, `plan_bypass`.

## 5. Permission system

### 5.1 Modes

| Flag value | Docs name | Behaviour |
| --- | --- | --- |
| `auto` or `normal` | Normal | Auto-approves read-only tools in the current directory |
| `accept-edits` | Accept Edits | Also workspace edits |
| `smart` | Smart | Also whatever a fast model judges safe; rolling out gradually |
| `dangerous`, `yolo`, `bypass` | Bypass | Everything, except that org deny and ask rules still win |
| `autonomous` | Autonomous | Accept Edits plus any shell command, inside the OS sandbox |

Parser error [str]: "Valid options: normal (auto), accept-edits, ... dangerous (yolo, bypass),
autonomous (requires --sandbox)".

Findings the man page omits:

- `autonomous` is a fifth mode, absent from the man page's list.
- `--sandbox` **forces** this mode: "Warning: --sandbox always uses the autonomous permission mode;
  ignoring --permission-mode" [str]. So in a sandboxed session shell commands run without per-command
  prompts, and the sandbox is the boundary.
- Org policy can restrict modes and fall back: "Permission mode ... is restricted by your
  organization's policy. Falling back to normal" [str].
- Agent mode is separate: Normal, Plan, Ask [docs]. The database column was renamed
  `permission_mode` → `agent_mode` [str].

### 5.2 Rules

- Shape: `permissions.{allow,ask,deny}` [str].
- Rule kinds [str]: `Read(`, `Write(`, `Exec(`, `ExecExact(`, `Fetch(domain:`, `Fetch(`,
  `WriteToProcess(`, plus `mcp__` tool names. Skills can also grant tools through `allowed-tools`.
- Sources: user config, project config, imported tool config, and team settings, which cannot be
  overridden [str, docs].
- `toolbox/src/tools/readonly_path_extractor.rs` and `powershell_parser.rs` suggest that shell
  commands are parsed to derive what they touch [inferred].

### 5.3 Smart mode classifier

`agent-ext/src/smart_permission/{classifier,mod}.rs`. Prompt excerpts [str]:

- "You are a security classifier for a coding agent. Decide whether the single requested action
  below is safe to run automatically WITHOUT user approval."
- Reply format: `{"decision": "allow" | "deny" | "ask", "reason": "..."}`.
- Untrusted input is wrapped in nonce-tagged blocks such as `<project_rules-{nonce}>`, and the prompt
  says never to allow because that text asks for it. This is a prompt-injection defence.
- Abstain reasons: `no_model`, `uncertain`, `parse_error`, `inference_error`, `timeout`,
  `interrupted`. Decisions are recorded in analytics as `SMART_DECISION` with
  `classifier_request_id`; `PERMISSION_PROMPT_RESOLVED` is a sibling event.

## 6. Configuration and state

### 6.1 `~/.config/devin/config.json`

Top-level keys [str, recovered from serde field lists]:

```text
version  agent  shell  theme_mode  theme_auto_detect  hooks  mcpServers  permissions
pty_for_noninteractive_exec  read_config_from  metrics_endpoint  skip_home_directory_warning
show_path  include_gitignored_files  respect_gitignore  attribution  subagents_enabled
disabled_tools  unicode_mode  legacy_terminal  disable_osc  skip_workspace_trust  notify
sandbox  proxy  devin  otel  mouse_capture  local_tools_bypass_permissions  show_hints
typewriter  auto_update  session_store  keymap
```

| Section | Fields |
| --- | --- |
| `agent` | `model`, `subagent_model`, `preferred_family_models`, `show_history_on_continue`, `compaction_threshold_tokens`, `codex_tools`, `verbosity`; thinking display `stream` or `show_full_thinking` |
| `sandbox` | `allowed_domains`, `denied_domains`, `network_mode`, `excluded.{allow,ask,deny}` |
| `otel` | `enabled`, `endpoint`, `logs_endpoint`, `metrics_endpoint`, `headers`, `logs_headers`, `metrics_headers`, `resource_attributes`, `timeout_ms`, `log_export_interval_ms`, `metric_export_interval_ms`, `disable_user_prompt_logging`, `disable_tool_logging`, `no_proxy`, `blocking` |
| `devin` | includes `org_id` |
| `session_store` | `sqlite` or `files` (env `DEVIN_SESSION_STORE`) |
| `read_config_from` | `cursor`, `claude`, `zed`, `copilot`, `agents_standard`, `cloud` |
| `mcpServers.*` | `command`, `args`, `env`, `cwd`, `url`, `serverUrl`, `transport`, `headers`, `oauthClientId`, `oauthClientSecret`, `oauthScopes`, `oauthResource`, `tokenScope`, `installationId`, `disabled`, `disabledTools` |

The file is versioned: "Wrote config schema version", and "Config file version ... is newer than the
supported version; leaving as-is". `schemars` is linked, so a JSON schema likely exists internally
[inferred]; none was found in the binary as text.

Project files: `.devin/config.json`, `config.local.json`, `.devin/mcp_config.json`,
`.devin/mcp_config.local.json`, `.devin/hooks.v1.json`, `.devin/skills/`, `.devin/plans/`. User-level
agents live in `~/.config/devin/agents/`, plans in `~/.devin/plans/` [str, docs].

**Interop with other tools.** Importers (`config-importers`) read `.claude/settings.json`,
`.claude/settings.local.json`, `.claude/mcp_servers.json`, `.claude/`, `.cursor/rules/`,
`.windsurf/{rules,hooks.json,skills}`, `.windsurfrules`, `.zed/settings.json`, `.agents/`,
`AGENTS.md`, `agents.md`, `AGENTS.local.md`, `.windsurfignore`, `.codeiumignore` [str].

Hooks are Claude-compatible:

- Events include `PreToolUse`, `PostToolUse`, `PreCompact`, `PostCompact`, `SessionStart`,
  `SessionEnd`, `PermissionRequest`, `Notification`, `SubagentStop`.
- Types are `command`, `prompt`, `powershell`.
- Fields are `timeout`, `working_directory`, `show_output`, `fail_closed`, `matcher`.
- Output includes `permissionDecision`, `permissionDecisionReason`, `updatedInput`.
- Plugins accept both `.devin-plugin/plugin.json` and `.claude-plugin/plugin.json`, and hooks see
  `CLAUDE_PROJECT_DIR` and `CLAUDE_PLUGIN_ROOT` [str].

### 6.2 Local state

| Item | Location | Evidence |
| --- | --- | --- |
| Logs | `~/.local/share/devin/cli/logs/devin_<timestamp>_<pid>.log`, `*.log.gz` | [docs] |
| Session DB | `~/.local/share/devin/cli/sessions.db` (override `DEVIN_SESSIONS_DIR`, `CHISEL_SESSION_DB`) | [str, path inferred] |
| Older DB name | `cli_sessions.db`, imported | [str] |
| Summaries | `$XDG_DATA_HOME/devin/summaries/<session_id>.md` | [man] |
| File store | `log.jsonl`, marker `.prompt-history-imported` | [str, path unknown] |
| Credentials | `credentials.toml` | [str, path unknown] |
| Team cache | `team_settings.bin` | [str, path unknown] |
| Workspace trust | `trusted_workspaces.json` | [str, path unknown] |
| Skill analytics spool | `skill_events_spool.jsonl` with a lock | [str, path unknown] |
| Worker cache | `~/.devin/worker/cache` (`DEVIN_WORKER_CACHE_DIR`) | [str] |
| Install | `~/.local/bin/devin`, `_versions/<ver>/`, `_versions/current` symlink | [docs, tree] |

`sessions.db` is SQLite with refinery migrations. Tables [str]: `sessions` (`id`,
`working_directory`, `backend_type`, `model`, `agent_mode`, `created_at`, `last_activity_at`,
`total_tokens`, `current_epoch`, `hidden`, `metadata`, `main_chain_id`), `messages`, `message_nodes`
(immutable nodes forming a forest, with JSON `chat_message`), `subagent_heads` (persistent subagent
chains, for example the Local Fusion sidekick), `tool_call_state` (serialized ACP tool call JSON for
replay), `live_row_times`, `prompt_history`, `rendered_commits`, `app_state`, and `message_search`, a
contentless FTS5 index maintained by triggers.

### 6.3 Other environment variables and network endpoints [str]

- Harness: `DEVIN_HARNESS`, `DEVIN_HARNESS_LEAD_ONLY`, `DEVIN_HARNESS_SIDEKICK_ONLY`,
  `DEVIN_PREFER_EXEC_TOOL`, `DEVIN_SIDEKICK_PREFER_EXEC_TOOL`, `DEVIN_COMPACTION_THRESHOLDS`,
  `DEVIN_SIDEKICK_COMPACTION_THRESHOLDS`, `DEVIN_MEMORY_GUARD_PERCENT`,
  `DEVIN_CONTINUE_ON_OUTPUT_TRUNCATION`, `DEVIN_TRACEPARENT`,
  `DEVIN_CLI_DANGEROUSLY_FOLLOW_SYMLINKS`.
- Endpoints: `api.devin.ai`, `app.devin.ai`, `server.codeium.com` (`WINDSURF_API_SERVER_URL`),
  `static.devin.ai/cli/current/manifest.json` (updates), enterprise and Windsurf variants of that
  manifest.
- RPC: Connect protocol to `/exa.api_server_pb.ApiServerService/*` (`GetChatMessage`,
  `GetCliModelConfigs`, `GetWebSearchResults`, `AssignModel`, `RecordTrajectorySegmentEvents`,
  `SubmitBugReport`, managed plugins), `/exa.seat_management_pb.SeatManagementService/*`
  (`GetUserStatus`, `GetCliTeamSettings`, PKCE exchange), `/exa.attribution_pb.*`.
- Telemetry and flags: `/exa.product_analytics_pb.*/BatchRecordAnalyticsEvents`, Sentry (two
  ingest hosts), `unleash.codeium.com` feature flags, optional user OTLP export. There is also a
  check of chat code against public code ("Attribution check").

### 6.4 Built-in tools [str: `toolbox/src/tools/`]

`read`, `edit`, `write`, `apply_patch`, `exec_tool` (with `pty`, `raw_exec`, shell sessions and
`shell_edits` tracking), `write_to_process`, `kill_shell`, `grep`, `glob`, `fast_context`,
`instant_context`, `notebook_read`, `notebook_edit`, `view_image`, `webfetch`, `web_search`, `todo`,
`user_question`, `request_scope`, `write_plan`, `exit_plan_mode`, `persistent_subagent`, and MCP
(`call_tool`, `list_tools`, `list_servers`, `prompts`, `read_resource`).

## 7. Points for the M3 comparison

- Sandbox as boundary: `--sandbox` replaces per-command approval with OS confinement. `request_scope`
  gives the model a defined way to ask for more, and the system prompt tells it not to work around
  restrictions.
- Protected-file re-mounts and the network bridge are concrete patterns for `harnez exec` and `wrap`.
- One agent path: the REPL as an ACP client of its own server, so TUI, headless and IDE share it.
- Secrets read inside the CLI so they never enter argv or transcripts.
- Claude hook, plugin and rule compatibility makes migration into Devin cheap.
- Smart mode's nonce-wrapped classifier prompt is a reusable approval pattern.

## 8. Not verified

1. Whether seccomp actually runs on this install (section 3.4). Needs a runtime test.
2. Runtime order of `bwrap` arguments, the meaning of the `--unshare-net` guard, and the
   `--ro-bind /dev/null` and `--tmpfs` purposes. These come from code layout only.
3. macOS seatbelt profiles (section 3.5). Needs a macOS build.
4. Wire names of each `cognition.ai/*` method, and the role of `chisel-server`.
5. Locations marked "path unknown" in section 6.2, which need a real install.
6. The man tree may not list every hidden command.
