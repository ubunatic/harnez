# Lean sprints: cost matrix, Terra removal, apply env (2026-10-05/06)

Host (Claude Opus) ran three lean sprints with `codex:sol` developers.
Tickets closed: 711 (per-tier cost matrix), 715 (remove Terra), 717 (apply env idempotency).
Filed: 715, 717 (and business 014, parked).

## What worked
- **Host plausibility checks on numbers.** Both chart misreads in 711 (Opus-low taken from
  Sonnet-high's dot, Astra-high taken from the xhigh dot) were caught by the host cropping
  and zooming the chart, not by the worker. The method is now in
  [ModelResearch.md](../ModelResearch.md) step 3.
- **Approval table before writing.** 711 showed the proposed cost changes as a table for
  the user to approve before any spec edit; scope cuts ("low/med/high only", "data review,
  no new measurements") landed in the ticket before the next turn.
- **Reproduction first in 717.** The failing idempotency test pinned the root cause (full
  env replacement plus the jev step re-adding a key) before the fix; the host's real-machine
  check (two runs, `cmp`, mtime) confirmed it. See [CLIDesign.md](../CLIDesign.md).

## What cost extra turns
- **711, 4 resume turns.** The worker skipped unlabeled chart points until told to estimate
  from the log axis; the scope narrowed twice mid-sprint (tiers, no measurements); the
  user's API key appeared mid-sprint.
- **715 crash.** Codex `apply_patch` with several operations on one file, plus stale context
  in `docs/Models.md`, failed and ended the turn (exit 1). Resuming with "one file per
  patch, re-read right before patching" finished it.
- **715 ran `harnez init` out of scope.** It created a new root `docs/ModelRoles.md` and an
  AGENTS.md entry; both were reverted. The handoff said "sync the root copy" without saying
  there was none.
- **Unrated delete.** `harnez agent delete` refuses an unrated session; rate first.
- **Host wording error.** A host prompt stated a user decision the user never made ("decided
  not to buy a paid plan"). Prompts should quote the user, not paraphrase intent.

## Pitfalls
- Shell expansion in close reasons: `harnez issues close 711 "... $0.0045 ..."` turned `$0`
  into `bash`. Use single quotes for reasons that contain `$`.
- Other sessions committed in the same repo meanwhile (`docs/AgenticLoop.md`); every handoff
  said "stage only your own files by path", and no foreign file was committed.
