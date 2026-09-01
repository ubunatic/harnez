# 158 — Add a `harnez find <entity> <query>` query command

**Status**: Closed — resolved in 6924a6b
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `cmd/harnez/main.go`, `internal/issues`, [[148-harnez-index-command-for-issues-docs-studies]], [[154-brief-git-repo-status-command]]

---

## 1. Problem & Motivation

The repository's Markdown issue tracker is searchable only through ad-hoc
shell commands. Those commands do not offer a stable, user-facing query
language for status filtering and Boolean text search, and they force agents
and users to remember the tracker layout.

Add a deliberate `harnez find <entity> [options] <query...>` command. Its
first useful entity is `issues`, making these invocations clear and
predictable:

```console
$ harnez find issues "is:open vram gtt"
# exact interpretation: is:open AND vram AND gtt

$ harnez find issues "status:open vram|gtt"
# exact interpretation: status:open AND (vram OR gtt)
```

The intended product feel is a short, intuitive query language with
Google/Gmail-like fuzzy text discovery: people should normally be able to
type the words they remember, while compact filters and `|` refine results
without requiring a formal search grammar.

## 2. Product Decisions and Query Contract

### Entity scope and command shape

- Implement `issues` as the only supported entity in the first release.
  Reject unknown entities with a concise error that names the valid entity;
  do not silently treat an entity as a directory or search arbitrary files.
- Use `harnez find <entity> [options] <query...>` rather than inventing
  entity-specific top-level commands. Parse flags with Cobra, trim the
  remaining query arguments, and join them with one space. Reject a missing
  or whitespace-only query.
- Add `-d, --dir <path>` to select the repository root, defaulting to `.` in
  the same style as `harnez index` and `harnez repo-status`. Search
  `issues/*.md` and `issues/archive/*.md`, excluding `issues/README.md` and
  every other directory. Active and archived issues are both in scope by
  default; use a status filter to narrow lifecycle state.
- Separate AND terms may be passed as separate shell arguments. An OR query
  must quote the full query or escape the pipe because an unquoted `|` is a
  shell pipeline operator:

  ```console
  harnez find issues status:open vram gtt
  harnez find issues "status:open vram|gtt"
  ```

  Quotes in these examples are shell quoting only; v1 has no phrase syntax.
- Place `find` as a new top-level verb. It is a repository-data discovery
  operation, not a mode of `status`, `index`, or `assess`; no existing command
  provides a natural parent. This is consistent with the concrete-placement
  reasoning in [[154]] and does not reopen the broader assessment closed in
  [[153-command-tree-placement-spec-assessment]].

### Searchable document and deterministic fuzzy matching

- Search the H1 title after removing its leading ticket number and the issue
  body after the first metadata-closing horizontal rule. Do not search the
  metadata header itself. Preserve text inside code spans/fences and link
  destinations so remembered identifiers and paths remain discoverable.
- Normalize both searchable text and bare alternatives by Unicode lowercasing,
  replacing Markdown syntax and non-letter/non-number characters with spaces,
  and collapsing whitespace. This deliberately turns punctuation-separated
  identifiers such as `time-gauge` into adjacent searchable tokens without
  exposing regex syntax.
- A bare alternative matches a field when its complete normalized text is a
  substring of that field, or when each of its normalized tokens matches a
  field token by prefix or bounded typo distance. Use Damerau-Levenshtein
  distance so a transposition counts as one edit: terms of four characters or
  fewer receive no typo expansion, terms of five through eight allow one edit,
  and terms of nine or more allow two. This protects short identifiers such as
  `gtt` and `vram` from noisy fuzzy expansion.
- Every whitespace-separated group must match. For an OR group, retain the
  best matching alternative. Assign match classes in this order:

  1. exact normalized substring in title;
  2. token-prefix match in title;
  3. bounded fuzzy-token match in title;
  4. exact normalized substring in body;
  5. token-prefix match in body;
  6. bounded fuzzy-token match in body.

  Rank a result by its worst group class, then the sum of all group classes,
  then numeric ticket number, then repository-relative path. This puts strong
  title matches ahead of weak body matches while remaining deterministic.
- Document the searchable fields, normalization, fuzzy thresholds, and
  ranking in `--help`. Do not require regexes or a formal Boolean language for
  ordinary discovery.

### Version-one issue-query grammar

- Whitespace between terms is AND: `vram gtt` means issues containing both
  terms.
- `|` separates OR alternatives within one term: `vram|gtt` means either;
  `vram|gtt memory` means `(vram OR gtt) AND memory`.
- A recognized `field:value` token is a filter ANDed with other terms.
  Version one supports `status:<value>` and the alias `is:<value>`.
  `status:open` and `is:open` match every unresolved issue whose leading raw
  lifecycle is `Open`, `In Progress`, or `Blocked`. `in-progress` and
  `blocked` narrow to their corresponding leading raw lifecycle;
  `closed` and `draft` match their canonical categories. Matching ignores
  case and explanatory status suffixes, but the accepted query spellings are
  exactly `open`, `in-progress`, `blocked`, `closed`, and `draft`.
- Boolean precedence is fixed: field filters and whitespace AND bind outside
  text OR, so `status:open vram|gtt` is `status:open AND (vram OR gtt)`.
- `|` is valid only inside one compact text group. A standalone pipe, leading
  or trailing pipe, empty alternative (`a||b`), whitespace around a pipe, or
  filter alternative (`status:open|closed`) is a usage error. Parentheses,
  literal quote characters, negation, an empty filter, unknown fields, and
  unsupported status values are also usage errors with an actionable hint.
  Regex-looking punctuation is normalized as ordinary text and is never
  executed. These cases must not degrade into zero-result searches or guessed
  Boolean semantics.

### No-result behavior: preserve the query

`harnez find` must return only issues satisfying every AND group under the
defined fuzzy matcher. It must **not** silently relax `is:open vram gtt` into
`is:open AND (vram OR gtt)`: that makes an apparently precise query return
issues that do not satisfy it.

An explicit relaxation mode may be assessed in a separate ticket after this
contract is stable. It must not be selected or implemented as part of v1.

### Output, exit status, and errors

- Default output is UTF-8, deterministic, and tab-separated, with exactly one
  result per line:

  ```text
  NUMBER<TAB>RAW_STATUS<TAB>PLAIN_TITLE<TAB>REPOSITORY_RELATIVE_PATH
  ```

  Remove the leading `NNN —` portion and Markdown formatting delimiters from
  the displayed title, collapse embedded whitespace, and print paths such as
  `issues/158-find-entity-query-command.md`. Replace any embedded tabs or
  newlines in status/title fields with spaces. Emit no ANSI styling or header.
- Apply the fuzzy relevance ordering defined above. Raw status is displayed so
  suffixes such as `Blocked — waiting for upstream` remain useful.
- Successful search, including zero matches, exits 0. Zero matches emit
  nothing on stdout or stderr. Invalid entity/query, a missing/unreadable
  tracker, or malformed ticket data exits non-zero with actionable stderr.
- `--json` is out of scope for v1; add it only in a follow-up with a separately
  specified stable schema and ordering contract.

## 3. Implementation & Verification Plan

- [x] Add a top-level Cobra `find` command, accepting an entity, `-d/--dir`,
      and a non-empty query; document grammar, shell quoting, fuzzy behavior,
      ranking, output, and examples in its long help and README command
      reference.
- [x] Reuse or extend `internal/issues` scanning/parsing rather than scraping
      `issues/README.md`. Extend its issue representation or add a focused
      search document type so the post-metadata body is available. Cover both
      active and archived files and exclude metadata from bare-text matching.
- [x] Implement parsing/evaluation as a small independently tested package,
      not inline Cobra argument handling. Test aliases, normalization, fuzzy
      thresholds and transposition, short-token protection, whitespace AND,
      `|` OR, precedence, ranking, field-plus-text combinations, exact status
      lifecycle behavior, suffixes, and every malformed-query error.
- [x] Test a temporary tracker fixture with open, closed, archived, and
      suffix-status tickets. Include hits found only in body text and code,
      text appearing only in metadata that must not match, title/body ranking,
      stable ties, exact tab-separated output, silent zero-match exit 0, and
      non-zero usage errors.
- [x] Confirm no default Boolean relaxation occurs for a no-result AND query.
- [x] Run `go test ./...`, `make install`, `harnez status`, and update the
      README command reference plus this ticket/index before closure.

## 4. Likely Impacted Files

- `cmd/harnez/main.go` or a focused `cmd/harnez/find.go` for Cobra wiring.
- A new `internal/find` (or similarly narrow) parser/evaluator package and
  tests, using `internal/issues` records.
- `README.md` command reference and `issues/README.md`.

## Scope Note (added at close)

Two judgment calls where the spec text didn't fully pin down real-repo edge behavior:

- **Body extraction when no metadata-closing rule exists.** The spec defines the body as
  "everything after the first metadata-closing horizontal rule," but only ~58 of this repo's
  137 tickets actually use a `---` line after the header (newer tickets, including this one,
  do; many older ones go straight from `**Related:**` to the first `## ` heading with just a
  blank line). At the time this ticket closed, `internal/issues.ParseBody` implemented the rule
  literally: if no thematic break (`---`/`***`/`___`) appeared after the H1 title, `Body` was `""`
  and the ticket was searchable by title only. **This limitation is resolved by
  [[162-find-body-extraction-three-tier-fallback]]**, which extended `ParseBody` to a three-tier
  fallback (`---`/`***`/`___` → first `## ` heading → whole document after the H1 title), removing
  the empty-body/title-only-search case for every ticket that has an H1 title.
- **What counts as "malformed ticket data."** Section 2 lists "malformed ticket data" alongside
  "missing/unreadable tracker" as a non-zero-exit case. A ticket file simply missing its
  `**Status:**` tag or H1 (already tolerated elsewhere, e.g. `harnez status`'s
  `missing_status_tag` diagnostic) is common and not treated as fatal here — it degrades to an
  empty/unfiltered status field rather than aborting the whole search. "Malformed/unreadable"
  is implemented as it already was in `internal/issues.Scan`: a missing/unreadable `issues/`
  directory or an unreadable individual file returns an error; a loosely formatted ticket does
  not.

## 5. Out of Scope

- Full Boolean expressions, parentheses, negation, regexes, and arbitrary
  filesystem search.
- Searching entities other than `issues` in v1.
- Any implicit query relaxation that changes exact search semantics.
- Phrase search, JSON output, and configurable ranking/fuzzy thresholds.
