# Harnez Search Capabilities (2026-09-25)

**Scope**: inventory of user-facing search, lookup, indexing, and read surfaces present in this checkout. “Search” here means locating content or records; generated indexes and file readers are included where they are commonly used for discovery.

## Inventory

### `harnez find`

- **Command / targets**: `harnez find <entity> [options] <query...>` currently supports only the `issues` entity. It scans `issues/*.md` and `issues/archive/*.md`; excludes `issues/README.md` and all other paths. Sources: `cmd/harnez/find.go`, `internal/issues/issues.go`.
- **Subcommands / shorthand**:
  - `find issues [query...]`: fuzzy issue discovery; no query lists issues in ticket-number order.
  - `find issues last`: same unfiltered listing, default newest 10.
  - `find issues open|blocked|closed|draft [query...]`: status-filter shorthands. `in-progress` is available through `status:in-progress`/`is:in-progress`.
  - `find issues next` or `--next`: calculates max ticket number + 1. Read-only, no reservation.
  - `find issues history [--project NAME]`: reads issue-count snapshots stored by `harnez index`, oldest first; not text search.
- **Flags**: `-d/--dir` repository root; `--next`; `--json`; `-r/--raw` and `-t/--text` deterministic TSV; `-I/--image` PNG overview; `--project` for history; `-n/--limit` default 10; `-a/--all` bypasses result limiting. Unfiltered listing keeps the newest `limit`; filtered listing keeps the top-ranked `limit`. TTY output defaults to a visual card; pipes default to TSV. Source: `cmd/harnez/find.go`.
- **Matching / ranking**: AND across whitespace terms, OR alternatives within one `a|b` term, and `status:`/`is:` filters ANDed with text. Normalizes by Unicode lowercase, Markdown/punctuation removal, and whitespace collapse. A term matches as a substring or if each token matches a field token by prefix or bounded Damerau–Levenshtein typo distance (<=4 chars: no typos; 5–8: one edit; 9+: two edits). Searches stripped H1 title and body after metadata; metadata is excluded. Ranking favors title exact, title prefix, title fuzzy, then the corresponding body classes; ties use ticket number then path. No regex, parentheses, negation, quotes, or general Boolean grammar. Sources: `internal/find/query.go`, `internal/find/match.go`, `internal/find/search.go`.
- **Limits**: only issue files, no code/docs corpus; each query scans the current ticket set rather than consulting a text index. Query syntax intentionally stays small. `next` reports but does not claim a number.

### `harnez issues` lookup surfaces

- **`issues list [filter]` / `issues -l`**: thin wrapper over `find issues`; defaults to `is:open`. Shares fuzzy/filter semantics, `-n/--limit`, `-a/--all`, `-d/--dir`, `--json`, `-r/--raw`, `-t/--text`, `-I/--image`; also accepts bare `-N` limit shorthand. An explicit filter replaces the default open filter. Sources: `cmd/harnez/issues.go`, `cmd/harnez/find.go`, `internal/find/`.
- **`issues show <ticket-number>`**: exact issue lookup, accepting numeric forms with or without leading zeroes; duplicate numbers error. `findTicketFile` also resolves non-numeric path, filename, or slug forms to disambiguate. Displays full Markdown (or JSON/card); it does not fuzzy-search body/title. Scope is the same active and archived issue files. Sources: `cmd/harnez/issues.go`, `internal/issues/issues.go`, `internal/readcard/`.
- **`issues` flags relevant to lookup**: `-d/--dir`, `--json`, `-r/--raw`, `-t/--text`, `-I/--image`; `-n/--limit`, `-a/--all`, `-l/--list`. `issues lint` validates tracker consistency but is diagnostic, not search. Source: `cmd/harnez/issues.go`.

### `harnez read` and docs operations

- **`harnez read [flags] [files...]`**: reads named files or stdin; it is a reader, not a file/content locator. Text controls include `-L/--lines`, `--head`, `--tail`, `-n/--number`, `--line-numbers`, `--raw`, and `--text`; `-I/--image` and `--auto` route content to PNG cards or text. It accepts paths supplied by the caller and does not walk directories, match names, or search file contents. Sources: `cmd/harnez/read.go`, `internal/readcard/read.go`.
- **`harnez docs`**: no docs-search or docs-lookup command. `docs variant` swaps an installed document variant; `docs cards build/check` builds or validates visual doc cards. Sources: `cmd/harnez/docs.go`, `cmd/harnez/docs_cards.go`.
- **`harnez dochistory [files...]`** (aliases `doc-history`, `repo-history`) analyzes Git history for selected docs/files and category tracks; this is temporal analysis, not text lookup. Sources: `cmd/harnez/dochistory.go`, `internal/assess/`.
- **`harnez index [--check]`**: generates `issues/README.md` from issue metadata and the studies table in `docs/README.md` from `docs/studies/*.md`; records issue-count snapshots used by `find issues history`. `--check` reports drift without intended changes. It indexes table metadata only and is not a query engine or full-text index. Sources: `cmd/harnez/index.go`, `internal/index/index.go`, `internal/issues/issues.go`, `internal/telemetry/issuesnapshot.go`.

### Other record lookup

- **`harnez feedback list`** lists this project's unreviewed feedback log entries (`--all` includes promoted entries); `feedback promote <id>` locates an entry by exact ID. Neither provides fuzzy text search. Sources: `cmd/harnez/feedback.go`, `internal/feedback/`.
- **`harnez agent list [--dir]`** lists agent sessions, with an optional exact working-directory filter. It does not search prompts or session contents. Source: `cmd/harnez/agent.go`.

### Grep/find, MCP, and hooks

- Harnez has no built-in code grep, filename `find`, or content search command. Shell `grep`/`rg`/`find` are external tools, with their own literal/regex/glob behavior; harnez does not index code for them.
- The issue-tracker instructions recommend `harnez find ... issues` instead of raw grep for issue discovery. That is guidance, not a shell-command interception or grep rewrite. Source: `config.yaml` (embedded managed instruction section; mirrored in `AGENTS.md`).
- Harnez MCP exposes agent lifecycle tools (`harnez_spawn_agent`, `harnez_command`, wait/list/status/resume/stop); it exposes no search/find/grep/read tool. `harnez_list_agents` lists sessions and can filter by working directory only. Source: `internal/mcp/server.go`.
- Read hooks can deny unconstrained or large native file reads and redirect the caller to `harnez read --auto` or `harnez read -n -L ...`. They do not redirect grep/find requests and do not search code. Sources: `cmd/harnez/hook.go`, `internal/codex/hooks.go` (hook integration).

### Search and ranking implementation packages

- `internal/find/`: query parsing, normalization, fuzzy matching, status filters, and deterministic ranking. No persistent index; evaluates the provided issue-file slice.
- `internal/issues/`: scans/parses issue Markdown and tracker tables; supplies issue data and number allocation. Scan recognizes numbered Markdown ticket files under the active/archive directories; it is not a general repository crawler.
- `internal/index/`: renders generated issue and studies tables from source files; no search ranking or full-text index.
- `internal/readcard/`: renders issue summaries and file contents into cards; presentation only, not matching.

## Gaps

- **Finding code**: no first-party code/file-name search, symbol search, or indexed repository search; users depend on external `grep`/`rg`/`find` or host tools.
- **Finding docs**: no full-text or fuzzy search over docs, no docs entity for `harnez find`, and no docs catalog lookup beyond generated tables/cards and Git history analysis. `harnez read` requires a known path.
