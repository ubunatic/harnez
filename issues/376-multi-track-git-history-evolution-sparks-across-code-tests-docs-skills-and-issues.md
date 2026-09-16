# 376 — Multi-track git history evolution sparks across code, tests, docs, skills, and issues

**Status**: Open  
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [189](189-git-history-doc-token-time-series-telemetry.md), [190](190-multi-doc-stacked-token-history-canary.md), [192](192-integrate-git-doc-history-cli-command.md), [377](377-project-level-token-attribution-and-generative-cost-of-change-metrics.md), [378](378-fleet-wide-multi-repo-git-history-sparks-and-token-attribution-matrix-for-uman.md)  

---

## 1. Summary & Motivation

`harnez dochistory` currently extracts commit-by-commit token and byte evolutions for individual files and directories (primarily focusing on markdown in `docs/` and `issues/`).

However, developer repositories evolve across multiple distinct functional dimensions simultaneously:
1. **Code**: Source files (`*.go`, `*.rs`, `*.py`, `*.c`, `*.sh`, `*.ts`).
2. **Tests**: Test suites (`*_test.go`, `test_*.py`, `tests/**`).
3. **Docs**: Narrative and reference documentation (`docs/**/*.md`, `README.md`, `CLAUDE.md`, `AGENTS.md`).
4. **Skills**: Agent skill definitions (`skills/**/SKILL.md`, `commands/*.md`).
5. **Issues**: Ticket tracking files (`issues/*.md`, active vs closed velocity).

This ticket tracks extending the high-performance Git plumbing engine (`internal/assess/dochistory.go`) to categorize git tree blobs into parallel tracks across history and output unified multi-track evolution sparklines.

---

## 2. Proposed Design & CLI UX

### Multi-Track Inspection (`harnez repo-history` / `harnez dochistory --tracks`)

```text
Repo Evolution (harnez · 840 commits · 100d)
Code:    [ ▂▃▄▅▆▇█] 18.4k LOC (84.2k tokens)
Tests:   [  ▂▃▄▆▇█] 12.1k LOC (56.0k tokens) · 0.66 test/code ratio
Docs:    [ ▂▃▅▆▇██] 48.5k tokens (112 files)
Skills:  [   ▂▃▅██] 8 skills (14.2k tokens)
Issues:  [  ▂▄▆▇█▅] 375 tickets (38 open · 337 closed)
```

### Architecture
1. **Track Classification Rules**:
   - `Code`: Matches recognized language extensions excluding test files.
   - `Tests`: Matches test filename conventions across Go, Python, Rust, TS.
   - `Docs`: Markdown files excluding tickets.
   - `Skills`: Agent skill declarations in `skills/` or `commands/`.
   - `Issues`: Ticket files in `issues/` (differentiating active from closed via status markers).
2. **Plumbing Sampling**:
   - Sample commit snapshots across Git history (e.g. exponential, weekly, or commit-stride buckets) to keep execution under 100ms.
   - Compute parallel 8-step Unicode sparklines per track.

---

## 3. Acceptance Criteria

- [ ] `internal/assess/` provides track-based classification for git tree blobs (`code`, `tests`, `docs`, `skills`, `issues`).
- [ ] `harnez dochistory --tracks` (or `harnez repo-history`) computes and displays multi-track sparklines and current volume.
- [ ] Test/code ratio is computed and highlighted in terminal output.
- [ ] `--json` mode returns structured timeseries arrays for each track.
- [ ] Unit tests in `internal/assess/` and `cmd/harnez/` verify classification accuracy and execution performance (<100ms).
