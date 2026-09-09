# Independent review of b1e131a/ac5df82 (go.work isolation, issue 287) — Claude Sonnet 5

## Scope examined
- `internal/claude/gowork.go`, `internal/claude/gowork_test.go`, the `RunInitWithIssuesGit`
  wiring in `internal/claude/init.go`, `docs/lang/Go.md`'s new "Workspace Isolation" section,
  and issue 287's acceptance criteria/closure text against the actual diff.
- Live verification: re-ran the unit suite, and independently confirmed the ticket's live-canary
  claim by finding the real artifact it left behind (`~/projects/trafficsim/go.work`, commit
  `d0efb69`, `use ./scripts`) rather than trusting the ticket's prose alone.

## Findings
- **Filed issue 289**: `findGoModules`'s directory walk only skips `.git/.hg/.svn/node_modules/vendor`,
  not `testdata` or `_`/`.`-prefixed dirs. A `go.mod` fixture under `testdata/` (a common pattern for
  Go-tooling repos) gets treated as a project module; if it's intentionally malformed, `go work init`
  hard-fails and that error aborts the *entire* `harnez init` run, not just the workspace step.
  Reproduced live with a synthetic malformed `testdata/brokenmod/go.mod`.
- **Filed issue 290**: unrelated to the reviewed diff's logic, but found while verifying it —
  `go build ./...` under the ambient `~/projects/go.work` (the exact config issues 284/287 both
  preserve as the intended default) currently fails: voxi's `audiolevel.Spec` gained a 4th return
  value in voxi commit `7b76138`, a few hours after harnez's `internal/usage/miclive.go` (commit
  `14f0aed`) started depending on the 3-value shape. `make check`/`make test` run plain
  `go test ./...` with no `GOWORK=off`, so this currently breaks harnez's own primary dev loop for
  anyone with the ambient workspace active — the exact class of hazard issue 287 is about, except
  hitting the *intentional* co-dev pairing rather than an unrelated sibling.
- No defects found in the core `planGoWorkspace`/`reconcileGoWorkspace` decision logic itself: the
  seven policy branches (existing local file, no module, no parent workspace, `GOWORK` explicit,
  non-enclosing workspace, already-included, omitted module) are each covered by a focused test,
  and `go work edit -json` is confirmed read-only (no edit flags), so the membership probe cannot
  mutate the ambient parent workspace. `runGoCommandWithoutWorkspace` correctly forces `GOWORK=off`
  for the actual `go work init` write.
- Issue 287's closure text (§3.2) accurately reflects the diff; no unverified or exaggerated claims
  found. The `dry-run` half of §3.1's "dry-run or explainable decision path" phrasing was resolved
  as explainable-only (printed reason per branch, no flag to preview without writing) — acceptable
  since the criterion was an "or" and issue 4's checklist doesn't separately require a dry-run flag.

## Verification performed
- `GOWORK=off go build ./...`, `GOWORK=off go vet ./internal/claude/...`,
  `GOWORK=off go test ./internal/claude/...` — all pass.
- `go build ./...` under the ambient workspace (no override) — fails, reproducing issue 290 live.
- Manual repro of issue 289's testdata-fixture hazard against a throwaway `/tmp` module tree.
- Confirmed issue 287's live-canary claim by inspecting `~/projects/trafficsim/go.work` and its
  git history, not just re-reading the ticket's own account of it.

## Effectiveness of this review call
- Worth repeating for infra-shaped tickets (build tooling, cross-repo mechanisms): the two real
  findings both came from *running* the code under the same ambient conditions the feature exists
  to manage, not from reading the diff. A diff-only read would have missed both.
- The independent-artifact check (trafficsim's `go.work`) is a cheap, high-value habit for any
  ticket that claims a "live canary" — the claim is falsifiable in under a minute if the artifact
  doesn't exist or doesn't match.
- No duplicate-ticket risk encountered; `harnez find issues "gowork"` cleanly surfaced 284/287 with
  nothing to collide with before filing 289/290.
