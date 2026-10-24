# 898 — Recommend gokay check before commits in Go repo agent guidance

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 177 (lean post-edit build check); `../gotester/docs/Gokay.md`

---

/goal Add one line to the AGENTS.md guidance harnez manages for Go repos, recommending `gokay check` before commits when `gokay` is installed, or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Agents in Go repos either run repo-wide `gofmt`/`go vet`/`go test ./...` (slow, noisy with legacy
drift) or skip checks. `gokay` (repo `~/projects/gotester`, formerly gotester) checks only the
agent's own change: `gokay check` runs fmt (check only), vet, golangci-lint on changed lines and
tests of changed packages, and prints a four-row summary with a meaningful exit code.

## 2. Specification
- One line in the harnez-managed Go guidance (likely the `docs/Go.md` bundle summary or the Go
  block harnez writes into AGENTS.md; check which one is the source), e.g.:
  "Before committing Go changes, run `gokay check` (add `--base <branch>` without `origin/HEAD`)
  when `gokay` is on PATH."
- It must be conditional on `gokay` being installed; repos without it keep using `make test`.
- Do not duplicate `docs/TestTiers.md`; reference it if the tiers need to mention gokay.

## 3. Acceptance
`harnez init` in a Go repo writes the line once; non-Go repos are unchanged.
