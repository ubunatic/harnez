# External Agent Session Discovery

`harnez agent list` includes provider session metadata that is safe to discover
from local session files. Provider paths, file patterns, record formats, and
field mappings are defined in `spec/agent.yaml` and checked by its JSON Schema.
External sessions appear in global lists (`--all-sessions` or an unscoped
caller); host-scoped and `--children` views contain only sessions with Harnez
lineage metadata.

## Codex

Harnez scans `{home}/.codex/sessions/*/*/*/rollout-*.jsonl` and reads only the
first JSONL record. It accepts a `session_meta` record and extracts the session
ID, optional provider session ID, working directory, model provider, source, and
creation timestamp using the mappings in the spec. Codex's rollout reader
defines the session metadata record used in these local files:
[Codex rollout metadata reader](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/list.rs).

Discovered entries have status `available` when their metadata can identify the
session and project. In the default `agent list` view, Harnez matches rollout
records to live Codex CLI processes by process start time and working directory;
resumed sessions match by their session ID on the command line. The match window
is defined by `active_match_window_seconds` in `spec/agent.yaml`. This process
match is currently supported on Linux. `--dead` lists all discoverable Codex
rollouts, including historical sessions. Harnez uses the rollout file's
modification time for `last_active_at`. Duplicate provider session IDs are
collapsed, and records already present in Harnez's registry are not repeated.
The latest rollout `token_count` event provides current context use (the latest
turn's full input count, including cached input) and model context-window
capacity. These are the live window figures: context use can change each turn
or after `/clear`. The same event provides session-wide input, cached-input,
and output totals. `SESSION_USED` is calculated as total input minus cached
input, plus total output, matching Codex's `used-tokens` status-line metric.
For example, 38.4M input - 37.6M cached + 208K output = 965K session used.
Session input can greatly exceed window capacity because input is counted again
across turns. `SESSION_USED` is a cumulative usage estimate, not current context
size or a direct measurement of subscription quota. These rollout metrics are
read for active sessions; `--dead` discovery stays metadata-only to avoid
scanning full historical transcripts and may show `-` for usage fields. Hook
snapshots are not used for active list metrics because they can cover a
different period, for example after `/clear`.

The list exposes metadata only. It does not parse conversation content or
provide attach, resume, or context inspection for external entries.

## Provider coverage

Codex rollout metadata is the only external format currently enabled. Harnez
does not infer Claude Code identities from local transcript files. Anthropic
documents resuming by a known session ID, but does not define a local listing or
discovery metadata contract in its [CLI reference](https://docs.anthropic.com/en/docs/claude-code/cli-usage).
Other providers remain unsupported until they offer a safe,
provider-supported local discovery mechanism.
