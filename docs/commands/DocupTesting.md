# Docup Testing

Maintain the repository's testing guide at `docs/Testing.md`.

Inspect these sources first, stopping at the Docup evidence cap:

1. `AGENTS.md` and the repository's applicable local instructions.
2. `docs/Testing.md` and `docs/other/Canary.md`.
3. `Makefile` targets for checks, tests, lint, smoke, install, and canaries.
4. Representative `*_test.go` files and `scripts/smoke-test.sh` or relevant
   `scripts/canary-*` entry points.
5. At most 10 relevant recent commits when history is needed to confirm drift.

Compare the guide with the repository's actual verification layers: package
tests, integration-style tests, static checks, smoke tests, canaries/live
checks, and manual or visual verification. Check commands, paths, and links
against their definitions. Look for concrete omissions or stale instructions;
do not turn this into a general repository audit.

If the evidence shows drift, make only the smallest documentation edits needed
to `docs/Testing.md` and related documentation links. Make no more than 3
documentation edits in one invocation. If the guide is accurate, make no edits
and report a no-op.

Verify changed links and formatting, and run the narrowest relevant checks. Do
not run live canaries merely to review documentation unless the documentation
claims depend on a mechanism that cannot be checked from its definition. Report
checks run separately from checks only inspected.
