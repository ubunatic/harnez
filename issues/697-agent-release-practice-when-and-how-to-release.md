# 697 — Agent release practice: when and how to release

**Status:** Open
**Priority:** P2 (Medium)
**Severity:** Moderate
**Category:** Docs / Agent Practice
**Related:** [091 — release spec](091-language-agnostic-release-spec-and-thin-make-release.md), [097 — skip release without diff](097-release-skip-when-no-diff-since-prev-tag.md), [246 — commit and publish skills](246-add-commit-and-publish-skills-for-staged-commit-ownership-and-multi-project-release-publish-workflows.md), [GoRelease](../docs/GoRelease.md)

---

## 1. Problem & Motivation

`docs/GoRelease.md` covers setup and mechanics (version.yaml, GoReleaser, minisign, Forgejo). It does not teach an agent **when** a release is safe or **how** to cut one from a tree that is mid-work. Agents get "quick release" requests in the middle of sprints, with half-finished commits and other sessions' uncommitted edits in the same worktree.

**Goal:** a short, rule-style section (GoRelease.md §4 or a harnez rule) that an agent can follow in one pass.

## 2. Case study — cati v0.2.9 (2026-10-03)

Context: lean sprint on cati issue 076 (spec ownership), milestones M1–M5 committed and reviewed. M6 was running when the user asked "commit current code" and then "I need a quick release".

What happened, in order:

1. **Committed in separate, owned commits.** The other developer's docs and issue edits became one commit, and the stopped M6 developer's partial work became a `wip(...)` commit with "full tests not run" in the body. Mixing them would have made the later revert impossible.
2. **Ran the full suite before releasing**, not just vet. `go build`/`go vet` were green on the WIP, but `go test ./...` caught a startup break: a new strict YAML decoder (`KnownFields(true)`) rejected `$schema:` in `spec/style.yaml`. A vet-only gate would have shipped a browser that fails at startup.
3. **Parked the WIP with `git revert`, not by rewriting history.** Revert, then re-run `go vet`, `go test ./...`, and `make preflight` (all green), then `harnez release -b patch`. `harnez release` tags and pushes HEAD, so HEAD itself had to be releasable.
4. **Restored the WIP right after** with a revert of the revert, recorded the break as pre-work in the sprint ticket, and resumed the developer.

Result: v0.2.9 was published with signed linux/darwin artifacts, and no broken code was shipped. The sprint resumed within minutes.

Friction observed:

- No written rule said "full tests, not vet, gate a release", or "release only from a HEAD you have tested".
- `harnez release` has no `--ref`/commit option, so the revert/re-revert dance was the only way to release while WIP sat on top.
- `harnez release` output does not repeat the pre-release check results, so the agent must remember to run them itself.

## 3. Proposed rules (to land in docs)

**When to release**

- Only on an explicit user request, or as the documented last step of a workflow the user started (e.g. `/publish`). It pushes and publishes, so it is outward-facing.
- Only from a HEAD where the full test suite, vet, and the project preflight passed **on that exact commit**.
- Not with uncommitted changes from other sessions in the release scope. Commit them separately with their owner's consent, or leave them uncommitted (the release only takes committed state).
- Skip when nothing changed since the last tag (097).

**How to release from a mid-work tree**

1. Commit each owner's changes separately, and mark partial work as `wip(...)` with its test status in the body.
2. Run the full gate: `go vet ./...`, the full test suite (output to a file, grep for failures), and `make preflight`.
3. If the gate fails on WIP: `git revert` the WIP commit, re-run the gate, release, then revert the revert. Never `reset`, rebase, or force-push shared history for a release.
4. `harnez release -b patch` (or as asked), and check the result: tag pushed, assets listed, "completed successfully".
5. Report the version and what it contains (milestones, known theoretical bugs), and record any break found in the owning ticket.

## 4. Candidate tooling (decide separately)

- `harnez release --ref <commit>` to release a tested commit below WIP without reverts.
- A built-in pre-release gate: run the configured test command (or refuse with a hint) before tagging.
- Print a one-line "gate: vet ok, tests ok, preflight ok" summary in the release output.

## 5. Acceptance

- GoRelease.md (or a harnez rule) has a "When and how to release" section covering §3, kept short.
- The case study stays here as the evidence, not in the evergreen doc.
- §4 items are either filed as separate tickets or rejected, with a reason recorded here.
