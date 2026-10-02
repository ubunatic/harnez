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
session and project. This describes a selectable local session record; it does
not claim that a Codex process is running. Harnez uses the rollout file's
modification time for `last_active_at`. Duplicate provider session IDs are
collapsed, and records already present in Harnez's registry are not repeated.

The list exposes metadata only. It does not parse conversation content or
provide attach, resume, or context inspection for external entries.

## Provider coverage

Codex rollout metadata is the only external format currently enabled. Harnez
does not infer Claude Code identities from local transcript files. Anthropic
documents resuming by a known session ID, but does not define a local listing or
discovery metadata contract in its [CLI reference](https://docs.anthropic.com/en/docs/claude-code/cli-usage).
Other providers remain unsupported until they offer a safe,
provider-supported local discovery mechanism.
