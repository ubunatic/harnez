# 340 — Support label, project, and category query filters in harnez find issues

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Ergonomics / Feature
**Category**: Issue Tracker / CLI Ergonomics
**Related**: [docs/practices/IssueTracking.md](../docs/practices/IssueTracking.md), [issues/158](158-find-issues-fuzzy-query-grammar-and-ranking.md)

---

## 1. Problem & Motivation

`harnez find issues` provides fast fuzzy searching and structured filtering across repository tickets. However, its query grammar ([`internal/find/query.go`](file:///home/uwe/projects/harnez/internal/find/query.go#L34)) currently only supports `status:` and its alias `is:` (e.g. `harnez find issues status:open`).

While full-text search matches words in the title and body, it cannot restrict matches strictly to structured metadata. As repository trackers scale and multi-ticket initiatives / epics form (e.g. the macOS portability project across tickets 286, 334–339), agents and developers need to query tickets by metadata attributes such as:
- `harnez find issues project:macos status:open`
- `harnez find issues category:architecture`
- `harnez find issues label:portability` or `tag:performance`

## 2. Technical Specification

### 2.1 Metadata Header Parsing (`internal/issues/issues.go`)
Extend `IssueFile` to parse optional metadata fields from the markdown header block (between `# NNN` and the first `---` rule):
- `**Category:** <category>` (e.g. `Architecture / Multi-OS`, `Bug`, `Feature`)
- `**Project:** <name>` (e.g. `macOS-Portability`, `Agentic-Loop`)
- `**Labels:** <tag1, tag2>` or `**Tags:** <tag1, tag2>` (comma-separated list, e.g. `macos, terminal, performance`)

### 2.2 Query Grammar Extension (`internal/find/query.go`)
Extend `splitFilter` and `Filter` evaluation to accept:
- `category:<name>` / `cat:<name>`: matches normalized ticket category (case-insensitive, hyphen-tolerant).
- `project:<name>` / `epic:<name>`: matches ticket project name.
- `label:<tag>` / `tag:<tag>`: matches any tag in the ticket's labels list.
- Multiple filters are combined with logical AND (e.g. `status:open project:macos label:terminal`).

### 2.3 CLI & JSON Output (`cmd/harnez/find.go`)
- Ensure `harnez find issues --json` includes the parsed `category`, `project`, and `labels` arrays in each result object.
- Provide actionable error hints when an unsupported filter key or malformed syntax is supplied.

## 3. Implementation & Verification Plan

1. **Parser & Struct Tests**:
   - Add unit tests in `internal/issues/issues_test.go` verifying parsing of `Category`, `Project`, and `Labels`/`Tags` from header blocks.
2. **Grammar & Matching Tests**:
   - Add unit tests in `internal/find/query_test.go` and `internal/find/match_test.go` covering `category:`, `project:`, `label:`, and compound filter queries.
3. **Docs Update**:
   - Update `docs/practices/IssueTracking.md` with the new metadata header fields and `harnez find` query examples.
4. **Regenerate Index**:
   - Run `harnez index -d .` and verify `make check`.
