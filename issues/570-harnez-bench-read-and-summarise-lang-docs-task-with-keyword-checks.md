# 570 — harnez bench: read-and-summarise lang docs task with keyword checks

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling
**Related**: [[569-harnez-bench-run-stderr-preamble-and-live-progress-table-on-stdout]]

## Background

The user ran "read six lang docs and summarise as a table" by hand with codex luna:med in three
read modes. The six sessions are in `docs/data/codex-bench-read-lang-docs-*-luna-med-pass-*.md`
(Git LFS). This becomes a bench task so it runs via `harnez bench run --read native,text,card`.

## M1 — task and all-keywords check

- New task `read-lang-summary` in `internal/bench/tasks.yaml`. Fixtures: `docs/lang/` Go.md,
  Make.md, ManPages.md, Bash.md, Git.md, Markdown.md. Prompt, per read mode:
  - native: "Read only Go.md Make.md ManPages.md Bash.md Git.md Markdown.md in docs/lang and
    summarise as a table. (Use your native read tool for reading text files)"
  - text: same, ending "(Use 'harnez read <file>' for reading text files)"
  - card: same, ending "(Use 'harnez read -I <file>' for reading text files as PNG cards)"

  Reuse the existing `read_modes` mechanism if it can carry these sentences for this task;
  otherwise add the smallest spec field that can. No hard-coded prompt text in Go (docs/Spec.md).
- New check: a list of required keywords that must **all** appear (case-insensitive, each may be
  an RE2 alternative), e.g. `require_all:` in yaml. Score fails and names the missing keywords.
  - Response must contain a Markdown table (a `|---` separator row).
  - One keyword per doc name: `Go`, `Make`, `ManPages|man pages`, `Bash`, `Git`, `Markdown`
    (word-bounded so "Go" does not match inside other words where possible).
  - One key point per doc, taken from the doc itself, e.g. `PascalCase` for Markdown.md. Choose
    points that a correct summary is very likely to mention.
- Validation test: score the final answer of each of the six `docs/data` sessions against the task;
  all six must pass. If a file is still a Git LFS pointer (no `git lfs pull`), `t.Skip` with that
  reason. If a chosen keyword fails on a real session, pick a better one rather than dropping it.
- Unit tests for the new check (all present, one missing, no table). Update `docs/Bench.md`.
- No live model calls. One `make test-q1`, commit `feat(bench): ... (issue 570 M1)`.
