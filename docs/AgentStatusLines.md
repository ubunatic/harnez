# Claude Code and Antigravity CLI Status Lines

Claude Code and Antigravity CLI run a configured `statusLine` command with a
JSON payload on stdin. The payload includes session, context, workspace, and
tool-specific state. Codex uses its native TUI status items in `~/.codex/config.toml`
instead of a custom command. See the [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
and the [Claude Code reference](https://code.claude.com/docs/en/statusline#available-data)
and [Antigravity CLI reference](https://www.antigravity.google/docs/cli/statusline/)
for their current schemas; fields can be absent or null depending on tool
version, account, and session state.

## Useful Payload Fields

### Antigravity CLI

The AGY status-line payload can include:

- **Context**: `context_window.total_input_tokens`,
  `total_output_tokens`, `context_window_size`, `used_percentage`,
  `remaining_percentage`, and `current_usage` token counts, including cache
  reads and writes. `exceeds_200k_tokens` indicates whether a fixed 200k
  threshold was exceeded.
- **Model and session**: `model.id`, `model.display_name`,
  `conversation_id` (`session_id` is a compatibility alias), `transcript_path`,
  `version`, and `product`.
- **Quota**: `quota` maps model/bucket IDs to `remaining_fraction`, reset time,
  and seconds until reset.
- **Agent and interaction state**: `agent_state`, `execution_mode`,
  `pending_input_count`, `tool_confirmation_pending`, and `task_count`.
- **Workspace and source control**: `cwd`, `workspace.current_dir`,
  `workspace.project_dir`, and `vcs` details such as type, branch, client, and
  dirty state; `sandbox` reports sandbox settings.
- **Other indicators**: `artifact_count`, `terminal_width`, `plan_tier`,
  `email`, and `vim.mode` when Vim mode is enabled.

Some fields are optional or session-dependent. In particular, current context
usage may be unavailable before the first API call.

### Claude Code

- **Context**: `context_window.total_input_tokens`,
  `context_window.total_output_tokens`, `context_window.context_window_size`,
  `context_window.used_percentage`, `context_window.remaining_percentage`, and
  `context_window.current_usage` (input, output, cache-write, and cache-read
  token counts). `current_usage` is null before the first API response and
  immediately after compaction.
- **Prompt cache**: `prompt_cache.hit_ratio`, `warm`, `ttl`, `requests`, and
  miss statistics. Available after the first API response on Claude Code
  v2.1.251 or later.
- **Model and session**: `model.display_name`, `effort.level`, `session_name`,
  `session_id`, `agent.name`, `fast_mode`, `thinking.enabled`, and
  `output_style.name`.
- **Cost and activity**: `cost.total_cost_usd`, session and API duration, and
  lines added or removed.
- **Limits**: `rate_limits.five_hour`, `seven_day`, and sometimes
  `spend_limit`, each with utilization and reset time. Availability depends on
  account or gateway.
- **Workspace and review**: current/project directories, added directories,
  repository identity, git worktree, open PR or merge request and review state,
  plus active worktree details.
- **Interface and traceability**: `vim.mode`, `version`, `prompt_id`, and
  `transcript_path`.

## Harnez Status-Line Output

The output layouts are Go templates in `spec/statusline.yaml`, validated by
`spec/schemas/statusline.schema.json` and embedded in the Harnez binary. Both
templates receive `.Context.Available`, `.Context.TotalTokens`, `.Context.Size`,
`.Context.WindowAvailable`, `.Context.WindowSize`, `.Context.CacheRead`,
`.Context.CachePercent`, and `.Context.HasCache`. Claude also receives
`.Directory` and `.RateLimits`, whose
entries have `.Label`, `.RemainingPercent`, and `.Remaining`. AGY also receives
`.Directory` and `.Quotas`, whose entries have `.Name`, `.RemainingPercent`,
and `.RemainingFraction`.
Context is unavailable when `.Context.Available` is false; absent rate/quota
data produces empty collections. Edit the spec and rebuild Harnez to change
the layouts.

AGY quota entries are filtered by the active `model.id`: Gemini models show
`gemini-*` quota buckets, while Claude and GPT models show `3p-*` buckets. If
the payload has no recognized active model ID, quota entries are omitted.

For Claude Code and AGY, Harnez renders the effective working directory
(showing `project → current directory` when they differ), followed by current
context size in rounded thousands, such as `156k`. AGY also shows the context
window capacity from `context_window.context_window_size`, for example
`156k/200k`. Its custom output appears below the built-in status indicators.
When cache-read token counts are
present, either renderer shows their share of
current input context, for example `156k (42% cached)`. That percentage is calculated as
`cache_read_input_tokens / total_input_tokens * 100`; it is a current-context
share, distinct from Claude's session-wide `prompt_cache.hit_ratio`.

Claude's line also appends the remaining percentage for each available rate
limit, for example `5h 77% left`. AGY's line appends
the active model class's remaining percentage, for example `weekly 94% left`.
The active class prefix (`gemini-` or `3p-`) is omitted. Missing windows and
quota buckets are omitted.

AGY's custom line uses ANSI dim styling (`SGR 2`, reset with `SGR 0`);
Antigravity CLI supports ANSI styling in status-line output.

The Claude renderer omits context details when current usage is unavailable.
Both agents invoke Harnez with their own status JSON on stdin each time the
status line refreshes: Claude runs `harnez statusline`, while Antigravity CLI
runs `harnez statusline --agent agy`. Harnez renders the payload directly; it
does not poll either agent or make a separate data request.

Use `harnez apply --components usage` to install or update both global status
line settings and Codex's native footer items, and
`harnez status --components usage` to check them. Codex receives its default
model, directory, and thread items plus context-remaining, model-context,
five-hour-limit, and weekly-limit items. These are Codex's built-in displays, not a Harnez custom
renderer. The AGY setting enables `stack_with_default` so Antigravity's
built-in indicators stay visible. `harnez revert --managed` removes
Harnez-managed status lines and Codex status items. The
`harnez agy-statusline` command remains as a compatibility alias. Harnez does
not yet render the other fields listed above.
