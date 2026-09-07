# 280 — Make generated issue index and ticket numbering rebase-safe

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: Issue 269 (`harnez issues mv`), Issue 279 (persistent index lock
sidecar), commit `08ce3dc` (post-implementation rebase recovery audit),
`issues/README.md`, `.gitattributes`, Git merge drivers and rewrite hooks

---

## 1. Problem & Motivation

Independent clones can allocate the same issue numbers and discover the collision only
when their histories converge. A recent pull replayed local tickets 270 and 271 onto a
fetched branch that already owned 270 through 276. Git stopped on conflicts involving
the generated `issues/README.md`; the local tickets ultimately had to become 277 and
278.

Issue 269 added number-only `harnez issues mv`, but its audit in `08ce3dc` established
that `rebase --abort`, two ordinary move commits, and another rebase is not sufficient:
Git replays each original ticket-add commit before its later corrective rename commit,
so the collision occurs before the rename can help. Operators should not have to
rewrite historical commits or manually merge a generated index merely to reconcile
independently allocated ticket numbers.

`issues/README.md` is entirely generated from canonical ticket files. Its competing
Git versions contain no unique state worth line-merging. Harnez knows how to scan the
combined ticket set, detect duplicate numbers, select destinations above the combined
maximum, rewrite ticket headers, and regenerate the index. Integrate that knowledge
with Git so a generated-index conflict alone does not stop a rebase, while preserving
real conflicts and making every history mutation explicit and recoverable.

## 2. Desired Behavior and Design Investigation

Design and implement a supported Git integration with these properties:

1. Mark `issues/README.md` as a generated file with a repository-installed custom
   merge driver (likely via `.gitattributes`). On merge or rebase, the driver treats
   the three index versions as disposable and resolves the path successfully so a
   README-only conflict does not stop replay. The final index is regenerated from the
   ticket files rather than semantically merging table rows or conflict markers.
2. After replay, identify duplicate ticket numbers introduced by the locally replayed
   commits. Use Git's rebase/rewrite metadata or `post-rewrite` old/new commit mapping
   to distinguish local tickets from the fetched canonical tickets; do not guess from
   lexical path order, timestamps, or whichever file is encountered first.
3. Assign each colliding local ticket a deterministic number above the maximum across
   the combined ticket set, preserving stable ordering when multiple tickets collide.
   Rename its file, rewrite its header, and regenerate `issues/README.md` from scratch.
   The real 270/271 versus fetched 270–276 case must deterministically yield 277/278.
4. Preserve genuine conflicts. A same-path/same-ticket content conflict, ambiguous
   ownership, malformed ticket metadata, unsupported Git state, or failed repair must
   stop with a specific diagnostic and a recovery command; the generated-index rule
   must not broadly mark unrelated content as resolved.

Evaluate the safest lifecycle rather than assuming a hook can complete every step.
A `post-rewrite` hook may repair and stage deterministic changes, but silently amending
or creating commits after a successful rebase is surprising and potentially fragile.
Document and test an explicit safety decision among:

- a hook that repairs/stages and reports the required final commit;
- a hook that validates and directs the user to a dedicated repair command; or
- an atomic `harnez pull --rebase` (or narrowly named equivalent) wrapper that owns
  the complete fetch/replay/renumber/regenerate/commit transaction and recovery path.

The implementation may combine these, but must clearly define when the rebase is
considered successful, where the 270→277 mapping persists during replay, who commits
the repair, and what happens if the process is interrupted at each boundary. It must
not run `git rebase --continue`, amend commits, create commits, or discard user changes
implicitly unless that behavior is part of an explicit wrapper contract.

## 3. Installation, Validation, and Recovery Contracts

- Install/configure the merge driver, attributes, and hook through the appropriate
  project-local `harnez init` path. Do not add project-local behavior to `apply`; read
  `docs/CLIDesign.md` before changing either command. Installation must be idempotent,
  preserve unrelated user Git configuration/hooks, and work with Git worktrees.
- Avoid depending on one clone's globally configured driver without a clear bootstrap
  check. If a repository declares the attribute but the driver is unavailable, fail
  early with an actionable setup command rather than producing a misleading merge.
- Treat ticket files as the sole canonical index input. Regeneration must remove all
  conflict markers and stale A/B rows and be idempotent on a second run.
- Add pre-commit and/or pre-push/index lint gates that reject duplicate numbers,
  mismatched filename/header numbers, and index drift. Gates are defense in depth;
  they do not replace rebase repair and must not sweep unrelated working-tree files.
- Coordinate with the existing index locking design and issue 279. Concurrent index,
  issue, hook, and wrapper processes must serialize safely, never allocate the same
  destination, and either complete atomically or leave an unambiguous recoverable
  state.
- Persist any temporary renumber mapping under Git state rather than the tracked
  worktree. Define cleanup on success, abort, retry, checkout, and interrupted
  execution. Re-running repair after partial completion must be safe and idempotent.
- Preserve unrelated staged and unstaged user changes. Stage only paths owned by the
  repair, and report every rename and generated-file update.
- Support a dry-run/diagnostic mode that prints detected ownership, proposed mappings,
  affected paths, and the next Git/user action without changing files or Git state.

## 4. Acceptance Criteria

- [ ] A rebase whose only ordinary Git conflict is generated `issues/README.md` does
      not stop for manual line-level index resolution.
- [ ] The generated index is rebuilt solely from canonical ticket files and contains
      neither stale rows nor Git conflict markers after repair.
- [ ] A test reproducing local 270/271 replayed onto fetched 270–276 finishes with the
      local tickets at 277/278, correct in-file headers, a consistent index, and no
      duplicate-number diagnostics.
- [ ] Ownership and allocation are deterministic across repeated runs and are derived
      from combined history/rewrite information, not only the current local maximum.
- [ ] Real same-ticket content conflicts and ambiguous ownership still stop safely
      with actionable recovery guidance.
- [ ] The chosen hook/wrapper contract explicitly tests commit/staging behavior and
      never silently amends or creates history outside that documented contract.
- [ ] Install/update/remove flows for attributes, merge-driver configuration, and
      hooks are idempotent and preserve unrelated configuration and hook content.
- [ ] Pre-commit/pre-push or equivalent validation rejects duplicate ticket numbers,
      filename/header mismatch, and index drift.
- [ ] Concurrent and interrupted operations cannot overwrite tickets, allocate the
      same destination, lose mappings, or strand an unrecoverable rebase.
- [ ] Repair is idempotent; a retry after success or partial failure produces no
      additional renumbers or index changes.
- [ ] Unrelated working-tree and index changes remain untouched.

## 5. Verification Plan

- Build integration fixtures with two independent clones/branches, not only an
  artificial directory containing duplicate files. Replay the exact 270/271 versus
  270–276 topology and assert commit graph, ownership selection, 277/278 allocation,
  generated index, staged paths, and final `harnez status` output.
- Cover README-only conflict, a genuine same-ticket conflict, multiple collisions,
  gaps and already-high numbers, malformed headers, missing driver configuration,
  detached HEAD, merge and apply rebase backends where supported, linked worktrees,
  dirty/staged unrelated files, and absent/non-Git repositories.
- Inject failures after mapping creation, ticket rename, header rewrite, index
  regeneration, staging, and any final commit step. Verify documented retry/abort
  recovery and no data loss.
- Run concurrent repair/index/issue operations to verify lock and number-allocation
  behavior, including two processes proposing the same next number.
- Verify installation twice, upgrade from an existing managed repository, and removal
  or disablement without damaging user hooks/configuration.
- Run focused Go tests, `go test ./...`, `make install`, `scripts/smoke-test.sh`,
  `harnez index --check`, and `harnez status` before closure.

## 6. Non-goals

- Preventing independent clones from ever reserving the same number; without shared
  coordination, reconciliation remains necessary.
- Automatically resolving substantive edits to the same ticket.
- Treating hand-edited `issues/README.md` content as canonical or preserving it across
  regeneration.
- Pushing rewritten or repaired history automatically.
