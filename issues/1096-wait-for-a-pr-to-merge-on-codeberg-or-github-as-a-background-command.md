# 1096 — Wait for a PR to merge on Codeberg or GitHub as a background command

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1054, 1055

---

/goal An agent can run one harnez command as a background job that exits when a given PR on Codeberg (Forgejo) or GitHub is merged or closed, so the job's exit notifies it and nobody has to say "merged". Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

In team mode the host opens an AGit PR and then waits for the owner to merge it. Today the owner has to say "merged" in chat, and the agent cannot tell a merged PR from an open one without asking. On 2026-11-02 the owner said "78 merged" while Codeberg still showed PR 78 open, so the agent had to check by hand with curl. A wait command the agent starts in the background (Claude Code: Bash `run_in_background`) would wake it on the real merge.

## 2. Technical Specification / Findings

- Shape (open for the developer plan): `harnez pr wait <number|url> [--remote origin] [--timeout ...]`. It works out the forge and repository from the git remote URL, polls, and exits 0 on merged, with a distinct non-zero code for closed-unmerged, timeout or an API error. It prints one line naming the PR, state and merge commit.
- Forgejo/Codeberg: `GET /api/v1/repos/<owner>/<repo>/pulls/<n>` gives `state` and `merged` (used by hand in this session; no token needed for public repos). `tea`/`fj` may serve as an optional transport.
- GitHub: `gh pr view <n> -R <owner>/<repo> --json state,mergedAt`, or the REST API.
- Poll interval, timeout and API base paths are configurable values: they go in `spec/` with a schema (`docs/Spec.md`), not as Go defaults.
- Never print a token, including in errors.
- After a merge the agent still runs `git fetch` and checks `git merge-base --is-ancestor <sha> origin/main`; the command may offer that check (`--contains <sha>`).

## 3. Implementation & Verification Plan

- Unit tests with a fake HTTP server for both forges: merged, closed unmerged, still open until timeout, 404 and a transient error.
- Live check: wait on an already-merged Codeberg PR (for example 78) and get exit 0 at once.
- Document the background use in the sprint and `/issue` skills' publish steps and in `harnez pr wait --help`.
