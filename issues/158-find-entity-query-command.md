# 158 — Add a `harnez find <entity> <query>` query command

**Status**: Open
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

Add a deliberate `harnez find <entity> <query>` command. Its first useful
entity is `issues`, making these invocations clear and predictable:

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
- Use `harnez find <entity> <query>` (one or more remaining arguments joined
  with a single space) rather than inventing entity-specific top-level
  commands. A quoted query is recommended when it contains spaces or `|`,
  but the shell's normal argument splitting must not change AND semantics.
- Place `find` as a new top-level verb. It is a repository-data discovery
  operation, not a mode of `status`, `index`, or `assess`; no existing command
  provides a natural parent. This is consistent with the concrete-placement
  reasoning in [[154]] and does not reopen the broader assessment closed in
  [[153-command-tree-placement-spec-assessment]].

### Version-one issue-query grammar

- Bare terms use forgiving, case-insensitive fuzzy text matching over each
  issue's title and body. Define a deliberately small, deterministic v1
  matcher (for example normalized substring and/or token-prefix matching),
  document its searchable fields and normalization in `--help`, and cover it
  with tests. Do not require users to learn regexes, exact phrases, or a
  formal Boolean language for ordinary discovery.
- Whitespace between terms is AND: `vram gtt` means issues containing both
  terms.
- `|` separates OR alternatives within one term: `vram|gtt` means either;
  `vram|gtt memory` means `(vram OR gtt) AND memory`.
- A recognized `field:value` token is a filter ANDed with other terms.
  Version one supports `status:<value>` and the alias `is:<value>`, with the
  same accepted normalized values as the tracker (`open`, `in-progress`,
  `blocked`, `closed`, `draft`). It must match statuses with explanatory
  suffixes (for example `Open — deferred`).
- Boolean precedence is fixed: field filters and whitespace AND bind outside
  OR, so `status:open vram|gtt` is `status:open AND (vram OR gtt)`. Parentheses,
  negation, field OR, regexes, and implicit phrase syntax are explicitly out
  of scope for v1 and must produce either literal-text behavior documented in
  `--help` or a clear parse error — choose one, test it, and do not guess.
- Malformed filters (empty `status:`, an unknown field, unsupported status,
  dangling/empty OR alternative) are usage errors, not zero-result searches.

### No-result behavior: preserve exactness

`harnez find` must return only exact matches by default. It must **not**
silently relax `is:open vram gtt` into `is:open AND (vram OR gtt)`: that makes
an apparently precise query return issues that do not satisfy it.

After the exact engine and output contract are stable, assess an explicit,
opt-in discovery mode such as `--relax-and` (or a separately named
`--suggest-relaxation`) that, only after zero exact results, prints a clearly
labeled alternative query and its results. It must never alter the default
result set, exit status, or machine-readable output. Do not select a flag or
implement this mode until its UX is designed and tested.

### Output, exit status, and errors

- Default output is deterministic, one compact result per line: ticket number,
  status, title, and repository-relative ticket path. Sort by numeric ticket
  number ascending. Keep Markdown/ANSI out of the default format.
- Add `--json` only if the implementation can define and test a stable
  schema (number, title, raw status, path); otherwise leave it as a follow-up.
- Successful search, including zero exact matches, exits 0. Zero results print
  no result lines; a succinct `no issues matched` diagnostic may go to stderr
  only if it does not compromise pipe-friendly stdout. Invalid entity/query,
  unreadable tracker data, or malformed filters exit non-zero with actionable
  stderr.

## 3. Implementation & Verification Plan

- [ ] Add a top-level Cobra `find` command, accepting an entity and a
      non-empty query; document grammar and examples in its long help and the
      README command reference.
- [ ] Reuse or extend `internal/issues` scanning/parsing rather than scraping
      `issues/README.md`; include both active and archived issue files only
      when the chosen `issues` entity contract says so. **Decision needed at
      implementation:** search active tickets only by default, or all tracker
      tickets; whichever is chosen must be named in help and covered by tests.
- [ ] Implement parsing/evaluation as a small independently tested package,
      not inline Cobra argument handling. Test aliases, normalization,
      whitespace AND, `|` OR, precedence, field-plus-text combinations,
      matching status suffixes, and every malformed-query error.
- [ ] Test a temporary tracker fixture with open, closed, archived, and
      suffix-status tickets; assert deterministic output, zero-match exit 0,
      and non-zero usage errors.
- [ ] Confirm no default fallback relaxation occurs for a no-result AND
      query. If an opt-in relaxation UX is later added, file or amend a
      focused follow-up with exact output and exit semantics first.
- [ ] Run `go test ./...`, `make install`, `harnez status`, and update the
      README command reference plus this ticket/index before closure.

## 4. Likely Impacted Files

- `cmd/harnez/main.go` or a focused `cmd/harnez/find.go` for Cobra wiring.
- A new `internal/find` (or similarly narrow) parser/evaluator package and
  tests, using `internal/issues` records.
- `README.md` command reference and `issues/README.md`.

## 5. Out of Scope

- Full Boolean expressions, parentheses, negation, regexes, and arbitrary
  filesystem search.
- Searching entities other than `issues` in v1.
- Any implicit query relaxation that changes exact search semantics.
