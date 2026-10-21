<!-- harnez:topic: Research report (argon:low) on ticket hygiene at ~900 tickets: status tiers, directory splits, and pre-flight duplicate promotion -->
# Study: Ticket Hygiene at ~900 Tickets (Researcher: argon)

**Date**: 2026-10-21
**Ticket**: [842](../../issues/842-research-ticket-hygiene-at-900-tickets-archive-backlog-tiers-short-open-lists-duplicate-detection.md)
**Scope**: `internal/issues`, `internal/find`, `internal/index`, `cmd/harnez/issues.go`, `docs/IssueTracking.md`, `issues/*.md`. Evaluates three file-based architectures to cap active tickets at ~20 (≤100), lower agent context costs, isolate backlog/archive tiers to explicit requests, and enforce duplicate detection with promotion/reopening across all tiers.

---

## 1. Corpus Audit & Grooming Baseline

As of 2026-10-21 (`842`), `issues/` holds **836 ticket files** (`817` in `issues/`, `19` in `issues/archive/`) and an 843-line `issues/README.md`:
- **By canonical status**: **249 Open** (238 raw `Open` / `Open — <note>`, 7 `Blocked`, 4 `In Progress`), **562 Closed** (543 still sitting unarchived in `issues/`), and **17 Draft**.
- **By age (`status:open`)**: `<200`: 34 tickets; `200–399`: 66; `400–599`: 63; `600+`: 86.
- **By priority (`status:open`)**: 8 `P1`, 156 `P2`, 72 `P3`, 1 `P4 (Backlog)` (`825`), ~12 untagged early tickets (`005`–`016`).
- **Why `status:open` is 249 rows**:
  1. **Unexecuted `docs/Roadmap.md` §9 actions (~40 tickets)**: `508`, `601`, `544`, `302`, `342`, `152` are marked "Close candidate" (work shipped in `603`, `598`, etc.); 24 usage/view/mic tickets (`051`, `084`, `085`, `141`, `146`, `160`, `161`, `172`, `214`, `219`, `247`, `250`–`256`, `264`, `278`, `295`, `330`, `404`) are parked awaiting `../loom` migration or `560`; Dot8 tickets (`436`, `440`, `441`, `444`, `447`, `459`) are on hold.
  2. **Unconnected P2/P3 tail (~180 tickets)**: `docs/Roadmap.md` ("Now" bucket) schedules only **~12 active items**. The remaining ~180 tickets are valid `Next`/`Later` ideas or opportunistic polish that sit in `Open` because `IssueTracking.md` has no status between `Open` and `Closed`.
  3. **Duplicate creep**: e.g., `697` (P2) and `698` (P1) both specify "teach agents when and how to release repos". Because `harnez issues new` does not gate on duplicate search, agents skip or under-query `harnez find issues`.

---

## 2. Three Proposed Solutions

### Solution A: Lifecycle Status Tiering (`Backlog` / `Parked`) + Compact Index + `new` Duplicate Gate

Keep files flat in `issues/` (moving `Closed` to `issues/archive/`), but split "unresolved" into **Active** (`Open`, `In Progress`, `Blocked`) vs **Deferred** (`Backlog` / `Parked`).

- **Mechanism**:
  - Add canonical category `StatusBacklog` (`Backlog`, `Parked`, `Deferred`) in `internal/issues/issues.go` (`CanonicalizeStatus`, `LeadingLifecycle`) and verb `harnez issues backlog|park <n> [reason]`.
  - Keep active + backlog files in `issues/NNN-*.md`; move all 562 `Closed` files to `issues/archive/NNN-*.md` via a new `harnez issues archive` sweep command.
  - Compact `issues/README.md`: `harnez index` renders only Active (`Open`/`In Progress`/`Blocked`) and `Draft` rows in `issues/README.md`, plus a 1-line summary pointer to `issues/archive/README.md` (which indexes `Backlog` and `Closed`).
- **Listing & Search Behaviour**:
  - `harnez find issues -a status:open` and `harnez issues list` match **only** `Open`, `In Progress`, `Blocked` (~20 tickets, <1 KB context).
  - New filters: `status:backlog` (deferred only) and `status:unresolved` (active + backlog).
  - Unfiltered `harnez find issues "<query>"` still scans `issues/*.md` and `issues/archive/*.md` across all statuses (`Open`, `Backlog`, `Closed`, `Draft`).
- **Duplicate Flow**:
  - `harnez issues new "<title>"` automatically runs `find.Search` (title + body tokens) across all 836+ files (active, backlog, archived) before reserving a number.
  - If any candidate scores in `ClassTitleExact`–`ClassTitleFuzzy` (1–3) or multiple `ClassBodyExact` tokens, `new` aborts reservation (exit 2) and prints the top matches with actionable commands: `harnez issues open <id> "promoted: <context>"` (reopens from `Closed` or promotes from `Backlog` and moves out of `issues/archive/` if needed). Pass `--force` to confirm non-duplicate and allocate.
- **Migration of ~247 Active Tickets**:
  1. Close the ~10 shipped/superseded tickets noted in `Roadmap.md` §9 (`508`, `601`, `544`, `302`, `342`, `152`, `697` into `698`).
  2. Run `harnez issues backlog <ids...> "deferred to backlog"` on all ~215 tickets not in `Roadmap.md`'s **Now** bucket (or without `P0`/`P1` urgency).
  3. Run `harnez issues archive` to move all 562 closed tickets into `issues/archive/`. Result: **~20 Open**, ~215 Backlog, ~570 Closed in `archive/`.
- **Tooling & Doc Changes**: `internal/issues` (`StatusBacklog`), `internal/find` (`status:backlog|unresolved`), `cmd/harnez/issues.go` (`backlog`/`park` verb, `archive` verb, `new --force` preflight), `internal/index` (split table), `docs/practices/IssueTracking[.lite].md`, `.harnez/rules/Tools.md`, `commands/issue.md`, `commands/roadmap.md`.
- **Pros**: Zero link breakage for active/backlog cross-references; no directory thrashing when promoting/demoting between `Open` and `Backlog`; backward-compatible with `harnez find issues -a status:open` muscle memory.
- **Cons**: `issues/` directory still has ~235 `.md` files on disk (`ls issues/` remains noisy if an agent violates rules).
- **Risks**: Agents calling `harnez issues new` in scripts without `--force` may hit the duplicate gate on generic titles; mitigated by printing exact `--force` / `open <id>` instructions.

---

### Solution B: Three-Tier Directory Isolation (`issues/`, `issues/backlog/`, `issues/archive/`)

Enforce physical directory separation so `issues/` holds **only** the ~20 active sprint tickets on disk.

- **Mechanism**:
  - Three directories: `issues/*.md` (Active: `Open`, `In Progress`, `Blocked`), `issues/backlog/*.md` (Unscheduled `Open`/`Backlog`/`Draft`), and `issues/archive/*.md` (`Closed`).
  - `issues.Scan` walks all three (`"."`, `"backlog"`, `"archive"`).
  - Status verbs move files automatically via `git mv`: `harnez issues defer <n> [reason]` moves `issues/NNN-*.md` → `issues/backlog/NNN-*.md`; `harnez issues close <n>` moves to `issues/archive/`; `harnez issues open <n>` promotes from `backlog/` or `archive/` back to `issues/NNN-*.md`.
- **Listing & Search Behaviour**:
  - `harnez find issues status:open` (and `issues list`) restricts to top-level `issues/*.md` by default.
  - Explicit tier flags/filters (`status:backlog`, `status:closed`, or `tier:all`) opt into `issues/backlog/` and `issues/archive/`.
  - Text queries (`harnez find issues "<query>"`) always search all three directories and tag each result row with its path (`issues/backlog/...`, `issues/archive/...`).
- **Duplicate Flow**:
  - `harnez issues new "<title>"` scans all three directories. On a match in `backlog/` or `archive/`, running `harnez issues open <id> "<reason>"` atomically `git mv`s the file back to `issues/NNN-*.md`, sets `**Status**: Open — <reason>`, updates `issues/README.md`, and commits.
- **Migration of ~247 Active Tickets**:
  - Batch-move 543 unarchived closed files to `issues/archive/`.
  - Keep `Roadmap.md` **Now** + `In Progress` (~20 tickets) in `issues/`; `git mv` the remaining ~220 open tickets into `issues/backlog/` in one `harnez issues defer` batch.
- **Tooling & Doc Changes**: Update `issues.ScanFS`, `LintFS`, `findTicketFile`, `runIssuesVerb` (add file relocation + `rewriteIssueReferences` from `runIssuesMv`), `issues_rebase.go` (track `issues/backlog/` in `collectLocalTickets`), `IssueTracking.md`.
- **Pros**: Cleanest filesystem (`ls issues/` shows ~20 files); `issues/README.md` naturally shrinks if scoped to top-level `issues/`.
- **Cons**: Every status transition (`open` ↔ `defer` ↔ `close`) renames paths in Git and requires rewriting relative markdown links across `issues/` and `docs/` (high commit diff noise).
- **Risks**: Merge/rebase conflicts when one branch edits `issues/NNN.md` and another moves it to `issues/backlog/NNN.md`; `issues rebase` and external links in studies/commits break or need link-rewriting on every status flip.

---

### Solution C: Curated Active Working Set (`Priority: P0/P1` or Focus Pin) + Bounded Listing Defaults

Keep `Status: Open` for all unresolved tickets, but separate the **Active Focus (~20)** from the **Open Pool (~220)** via priority/focus metadata and strict CLI listing defaults.

- **Mechanism**:
  - Move all 543 closed tickets in `issues/` to `issues/archive/` (enforced automatically on `harnez issues close`).
  - Redefine `status:open` vs `status:active`: add `priority:p0|p1|p2|p3|p4` filter to `harnez find issues` (extending ticket `340`) and a `focus` verb (`harnez issues focus <n>` / `unfocus <n>`, setting `**Focus**: Active` or promoting priority to `P1`).
  - Change `harnez find issues` and `harnez issues list` so `-a` is no longer recommended in `.harnez/rules/Tools.md`; default `harnez issues list` (and `status:active`) returns only focused / `P0`–`P1` / `In Progress` tickets (capped at ≤25).
- **Listing & Search Behaviour**:
  - Default `harnez issues list` / `harnez find issues active` returns the ~20 active focus tickets.
  - Full open pool (~240 tickets) requires explicit `harnez find issues -a status:open` or `priority:p2`.
  - `harnez find issues "<query>"` searches all 836+ files (including `issues/archive/`) and ranks exact/prefix matches first.
- **Duplicate Flow**:
  - Add `harnez issues dup-check "<title>" [terms...]` (also run automatically inside `harnez issues new "<title>"`), which queries `issues/` + `issues/archive/` and boosting title/body token overlap.
  - When an archived (`Closed`) or unfocused (`P2`/`P3`) ticket matches, the agent runs `harnez issues focus <id> "reopened/promoted: <new context>"` which moves archived files from `issues/archive/` to `issues/`, sets `Status: Open`, marks focus/P1, and commits.
- **Migration of ~247 Active Tickets**:
  - Sweep all 543 closed tickets from `issues/` to `issues/archive/`.
  - Close the ~10 obsolete `Roadmap.md` §9 tickets.
  - Mark the ~20 `Roadmap.md` **Now** + `In Progress` tickets as `Focus: Active` (or `P1`); leave the remaining ~215 tickets in place as unfocused `P2`/`P3` (`P4` normalized to `P3`).
- **Tooling & Doc Changes**: Implement metadata filter parsing (`priority:`, `focus:`, `category:` from `340`) in `internal/find`, auto-move to `issues/archive/` on `close` and back to `issues/` on `open`, update `.harnez/rules/Tools.md` to replace `harnez find issues -a status:open` with `harnez issues list` (`status:active`).
- **Pros**: No mass status-line rewrites on the 215 open tickets during migration; preserves `Open` semantics for external tools; lands metadata filters (`340`) needed by `/roadmap`.
- **Cons**: If agents still run `harnez find issues -a status:open` from old prompt habits, they still ingest 240 lines; requires a second metadata concept (`Focus` or strict `P1` cap).
- **Risks**: Focus/priority inflation over time unless `harnez status` / `issues lint` warns when active focus exceeds a configured cap (e.g. `max_active_issues: 30` in `spec/`).

---

## 3. Comparison & Uncertainties

| Dimension | Solution A: `Backlog` Status Tier | Solution B: `issues/backlog/` Dir | Solution C: Focus Metadata + Cap |
|---|---|---|---|
| **Context cost of open list** | ~20 lines (`status:open` = active only) | ~20 lines (`issues/*.md` only) | ~20 lines (`status:active` default) |
| **README.md size** | ~30 lines (active only; rest in `archive/README.md`) | ~30 lines (active only) | ~250 lines (if open-only) or ~840 |
| **Git rename / link churn** | Low (only once on `archive`) | High (every `defer`/`open`/`close`) | Low (only on `close` ↔ `open`) |
| **Other harnez repos impact** | Non-breaking (`Backlog` is opt-in) | Medium (new subdir in `Scan`/`Lint`) | Non-breaking |

### Uncertainties to Resolve with User
1. **`issues/README.md` scope**: Should `issues/README.md` index *only* active tickets (shrinking it from 843 lines to ~30 lines and cutting git commit diff sizes on every status change), or must it remain a complete 840-row master inventory for human browsing on GitHub/Codeberg?
2. **Automatic move to `issues/archive/` on `close`**: Currently 543 of 562 closed tickets sit in `issues/` because `harnez issues close` rewrites `**Status**:` in place without moving the file. Should `close` auto-move to `issues/archive/` (and `open` auto-restore to `issues/`)?
3. **Hard vs. soft duplicate gate in `harnez issues new`**: Should `harnez issues new "<title>"` hard-fail (exit non-zero, requiring `--force` or `harnez issues open <existing>`) when a high-score title/body match exists in `Backlog`/`Closed`, or only print matches to stderr while still reserving?
4. **Hard cap enforcement**: Should `harnez status` / `harnez issues lint` emit a warning (or error) when active `Open` tickets exceed a threshold (e.g. 30 or 100 in `spec/`), or is periodic `/roadmap` grooming sufficient?
