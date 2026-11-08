# 1179 — edit guard flags append-only writes and edits inside scratch worktrees

**Status**: Closed — Edit guard no longer warns on >> appends or inside linked worktrees (.git file into /worktrees/; linkedWorktree in internal/exec/exec.go); submodules and main-tree rewrites still warn. New cases in TestScriptedRepoWriteConservative and TestExecHookEditGuardM2Cases fail without fix. make test-q1 exit 0. Jules PR 144 partly used (append), closed.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 937, 1119; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

/goal The Bash edit guard stays silent for append-only writes and for writes outside the session's main working tree, and still warns on in-place rewrites of tracked files, locked by tests; or stop and report when blocked on an owner decision.

## Problem

In the 2026-11-06 Jules cleanup session the guard (`scriptedRepoWrite`, `internal/exec/exec.go:1312`,
called at `:1232`; text in `spec/edit_guard.yaml`) fired on nearly every ticket update and escalated to
"Prove it was better with concrete token counts". Most hits were not the risky case 937 targets:

- **Append-only writes:** `printf '| row |' >> issues/<n>-*.md` to add a tracking-table row. An append
  cannot clobber existing content.
- **Scratch worktrees:** `sed -i` inside a throwaway `git worktree` under the session scratchpad, used
  for a mutation test (rename a JSON tag, run the test, discard the worktree). The guard resolved the
  repo root from the command's `cd`, not from where the main working tree lives.
- Single exact-line replacements verified by `grep -c`/`assert` were also flagged; those stay a judgement
  call and are out of scope here.

Noise trains agents to ignore the warning, which weakens it for the real case.

## Want

- No warning for `>>` appends to a file (any repo).
- No warning when the written path is outside the session's main working tree (another worktree,
  scratchpad, `/tmp`).
- Keep the warning for `sed -i`, `perl -i` and `>` rewrites of tracked files in the main tree.

## Done when

- Tests in `internal/exec` cover an append, a write inside a secondary worktree, and an in-place rewrite
  in the main tree (still warns).
