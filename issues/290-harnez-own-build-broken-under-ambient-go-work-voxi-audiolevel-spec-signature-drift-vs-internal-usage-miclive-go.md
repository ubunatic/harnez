# 290 — harnez's own build is broken under the ambient go.work: voxi's `audiolevel.Spec` signature drifted from `internal/usage/miclive.go`

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [issues/287](287-go-work-ambient-workspace-file-breaks-unrelated-tooling-in-sibling-repos-general-fix-needed.md), [issues/284](284-harnez-release-build-step-defaults-to-gowork-off-with-allow-workspace-override.md) (this is the intentional harnez+voxi pairing breaking, not an unrelated sibling), `internal/usage/miclive.go:90-94`, `~/projects/voxi/audiolevel/audiolevel.go:425,484`

---

## 1. Problem & Motivation

While independently reviewing b1e131a/ac5df82 (issue 287's `harnez init`
go.work isolation fix), verification of `go test ./internal/claude/...`
under the ambient `~/projects/go.work` (the normal harnez+voxi co-dev
configuration issues 284/287 explicitly preserve) failed at the build
step — not in the package under review, but in `internal/usage`:

```
$ go env GOWORK
/home/uwe/projects/go.work
$ go build ./...
# ubunatic.com/harnez/internal/usage
internal/usage/miclive.go:94:130: cannot use spec (variable of type func() (audiolevel.Metric, "time".Duration, "time".Duration))
    as audiolevel.Spec value in argument to audiolevel.StartManager
```

Root cause: `internal/usage/miclive.go` (harnez commit 14f0aed,
2026-09-08T09:30, "refactor(usage): extract live mic-meter capture into
voxi's audiolevel") builds a 3-return-value closure
`func() (audiolevel.Metric, time.Duration, time.Duration)` and passes it
as `audiolevel.Spec`. Voxi's `audiolevel.Spec` type
(`~/projects/voxi/audiolevel/audiolevel.go:425`) was changed to a
**4**-return-value shape,
`func() (metric Metric, window, attack, decay time.Duration)`, by voxi
commit 7b76138 ("fix(audiolevel): ease rises as well as falls, no more
instant-attack jumps"), committed 2026-09-08T13:46 — a few hours *after*
harnez's dependent commit.

`GOWORK=off go build ./...` succeeds (harnez's `go.mod`-pinned voxi
version is presumably still compatible); only the ambient-workspace build
picks up voxi's live, uncommitted-relative-to-harnez's-pin source and
breaks. Since `make check`/`make test`/`test`
(`Makefile:78,81,86`) run plain `go test ./...` with no `GOWORK=off`,
**any normal `make check` run in harnez right now fails** for anyone
with `~/projects/go.work` active — which issues 284/287 both establish
as the expected default dev configuration for this exact pairing.

This is a live, currently-reproducible break in harnez's primary
dev-loop command, discovered as a side effect of this review, not a
hypothetical.

## 2. Technical Specification / Findings

- `go env GOWORK` → `/home/uwe/projects/go.work`; `go build ./...`
  fails as shown above.
- `GOWORK=off go build ./...` → succeeds cleanly.
- Cause is a cross-repo API signature drift between two independently
  committed changes ~4 hours apart on 2026-09-08, with no CI or local
  gate that runs the ambient-workspace build to catch it.
- This is distinct from issue 287 (which is about *unrelated* sibling
  repos breaking) and issue 284 (which only hardened `harnez release`'s
  own build step) — this is the *intentional* harnez+voxi workspace
  pairing itself going stale, which neither prior ticket's fix
  addresses.

## 3. Implementation & Verification Plan

- Update `internal/usage/miclive.go`'s `spec` closure to voxi's current
  4-value `Spec` shape (add the `attack` duration voxi's fix
  introduced), and decide harnez-side what attack value to pass (likely
  wiring in an attack duration analogous to `WindowDuration`/
  `DecayDuration`, per whatever `watchMicLiveSpec()` exposes or a new
  field on it).
- Re-run `go build ./...` and `go test ./...` under the ambient
  `~/projects/go.work` (not `GOWORK=off`) to confirm the fix, since that
  is the config that actually caught this.
- Consider (separately, may warrant its own follow-up rather than being
  in scope here) whether harnez's `go.mod` should pin an explicit voxi
  version/replace directive so ambient-workspace builds can only drift
  as far as a deliberate version bump, instead of always tracking voxi's
  live working tree.

## 4. Acceptance Criteria

- [ ] `go build ./...` and `go test ./...` pass under the ambient
      `~/projects/go.work` (no `GOWORK=off`), not just with it off.
- [ ] `internal/usage/miclive.go`'s `Spec` closure matches voxi's
      current `audiolevel.Spec` signature.
- [ ] Record whether a pinned/`replace`d voxi version is adopted to
      prevent recurrence, or why it's deliberately left floating.
