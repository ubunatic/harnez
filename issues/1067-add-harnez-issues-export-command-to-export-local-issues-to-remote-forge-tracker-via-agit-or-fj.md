# 1067 — Add harnez issues export command to export local issues to remote forge tracker via AGit or fj

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1195 (forge create), 1196 (forge dupes), 1197 (prefilled link), 1064 (issues import), 1062 (AGit ergonomics), 1054 (push rules per repo), 321 (fj auth login), docs/IssueTracking.md

---

## 1. Problem & Motivation

While Harnez maintains a local, offline-first issue tracker in `issues/*.md`, teams collaborating across remote hosting platforms (Codeberg/Forgejo and GitHub) frequently need to publish or synchronize locally authored tickets to the repository's remote forge tracker.

Currently, this requires manual copy-pasting of markdown bodies, title, and metadata to the web interface. A dedicated `harnez issues export` command will automate this bridge by reading local issue files, authenticating via existing forge credentials (`fj`, `gh`, or API tokens), creating the remote ticket, and recording the remote URL back into the local ticket's `Related:` header.

## 2. Technical Specification

### CLI Syntax
```bash
harnez issues export <TICKET-NUMBER | --all | --active> [flags]
```
- `<TICKET-NUMBER>`: Specific local issue number (e.g. `1065`).
- `--remote <name>`: Target Git remote (default `origin`).
- `--backend <auto|fj|gh|api|agit>`: Mechanism to use:
  - `auto`: Inferred from target remote host (`codeberg.org` -> Forgejo; `github.com` -> GitHub).
  - `fj`: Use Forgejo CLI (`fj issue create`) or direct REST API (`/api/v1/repos/{owner}/{repo}/issues`).
  - `gh`: Use GitHub CLI (`gh issue create`) or direct REST API (`/repos/{owner}/{repo}/issues`).
  - `agit`: AGit-based creation where supported.
- `--dry-run`: Preview title, labels, and formatted body without creating remote issues.
- `--update-local`: Automatically write the returned remote issue URL into the local ticket's `Related:` header and commit.

### Data Mapping
1. **Title**: Parsed from local `# <n> — <Title>`.
2. **Labels**: Inferred from `Category` (e.g. `bug`, `feature`, `docs`) and `Priority` (`P1`, `P2`).
3. **Body**: Extracts content, prepending a reference to the local Harnez ticket ID.
4. **Header Sync**: After successful creation, updates the local ticket:
   ```markdown
   **Related**: https://codeberg.org/ubunatic/harnez/issues/45, ...
   ```

### Update 2026-11-09 (after 1195, 1197)

- **No `agit` backend**: AGit push options open and update pull requests only; there is no AGit path that
  creates an issue. Drop `--backend agit`.
- **Reuse 1195**: creation goes through `forgetracker.PlanCreate` and the `issue_cli` list in `spec/pr.yaml`
  (tea, fj, gh) instead of new adapters; `--backend` reduces to picking a CLI from that list. Creating needs a
  write login; the read-only `CODEBERG_TOKEN` is enough only for the duplicate check (1196) before export.
- **Hybrid flow this enables**: author tickets as `issues/*.md` and publish them by AGit PR with only git
  credentials; export them to the forge later from a clone where a CLI is logged in. With no login, fall back to
  the prefilled new-issue link from 1197.
- Skip tickets already exported (remote URL in `Related:`) and gate on forge duplicates (1196).

## 3. Goal & Acceptance Plan

/goal Implement `harnez issues export` to publish local issue markdown files to remote forge trackers via Forgejo (fj/API) and GitHub (gh/API), updating local Related headers with the remote URL, or stop and report when blocked on API credentials or remote permissions.

### Milestones
* **M1 — Remote Issue Exporters**: Implement Forgejo REST/CLI and GitHub REST/CLI export adapters in `internal/issues/export.go`.
* **M2 — CLI Command & Local Sync**: Add `harnez issues export` subcommand in `cmd/harnez/issues.go`, supporting `--dry-run` and `--update-local`.
* **M3 — Verification & E2E Tests**: Unit tests with mock forge servers and CLI dry-run verification.
