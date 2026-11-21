# 1245 — make install writes ~/go/bin/harnez while ~/.local/bin/harnez (goself) shadows it on PATH

**Status**: In Progress
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

## 4. Occurrence: taia background tasks run the stale binary (2026-11-21)

`make install` refreshed `~/go/bin/harnez` (01:26), which the foreground Bash shell finds first.
A taia background task runs `bash -lc`, where `command -v harnez` is `~/.local/bin/harnez` (built
2026-11-19, before the 2026-11-20 model ladders). A reviewer started there resolved
`--role reviewer --model lean` to `hydrogen:low` with "terra:med: unsupported", while
`harnez agent models --picks` from `~/go/bin` gives the shipped chain
`terra:low, luna:high, neon:low, haiku:high`. So agents in background tasks silently use an old
spec and old code until `~/.local/bin` is updated.

## 5. Resolution (2026-11-21)

Owner question "do we use goself properly" found three gaps against goself's own guide
(`skills/goself/SKILL.md` in goself v0.3.0); all three are fixed here:

- One install location. `make install` now runs `./harnez install --strategy self`, so goself
  writes `~/.local/bin/harnez`, the `⚙` symlink, shell PATH, completions and man pages, and
  restarts an active collector. It no longer writes `GOPATH/bin`. `self` copies the
  Makefile build, which keeps the `+<commit count>` version; `auto` would rebuild without it.
- Static build. `make build` uses `CGO_ENABLED=0` like GoReleaser, so the `self` strategy never
  copies a glibc-linked binary to a musl host (goself trap 1). Only `net`, `os/user` and
  `runtime/cgo` used cgo, and all three have pure-Go fallbacks.
- `go.mod` listed goself as `// indirect`; `go mod tidy` makes it (and `golang.org/x/sys`) direct.
- `harnez install` warns when PATH finds a different harnez first (for example a stale
  `~/go/bin/harnez` from the old recipe) and says which directory to put first.

Unchanged: runner mode still refuses `harnez install` (issue 1181, A4), so `make install` inside
a taia session fails with the stage-and-promote hint. `make promote` still writes `GOBIN` or
`GOPATH/bin`; aligning the promoter root with `~/.local/bin` is a follow-up if staged promotion
should also land first on PATH.
