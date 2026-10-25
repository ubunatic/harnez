# Bug Hunt

Run a focused hunt over active P1/P2 bug tickets, verify each against live code, and fix current bugs through the lean-sprint workflow. The host follows the role contract in `lean-sprint`.

## Invocation

- `/bug-hunt` selects all eligible tickets.
- `/bug-hunt <tickets>` limits the hunt to named tickets.

## 1. Pick

Use `harnez find issues -a status:active`; select Category Bug (or clearly bug-like categories such as Reliability) at P1/P2. Order fresh tickets and blockers first. Pair tickets that share code into one batch.

## 2. Re-verify (read-only)

Have read-only searchers check tickets against live code and recent history. They return one status per ticket: `STILL-CURRENT`, `PARTLY`, or `OBSOLETE`, with evidence, root-cause file and line, way-forward fix, write set, and test. Treat misleading or stale titles as clues, not facts.

Searchers may run in parallel with each other and with the active developer. Keep one writer at a time; the host commits only files it staged.

## 3. Record

Add a dated live-check section to each ticket with the result and evidence. The host commits these ticket updates before implementation. Keep the ticket as the record of the finding.

## 4. Decide

- **Obsolete:** close with the evidence and name any out-of-scope parts in the reason.
- **Not reproducible:** add diagnostics that make the next occurrence actionable, then block as `waiting for a live repro`. Do not close it.
- **Current or partly current:** fix the live problem according to today's design. State which ticket parts are out of scope.

## 5. Fix and review

Run the lean-sprint §2–5 loop for each batch. Keep one developer at a time, one commit per ticket, and one `make test-q1` run per batch. Close a ticket only after its acceptance checks pass.

The host performs the live check itself, such as a real `harnez apply`, live probes, real-flow git checks, and a clean `git status` after `harnez init`. Reopen the ticket with pre-work when acceptance is missed.

## 6. Teardown

Follow lean-sprint §6 for teardown and the study note. Stop and report if work loops or is blocked on a decision; do not check in with the user during the hunt.

Delegate the fix loop to `lean-sprint`, a study to `study`, and a parked follow-up to `issue`. Follow their instructions instead of copying their mechanics here.
