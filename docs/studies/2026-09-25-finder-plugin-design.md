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

Run matching finders in parallel with bounded timeouts. Parse each command's JSON array or JSON Lines records into the common result schema. A failing or timed-out finder should not discard successful results from other finders; report execution errors on stderr/diagnostics. Reject malformed records and never interpolate the query through a shell. Merge results, deduplicate by normalized path plus line, and rank by each finder's result order (reciprocal-rank merge; raw scores are not comparable across finders) with stable path/line tie-breaks. Preserve the source `kind` when supplied and identify the finder in diagnostics.

No harnez-owned index or cache is introduced in this design. Finders own their indexing, freshness, and ranking internals.

## Initial finder set

- **`neus`**: first external finder. Invocation: see "neus finder contract" below (`--root`, `--kind`, `--timeout`, exit code 3 = not indexed).
- **`rg`**: code fallback when `neus` is unavailable. Adapt matching lines into the common result format; use a small snippet and deterministic score.
- **Built-in fuzzy issue search**: first built-in finder, preserving existing issue matching and ranking. It supplies docs fallback results by searching documentation content when configured for docs scope.

The adapter should keep the external command boundary generic so additional finders can be registered without Go plugin loading or engine-specific code in the CLI.

## Configuration and safety

Project overrides take precedence over user-level definitions for a matching finder name. Validate names, scopes, command templates, and positive timeouts while loading configuration. Execute commands without shell expansion, pass the query as one argument, cap captured output, and enforce timeouts. Preserve paths relative to the selected repository root and avoid exposing secrets from configuration in diagnostics.

## neus finder contract (from neus session, 2026-09-25; neus ticket 009 implements --root/--kind/--timeout)
- Call: `neus search --json --root <abs dir> [--root ...] --kind code|docs -k <n> --timeout <dur> "<query>"`. Query is one argv element (no shell). harnez passes its timeout via --timeout and also kills the process after it.
- Output: stdout = one JSON array of {path,line,title,snippet,score,kind}; path absolute. Score is higher-is-better but only comparable within one neus run (RRF), so harnez merges finders by rank, not raw score. Warnings only on stderr.
- Exit codes: 0 ok (array may be empty), 3 root not indexed, 4 backend/db error. On 3 harnez may run `neus index <root>` once (incremental, content-hash skip, ~4s keyword-only) or pass `--index-stale`; never index on every query.
- Embeddings: default endpoint http://127.0.0.1:8746/v1/embeddings, model nomic-embed-text-v1.5 (lmcoder ticket 132); override via NEUS_EMBED_URL/NEUS_EMBED_MODEL or --embed-url/--embed-model. Unreachable -> keyword-only with stderr warning, still exit 0.
- Index DB: ~/.cache/neus/index.db (--db). neus needs nothing else from harnez.
- Blocked on: neus ticket 009 landing (neus will notify).
- Embedding server (lmcoder 132, commit 6a15120) is built but NOT started; start via `lmcoder service start --embed` after user go. Until then neus is keyword-only.

## Acceptance outline

- `find code` and `find docs` run applicable registered finders concurrently and produce stable merged results.
- JSON and JSON Lines output follow the shared schema; duplicate path/line pairs appear once.
- `--via` selects a single in-scope finder; errors and timeouts do not hide other finder results.
- The same search capabilities are available from one MCP tool.
- `neus` is preferred when available; missing neus falls back to `rg` for code and fuzzy search for docs.
- No harnez search index or cache is added.
