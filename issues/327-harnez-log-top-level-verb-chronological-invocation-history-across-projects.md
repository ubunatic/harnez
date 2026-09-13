# 327 — harnez log top-level verb: chronological invocation history across projects

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: issues/326 (`cli_invocations` write path — hard dependency), issues/328, issue 318 (bare `-N` pre-parser), issue 228 (`find issues history`), issue 120 (`harnez stats`), issue 192 (`harnez dochistory`), `docs/CLIDesign.md`

---

## 1. Problem & Motivation

Requested by the project owner: *"a `harnez log` command like `git log` — look at all
harnez-driven changes, a shortcut to any kind of history of what harnez has done in any
repository."* The owner explicitly prefers a **top-level verb**, not a flag on an existing
command.

The valuable, genuinely-new part of that idea is **invocation history**: which harnez
subcommands ran, when, in which project, by whom, and whether they succeeded. Issue 326
creates the store; this ticket is the read surface.

### What `harnez log` must deliberately NOT do

Repo *content* history is already fully covered and must not be re-implemented under a new
name:

| Question | Already answered by |
|---|---|
| What changed in `docs/` and `issues/`? | `git log --oneline -- docs/ issues/` (docs and issues live in the git tree by design) |
| How have managed docs evolved in size/category? | `harnez dochistory` (`cmd/harnez/dochistory.go`, issue 192) |
| How have open/closed ticket counts moved? | `harnez find issues history` (issue 228) |
| How often does a tool fail, what did distillation save? | `harnez stats` (issue 120) |

`harnez log` answers only the one question none of those can: **what did harnez itself run,
and when**. `find issues history` and `stats` stay exactly where they are — they are not
renamed, aliased, or moved under `log`. `stats` is an *aggregate* over tool calls; `log` is a
*chronological event stream* over CLI invocations. Different data source, different shape.

## Why this needs real work, not a quick patch

- **A new top-level verb has to earn its place in an explicitly load-bearing taxonomy.**
  `docs/CLIDesign.md` documents `apply`/`init` as deliberately disjoint, and issue 318
  records the `find` = read/discovery vs `issues` = status-mutation split as the same kind of
  "load-bearing" boundary. `log` is read-only and touches *harnez's own invocation record*,
  not repository entities — a third axis alongside `find` (repo entities) and `stats`
  (aggregates). That justification must land in `docs/CLIDesign.md`'s command table as part
  of this ticket, or the next agent will reasonably ask why this isn't `find invocations`.
- **The kitchen-sink failure mode is the default outcome here.** "History of everything
  harnez did" invites `harnez log docs`, `harnez log issues`, `harnez log stats` subcommands
  that wrap or duplicate `dochistory`, `find issues history`, and `stats`. That would give
  this project two names for each of three features and guarantee they drift. The mitigation
  is structural: **`harnez log` takes no subcommands at all.** Its `Long` help instead
  cross-references the four commands in the table above by name, so discoverability is
  preserved without duplication.
- **Zero-arg behaviour is specified, not incidental.** `docs/CLIDesign.md` §"Interactive
  discovery & bounded ingestion" requires bounded defaults, forgiving zero-arg defaults, and
  deterministic parsable output. A bare `harnez log` in an agent loop must not dump the whole
  table into the context window.

## 2. Technical Specification / Findings

### Command shape

```
harnez log [-N | -n <limit>] [--all] [--project <name>] [--session <id>] [--auto]
           [--agent <id>] [--human] [--command <name>] [--since <dur>] [--failed]
           [--dir <path>] [--json]
```

- **Zero-arg default**: the last 20 invocations for the project inferred from the current
  directory, newest first. Falls back to all projects when the cwd is not a known project.
- **`-N` bare limit**: reuse issue 318's shared bare-`-N` pre-parser (`cmd/harnez/bareflag.go`)
  so `harnez log -5` works exactly like `git log -5`. This is the direct precedent that makes
  the whole command feel git-shaped; do not add a second parser.
- **`--human` / `--agent <id>`**: filter by attribution (see issues/328).
- **`--failed`**: only rows with a non-zero `exit_code` — the highest-value filter for
  "what went wrong in this repo".
- **`--auto`**: filter to the current session, matching `harnez stats --auto`'s existing flag
  name and semantics (`cmd/harnez/stats.go:82`).
- **`--all`**: uncap, per CLIDesign's required escape hatch.

### Output

House style, matching `runFindHistory`'s `tabwriter` table (`cmd/harnez/find.go`):

```
TIME                  PROJECT   WHO     COMMAND      EXIT  DUR
2026-09-13T10:22:41Z  harnez    claude  issues new   0     41ms
2026-09-13T10:22:09Z  harnez    claude  index        0     88ms
2026-09-13T09:58:02Z  smarthome human   apply        1     210ms
```

- `--json` emits the row slice, matching `find issues history --json` and `stats --json`.
- Empty result prints a `no data: ...` line naming the likely cause, as `runFindHistory`
  already does for an unpopulated `issue_status_snapshots`.

### Backing data source

`cli_invocations` only (issues/326). No git-log parsing, no reads of
`~/.harnez/sessions/*.usage.json`. If 326's table is absent or empty, `log` says so and
exits 0.

## 3. Implementation & Verification Plan

### Proposed scope

- New `cmd/harnez/log.go` with `newLogCmd()`, registered in `main.go`'s `root.AddCommand(...)`.
- Query via issue 326's `QueryCLIInvocations`; all filtering pushed into the SQL `Filter`,
  no Go-side post-filtering of an unbounded result set.
- Wire the issue-318 bare-`-N` pre-parser.
- Add a `log` row to `docs/CLIDesign.md`'s command-responsibility table, with one line stating
  the `log` (invocations) vs `find`/`stats`/`dochistory` boundary.

### Verification

- [ ] `harnez log` with no args returns at most 20 rows, newest first, scoped to the
      current project.
- [ ] `harnez log -5` matches `harnez log -n 5` output exactly.
- [ ] `harnez log --failed` returns only non-zero-`exit_code` rows.
- [ ] `harnez log --project X` and `--command index` filter correctly, and combine.
- [ ] `--json` output round-trips to the same rows as the table rendering.
- [ ] With no telemetry DB or an empty table, `harnez log` prints a `no data:` line and
      exits 0 (does not error).
- [ ] `harnez log --help` names `git log`-style usage and cross-references `harnez stats`,
      `harnez find issues history`, `harnez dochistory`, and `git log -- docs/ issues/`.
- [ ] `harnez log` has **no** subcommands (assert the cobra command's `Commands()` is empty).
- [ ] `docs/CLIDesign.md` table includes `log`; `go test ./...` passes.
