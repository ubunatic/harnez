# 1235 — Use goself v0.3.0 for harnez install binary deployment, shell PATH, completions, and man pages

**Status**: Closed — integrated ubunatic.com/goself v0.3.0 into harnez install and internal/claude shell setup (removing dev/shellsetup); harnez install now deploys ~/.local/bin/harnez, symlinks ⚙, configures bash/zsh/fish PATH and completions, installs man pages, and restarts active harnez-agent-collector.service while preserving dependency installs (harnez install neus); HTO=0 make lint, HTO=0 make test-q1, and live go install + harnez install verified
**Closed-By**: session d3745c98-8c17-42c0-ab28-f6f0104b79c2
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [897](897-add-neus-as-optional-dependency-with-harnez-install-verb-and-server-indexer.md), [989](989-make-install-leaves-a-stale-telemetry-collector-running-new-record-kinds-wait-in-the-spool.md), [995](995-statusline-fails-with-text-file-busy-while-the-harnez-binary-is-being-replaced.md)

---

## 1. Problem & Motivation

Running `go install ubunatic.com/harnez/cmd/harnez@latest` (or `go install ./cmd/harnez`) followed by `harnez install` fails because `harnez install` currently only accepts a dependency name (`neus`) from `spec/dependencies.yaml`. It does not self-install or update the `harnez` binary, ensure the install directory is on `PATH`, install shell completions, link `⚙`, or install man pages. Meanwhile, `harnez` still carries an older pre-extracted `dev/shellsetup` copy instead of depending on `ubunatic.com/goself` v0.3.0.

## 2. Technical Specification / Findings

- Add `ubunatic.com/goself v0.3.0` to `go.mod`.
- Replace `dev/shellsetup` usages in `internal/claude/apply_shell_setup.go` (and `apply_shell_setup_test.go`) with `goself.NewShellSetup` / `goself.ShellConfig` so `harnez apply`, `diff`, `status`, `revert`, and `harnez install` share the upstream v0.3.0 shell setup implementation (including stray-marker and empty-rc safety fixes), and remove `dev/shellsetup/`.
- Extend `harnez install [host | dependency]`:
  - With a known dependency from `spec/dependencies.yaml` (e.g., `neus`), keep running `dependency.Install`.
  - With 0 args (local) or a remote `[host]` arg, deploy `harnez` atomically via `goself.New(...).Deploy(...)` (`--strategy auto|self|build|release|go`, `--dir ~/.local/bin`, `--version`, `--allow-dirty`, `--direct`, `--dry-run`, matching `.goreleaser.yaml` archive/checksum naming for `release`).
  - On local install, wire `PATH` and bash/zsh/fish shell completions via `goself.NewShellSetup` (unless `--no-shell`), symlink `⚙` alongside the installed binary, install roff man pages (unless `--no-man`), and update/restart `harnez-agent-collector.service` when installed and active.
- Update `docs/CLIDesign.md` and `cmd/harnez/testdata/help.golden`.

## 3. Implementation & Verification Plan

1. Add `ubunatic.com/goself v0.3.0`, migrate `internal/claude/apply_shell_setup.go` to `goself`, and remove `dev/shellsetup`.
2. Add `internal/selfinstall` and update `cmd/harnez/install.go` (+ tests and `help.golden`).
3. Verify with `HTO=0 make lint`, `HTO=0 make test-q1`, and live `go install ./cmd/harnez && harnez install` checks in an isolated `HOME` and on the local workspace.
