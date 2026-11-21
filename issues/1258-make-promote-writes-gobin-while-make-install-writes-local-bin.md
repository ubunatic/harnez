# 1258 — make promote writes GOBIN while make install writes ~/.local/bin

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Infrastructure
**Related**: 1181, 1245

---

## 1. Problem & Motivation

Since 1245, `make install` runs `harnez install --strategy self`, which writes `~/.local/bin`.
The host promoter (`make promote STAGE=<dir>`, `harnez install --staged`) still writes `GOBIN` or
`GOPATH/bin` (`internal/selfinstall/promote.go`, `ResolvePromoterRoots`, decision A4 in 1181).
On a PATH with `~/go/bin` first, a promoted build would shadow the goself install again, which is
the 1245 drift in reverse.

## 2. Technical Specification / Findings

- Owner host PATH (2026-11-21) lists `~/go/bin` before `~/.local/bin`.
- A4 names `GOBIN`/`GOPATH/bin` as the only approved binary roots; changing them is an owner
  decision about the 1181 trust boundary.

## 3. Implementation & Verification Plan

- Owner decides: add `~/.local/bin` (goself `DefaultDir`) as the promoter root, or keep A4 and
  have `--staged` print the same PATH-shadow warning as `harnez install`.
- Verify with `scripts/stage-install-test.sh` and the promoter tests on the host.
