# harnez tui — Architecture & Decisions

Consult before changing `harnez tui` (`cmd/harnez/tui.go`, `internal/tui/`, `spec/tui.yaml`).
Epic: issue 774. Library: `ubunatic.com/loom`.

## Shape

- One loom `Pane`; `AppWidget` owns header, chat, prompt and overlays (pickers, job panel).
- Background work (turns, jobs, file index) never touches widgets; it calls `Pane.Post`.
- Keys, labels, limits and formats live in `spec/tui.yaml` + schema; Go holds no copies.

## Decisions

| Decision | Why | Issue |
|---|---|---|
| Library gaps are fixed in loom (`Pane.Post`, `Pane.Suspend`, kitty keyboard), not worked around in harnez | loom rule: fix the library, not the caller | loom 319–321 |
| Turns reuse `internal/subagent` drivers (`claude -p`, `codex exec`, …) instead of API clients | keeps Pro subscription quota; one code path with `harnez agent` | 776 |
| Streaming uses separate `StreamDriverFor`; `DriverFor` (agent start/resume) stays unchanged | no behaviour change for agent callers; pinned by `TestClaudeBatchDriverUnchanged` | 776 |
| agy stays batch | its CLI only returns one JSON object | 776 |
| Cancel kills the provider's whole process group | grandchildren held stdout open and hung the turn | 776 |
| Rules header says "once per session"; turns resume native sessions | user decision: read rules once, resume cheaply | 791 |
| A backend session restarts fresh only on a verified stale error (`spec/agent.yaml` `stale_session_errors`, matched on provider stderr) | a quota/auth/network error must never silently drop context | 791 |
| Backend session ids persist per tui session in `~/.harnez/tui/sessions/<id>.json` | `--session X` resumes after restart | 791 |
| Newline: Shift+Enter, Ctrl+J (kitty keyboard), Alt+Enter, trailing `\`; Enter sends | legacy terminals send Ctrl+J as Enter | 775 |
| Ctrl+G edits the prompt in `$VISUAL`/`$EDITOR` via `Pane.Suspend` on `/dev/tty` | piped stdin must not reach the editor | 777 |
| `@` file picker (git ls-files or walk), `/` command picker (built-ins + skills); Enter selects while open | Claude Code–style prompt ergonomics | 777 |
| `!cmd` runs a background job; Ctrl+B opens the job panel | simple start syntax; jobs never go to the model | 778 |
| Job output is coalesced (≤ 1 queued Post per job) | loom's Post queue is unbounded; Ctrl+G pauses the loop | 778 |
| Ctrl+O hands the terminal to the native CLI resuming the current session, not to a live PTY | turns run non-interactively, so no PTY exists | 779 |
| Chat and job logs keep only SGR; all other escapes/controls are dropped | model and tool output is untrusted (screen clear, OSC 52 clipboard) | 779 |
| Cancelled turns show a neutral note, not an error | cancel is user intent | 776 |
| Header leads with status and `[resumed]`/`[new session]` | 80-column terminals clip the right edge | 791 |
| Model picker opens on Ctrl+T or Alt+M (`open_models`) and on a bare `/model` | Ctrl+M is CR (= Enter); Ctrl+T has no readline action here and no other binding; Alt+M needs Meta-as-Escape, so it is the fallback; `TestKeybindingsDoNotConflict` checks all spec bindings | 776 |
| Model picker is the `pickModel` kind of the inline picker (same state, ranking, `picker_*` keys); its query is typed in the overlay | no third overlay framework | 776 |
| Tiers are list rows (`codex:luna:low/med/high`, from `KnownModelEntries`), grouped by provider; no tier toggle | filtering "luna high" reaches any tier; one widget path | 776 |
| A pick calls `setModel`, the path of `/model <spec>` | one validation and note | 776 |
| Switching during a running turn is allowed; the turn keeps its model, the note says the switch applies to the next turn | no reason to block; cancel stays explicit | 776 |
| Session marker `↻` means the provider model has a stored backend session; it shows on every tier | sessions are keyed by provider:model, a tier change resumes the same session | 776, 791 |

## Open

- agy live streaming (needs a streaming agy CLI mode).
- Ctrl+O into agy runs without the usage-metering supervisor.
