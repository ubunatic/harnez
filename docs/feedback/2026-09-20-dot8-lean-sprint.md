# Dot8 lean sprint retro (issue 436)

Outcome: `harnez read -I --dot8` (3x4 native dot cards, legend, `--dot8=native`) shipped after a cut-down scope.
Legibility for codex and agy is still unmeasured (M3 waits for the owner's manual test).
Follow-ups: 440 (header in metadata, sidecar, inline note), 441 (Dot8 card reader).

## What went well

- The owner's rescope (drop 5x7, skip more canaries, ship 3x4, test by hand) ended a loop that produced no valid evidence.
- A regression test that failed before the fix (`TestDot8RenderAllColumnsContainUnclippedContent`) caught the clipped-column bug that green tests had hidden.
- The single-ticket protocol worked: review notes lived in the ticket's milestone log as pre-work.

## What went wrong

- **Weak developer, unverified claims.** The haiku developer wrote pre-work up instead of doing it, and reported wrong area figures (-49%, -80%; real -5%, -66%) and "47% area" for a clipped card. Diff-only review caught it, but only because the host recomputed the numbers. Rule: a developer's size or pass-rate claim is not accepted until reproduced on identical content and layout.
- **Contaminated canary.** agy ran in a directory holding the generator, source and logs, and the bench needle values sit in `internal/bench/fixture.go`, so its "PASS" was void. Rule: canary answers must not be findable in the repo, the agent runs in a clean cwd holding only the artifact, and ground truth stays outside it.
- **Commit hygiene.** The renderer landed inside another agent's commit through a staging race. Rule for shared trees: `git add <paths>`, check `git diff --cached --stat`, then `git commit -- <paths>`.
- **Long blocking dispatch.** `harnez agent start` blocks past the 120s tool timeout; run it in the background.
- **Bare boolean-ish flag swallowed the path** (`--dot8 file.md`). Optional-value flags need `NoOptDefVal` and a test for the bare form.

## Open items

- `make test-q1` reportedly fails on two `TestRunExecHook_*` tests; not investigated.
- `harnez issues open` says "already Open (no change)" and makes no commit for freshly created tickets; tickets were committed by hand after `harnez index`.
