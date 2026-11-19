# 1245 — make install writes ~/go/bin/harnez while ~/.local/bin/harnez (goself) shadows it on PATH

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1235, 1181, docs/studies/2026-11-19-session-isolation-forge-parity-and-quota-pack-story.md

---

## 1. Problem & Motivation

Since issue 1235 (`goself v0.3.0`), `harnez install` installs `~/.local/bin/harnez` (`internal/selfinstall/install.go`, `DefaultDir = "~/.local/bin"`), and `~/.local/bin` precedes `~/go/bin` on `PATH` (`which -a harnez` resolves `/home/alfred/.local/bin/harnez` before `/home/alfred/go/bin/harnez`). However, `Makefile`'s `install` target still runs `go install ... ./cmd/harnez` into `~/go/bin` (`GOPATH/bin`) and leaves `~/.local/bin/harnez` untouched. As a result, running `make install` updates `./harnez` and `~/go/bin/harnez`, while plain `harnez` on `PATH` silently keeps invoking the older `~/.local/bin/harnez` binary across sessions and sprint checks.

## 2. Technical Specification / Findings

- `Makefile` `install` writes `$(go env GOPATH)/bin/harnez` (`~/go/bin/harnez`) and `⚙ -> harnez` there, installs man pages via `./$(BINARY) man --install`, and restarts `harnez-agent-collector.service`.
- Observed locally on `2026-11-19` after `make install`: `/home/alfred/go/bin/harnez` (`39,491,601` bytes, `harnez version 0.5.9+5387`) differed from `/home/alfred/.local/bin/harnez` (`39,409,134` bytes, `harnez version 0.5.9`), and plain `harnez` resolved to `~/.local/bin/harnez`.
- Note issue 1181 / M14–M15 constraints: `internal/selfinstall` blocks `harnez install` inside Landlock runner mode, while `make install` runs on the host and already builds `./$(BINARY)`. Any alignment between `Makefile` `install`, `make promote`, and `harnez install` must preserve runner-mode confinement and `GOBIN`/`GOPATH/bin` compatibility for users without `~/.local/bin` setup.

## 3. Implementation & Verification Plan

- Decide whether `Makefile`'s `install` target should keep installing into `GOPATH/bin` and also sync/update `~/.local/bin/harnez` when `~/.local/bin` precedes `GOPATH/bin` on `PATH` (e.g. via `./$(BINARY) install --direct` or Makefile promotion), warn loudly when a shadowing binary exists on `PATH`, or unify promoter/Makefile default roots.
- Verify on host that running `make install` makes `harnez --version` on `PATH` match `./harnez --version`, without breaking runner-mode staging (`scripts/stage-install-test.sh`) or `make lint`.
