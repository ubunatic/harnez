# Finder Plugin Design (2026-09-25)

**Status**: Proposed design
**Source**: [Search capabilities inventory](2026-09-25-search-capabilities.md)

## Goal

Add code and documentation search to `harnez find` through a small registry of external finder commands. Harnez coordinates commands and presents consistent results; search engines remain separate executables.

## User interface

- Add `code` and `docs` targets to `harnez find`.
- Each target runs all registered finders in its scope concurrently, then merges, deduplicates, and ranks results.
- `--via <name>` selects only the named finder; reject unknown names and finders outside the requested scope.
- Return results as JSON array or JSON Lines. Each record has `path`, `line`, `title`, `snippet`, `score`, and `kind`.
- Expose the same search behavior and result schema through one MCP search tool.
- Existing fuzzy issue search remains supported and becomes the first built-in finder; use it as the docs fallback.

## Finder registry and execution

Finder definitions live in `~/.harnez/config.yaml`; project configuration may override them. A finder declares:

- `name`: stable selector used by `--via`.
- `scope`: `code` or `docs`.
- `command`: argument-safe command template with a query placeholder.
- `timeout`: maximum execution duration.

Run matching finders in parallel with bounded timeouts. Parse each command's JSON array or JSON Lines records into the common result schema. A failing or timed-out finder should not discard successful results from other finders; report execution errors on stderr/diagnostics. Reject malformed records and never interpolate the query through a shell. Merge results, deduplicate by normalized path plus line, and rank deterministically using finder score with stable path/line tie-breaks. Preserve the source `kind` when supplied and identify the finder in diagnostics.

No harnez-owned index or cache is introduced in this design. Finders own their indexing, freshness, and ranking internals.

## Initial finder set

- **`neus`**: first external finder. Invocation contract: `neus search --json -k N <query>`. It returns records with `path`, `line`, `title`, `snippet`, and `score`; harnez supplies `kind` from the finder scope when absent.
- **`rg`**: code fallback when `neus` is unavailable. Adapt matching lines into the common result format; use a small snippet and deterministic score.
- **Built-in fuzzy issue search**: first built-in finder, preserving existing issue matching and ranking. It supplies docs fallback results by searching documentation content when configured for docs scope.

The adapter should keep the external command boundary generic so additional finders can be registered without Go plugin loading or engine-specific code in the CLI.

## Configuration and safety

Project overrides take precedence over user-level definitions for a matching finder name. Validate names, scopes, command templates, and positive timeouts while loading configuration. Execute commands without shell expansion, pass the query as one argument, cap captured output, and enforce timeouts. Preserve paths relative to the selected repository root and avoid exposing secrets from configuration in diagnostics.

## Open: neus requirements (pending reply from neus session)

- Repository scope filter (`--root`).
- Content-kind filter (`--kind`, code or docs).
- Configurable embedding endpoint.

Confirm these option names and behavior with the neus session before finalizing its adapter contract.

## Acceptance outline

- `find code` and `find docs` run applicable registered finders concurrently and produce stable merged results.
- JSON and JSON Lines output follow the shared schema; duplicate path/line pairs appear once.
- `--via` selects a single in-scope finder; errors and timeouts do not hide other finder results.
- The same search capabilities are available from one MCP tool.
- `neus` is preferred when available; missing neus falls back to `rg` for code and fuzzy search for docs.
- No harnez search index or cache is added.
