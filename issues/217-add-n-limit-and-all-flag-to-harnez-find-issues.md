# 217 — Add -n Limit (Default: 10) and --all Flag to harnez find issues

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature / CLI
**Related**: [158-find-entity-query-command.md](158-find-entity-query-command.md), [194-reserve-next-issue-number.md](194-reserve-next-issue-number.md), [docs/CLIDesign.md](../docs/CLIDesign.md)

---

## 1. Problem & Motivation

`harnez find issues` searches and lists issues (e.g. `harnez find issues status:open`).
Currently:
1. Running `harnez find issues status:open` prints every matching issue across the repository (often 50–100+ tickets), flooding the terminal.
2. In interactive agent or human workflows, the most common inquiry is inspecting the **most recent** issues or a limited batch of top results. Users currently have to pipe to `tail -n 10` or `head -n 10`.
3. Running `harnez find issues` without a query currently errors with `find: query must not be empty`, preventing a quick view of recently reported issues.

---

## 2. Technical Specification

### 2.1 CLI Flags on `harnez find issues`
Add flags to `cmd/harnez/find.go`:
- `-n, --limit <int>` (default: `10`):
  - Limits the output to at most *N* entries.
  - For empty or broad status queries (e.g. `harnez find issues` or `harnez find issues status:open`), returns the last *N* issues (highest ticket numbers / newest).
  - When *N* is specified explicitly (e.g. `-n 5` or `-n 25`), bounds results to *N*.
  - `-n 0` or negative value is rejected with a clear usage error.
- `--all, -a` (bool, default: `false`):
  - Bypasses the default `-n 10` limit, returning all matching results.
  - When `--all` is set, `-n` limit is ignored or unbounded.

### 2.2 Default Query Behavior
- Allow running `harnez find issues` without arguments (or with only flags):
  - Defaults to listing the last 10 issues (equivalent to `harnez find issues -n 10`).
  - When combined with status filters like `harnez find issues status:open`, limits output to the last 10 open issues by default.

### 2.3 Slice / Ordering Contract
- `harnez find issues` sorts numerically by ticket number when ranks match.
- For a query that lists issues (like `status:open` or bare query), taking the last *N* issues should select the most recent / highest numbered tickets (e.g., ticket 208, 209, 210... 217), keeping them cleanly accessible.
- For ranked text search (e.g. `harnez find issues "vram"`), limit selects the top *N* best matches according to the ranking contract, unless `--all` is specified.

---

## 3. Implementation & Verification Plan

- [ ] Update `cmd/harnez/find.go` flags (`-n`/`--limit`, `--all`).
- [ ] Update `cmd/harnez/find.go` argument parsing to allow empty query when flags like `-n` or `--all` or no query are provided.
- [ ] Implement slice truncation logic in `cmd/harnez/find.go` / `internal/find`.
- [ ] Add unit and CLI integration tests in `cmd/harnez/find_test.go` and `internal/find/`.
- [ ] Run `make check` and `make install`.
- [ ] Update documentation / help text in `cmd/harnez/find.go`.

