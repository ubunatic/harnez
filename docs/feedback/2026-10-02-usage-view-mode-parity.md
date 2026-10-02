# Usage View-Mode Parity Retrospective (Issue 678)

## Outcome

Issue 678 made `normal`, `compact`, and `minimal` spec-defined presentation modes for
`harnez usage`. The mode defaults to `normal`; `--normal`, `--compact`, and `--minimal` select
presentation, while `--watch` independently enables repeated rendering and interaction. The mode
schema, loader, CLI, static renderer, watch renderer, and ANSI mockup now describe the same feature.

## What worked

- **The user clarified the behavioral axis.** The key correction was that PTY detection must not
  choose a different view. Treating presentation mode and watch lifecycle as independent inputs
  gave the CLI and renderer a simple contract.
- **The spec owns presentation policy.** Defining panels and chrome in `spec/usage.yaml` avoids
  separate Go defaults drifting from what users configure or read in the spec.
- **Independent review found real boundary gaps.** The first review found parser/schema mismatch
  and missing default/constrained parity assertions. After those fixes, a follow-up found that a
  footer competing with an overflow hint still differed at three and four rows. Sharing the
  footer-fit threshold and testing those heights closed the gap.
- **One quota run was enough.** After the review fixes, `HTO=0 make test-q1` passed; `make install`
  updated the local binary and man pages. The final `sol:med` review found no actionable issues.

## What I would do differently

- Add a parity matrix to the first implementation pass: default plus every explicit mode, each in
  one-shot and watch, with both roomy and threshold terminal sizes. The first test only compared
  a roomy compact view, leaving default and clipping behavior uncovered until review.
- Define the minimum-height footer rule alongside the mode schema and renderer interface, rather
  than deriving the static reserve separately after tests reveal a mismatch.
- Probe repository Make targets before invoking a presumed validation target. `make validate-spec`
  does not exist; parser and schema checks are covered by the quota test target. The failed target
  was recorded with Harnez feedback tooling and did not affect source changes.
- Use the configured model spelling from `harnez agent models` when dispatching a review. The
  attempted `codex:gpt-6.1-sol:medium` name was unsupported; the accepted `codex:sol:med` review
  completed successfully. This was a dispatch syntax error, not a review or code failure.

## Documentation and harness follow-up

The repo already had useful usage architecture docs, especially `docs/UsageCollection.md`, but its
view section did not specify that mode selection is independent of watch or how static frames
reserve live footer rows. That reusable rule is now documented there, and issue 678 records the
implementation and verification. No harness change is needed for this task. If this pattern recurs,
consider a small reusable parity-test helper for commands with one-shot and watch renderers; keep
that as a proposal until another command needs it.

The separate oversized-file extraction work remains tracked by issue 679 and was not part of this
ticket's implementation.
