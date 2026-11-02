# 1064 — Add harnez issues import command to fetch remote forge issues into local tracker

**Status**: Closed — harnez issues import <url> reads a Codeberg/Forgejo or GitHub issue (read-only, spec/pr.yaml forges + tokens), writes an Open ticket linking it, quotes the body as data, refuses PRs and repeats (18e55411); live import of codeberg forgejo#1 OK; go test ./... green. Export (1067) stays open.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1065, 1066, 817 (GH ticket ids), 321 (forge auth), docs/IssueTracking.md

---

## 1. Problem & Motivation

Contributors and external users frequently file issues directly on remote Git forges (such as Codeberg/Forgejo and GitHub). Currently, bringing a remote forge issue into the local Harnez tracker (`issues/*.md`) requires manual copying, reformatting metadata headers, running `harnez issues new`, and pasting the body.

A native `harnez issues import <url|number>` command will automate this pipeline: fetching the remote issue via REST API, extracting metadata, generating the compliant local ticket skeleton, and synchronizing `issues/README.md`.

## 2. Technical Specification

### CLI Syntax
```bash
harnez issues import <URL | ID> [flags]
```
- `<URL>`: Full URL to remote issue, e.g. `https://codeberg.org/ubunatic/harnez/issues/40` or `https://github.com/ubunatic/harnez/issues/42`.
- `<ID>`: Positional issue number, resolving against the primary repository remote (`origin` or `github`).
- `--dir, -d <repo>`: Target repository directory (default `.`).
- `--priority <P0-P3>`: Override default priority (default inferred or `P2`).
- `--open`: Automatically set status to Open (default true).

### API Resolution & Formatting
1. **Forge Detection**: Inspects remote URL domain:
   - `codeberg.org` or Forgejo instances: calls `/api/v1/repos/{owner}/{repo}/issues/{index}`.
   - `github.com`: calls `/repos/{owner}/{repo}/issues/{index}` via GitHub API or `gh api`.
2. **Authentication**: Uses public unauthenticated API endpoints for public repositories; falls back to environment tokens (`FORGEJO_TOKEN`, `CODEBERG_TOKEN`, `GITHUB_TOKEN`, or `gh` auth) when present.
3. **Template Transformation**:
   - Title: Extracted from remote issue title.
   - Related: Links remote issue URL, author username, and creation date.
   - Body: Transcribes markdown body, prepending remote author attribution.
4. **Tracker Registration**: Invokes internal ticket reservation (`allocateIssueNumber`), writes markdown, and executes index refresh.

## 3. Goal & Acceptance Plan

/goal Implement `harnez issues import` to fetch remote Codeberg/Forgejo and GitHub issues and generate local tracker files, verifying with automated tests and live imports of Codeberg #40 and #41, or stop and report if API access is blocked.

### Milestones
* **M1 — API Client & Formatter**: Implement forge REST fetcher for Codeberg/Forgejo and GitHub in `internal/issues/import.go`.
* **M2 — CLI Command**: Expose `harnez issues import` in `cmd/harnez/issues.go` with `--dir` and URL parsing.
* **M3 — Verification & E2E Tests**: Test against mock forge API servers and live public issues.
