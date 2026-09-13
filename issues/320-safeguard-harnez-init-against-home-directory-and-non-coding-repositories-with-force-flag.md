# 320 — Safeguard harnez init against home directory and non-coding repositories with --force flag

**Status**: Closed — implemented target directory safeguards and --force flag for init
**Priority**: P1 (High)
**Severity**: High
**Category**: Bug

---

## 1. Problem & Motivation

Running `harnez init` inside the user's home directory (`$HOME` or `~`) or a non-coding directory causes catastrophic accidental side effects:
1. It overwrites/creates `~/AGENTS.md` and `~/CLAUDE.md`, breaking the global harness symlink setup established by `harnez apply` (`~/AGENTS.md` -> `~/.claude/CLAUDE.md`).
2. It initiates `reconcileGoWorkspace` which runs `filepath.WalkDir` over the target directory. When run in `$HOME`, this crawls the entire user filesystem, traverses cache directories, dotfiles, and system mounts, hitting permission errors (e.g. `scan Go modules in /home/...: permission denied`) and failing with a cryptic Go error after already writing files.
3. It creates scaffolding in arbitrary non-coding folders (e.g. `~/Downloads`, `~/Documents`) when run accidentally without arguments.

While `harnez init --all` previously contained a safety check refusing `$HOME` (issue 068), single-project `harnez init` lacked equivalent safeguards.

## 2. Technical Specification / Design

1. **Target Directory Validation**:
   - Refuse running `harnez init` directly on the user's home directory (`$HOME` / `~`) or root filesystem directory (`/`), unless `--force` (`-f`) is passed.
   - Refuse running `harnez init` on a non-coding / non-project directory unless `--force` (`-f`) is passed.
   - A directory is recognized as an eligible project / coding repository if at least one of the following signals is present:
     - Git repository marker (`.git` exists or inside git work tree via `git rev-parse --is-inside-work-tree`)
     - Build or project manifests: `go.mod`, `go.work`, `go.work.example`, `Cargo.toml`, `package.json`, `pnpm-workspace.yaml`, `Makefile`, `GNUmakefile`, `makefile`, `justfile`, `Taskfile.yml`, `CMakeLists.txt`, `meson.build`, `build.zig`, `pyproject.toml`, `setup.py`, `requirements.txt`, `Pipfile`, `pom.xml`, `build.gradle`, `Gemfile`, `Containerfile`, `Dockerfile`, `docker-compose.yml`, `compose.yaml`
     - Existing agent configurations: `AGENTS.md`, `CLAUDE.md`, `issues/`, `spec/`
     - Source files in root or standard source subdirectories (`src/`, `cmd/`, `internal/`, `lib/`, `pkg/`, `scripts/`, `app/`): files ending in `.go`, `.rs`, `.py`, `.c`, `.cpp`, `.cc`, `.h`, `.hpp`, `.zig`, `.ts`, `.js`, `.sh`, `.java`, `.rb`, `.php`, `.swift`, `.kt`, `.scala`, `.lua`, `.nim`, `.hs`.
2. **CLI `--force` (`-f`) flag**:
   - Add `--force` / `-f` to `harnez init` allowing explicit overrides when a user intentionally wishes to initialize an empty or home directory.
3. **Resilient Go Module Scan**:
   - In `internal/claude/gowork.go:findGoModules`, ignore all hidden dot-directories (`.` prefix, e.g. `.cache`, `.config`, `.git`) and `testdata`.
   - If `WalkDir` encounters an unreadable directory (`walkErr != nil`), skip the directory (`return filepath.SkipDir`) instead of aborting the walk, preventing hard failure on permission-restricted subfolders.
4. **Documentation**:
   - Update `docs/CLIDesign.md` and CLI documentation to reflect `--force` / `-f` and the directory validation rules.

## 3. Implementation & Verification Plan

1. Implement `isEligibleProjectDir` and `validateInitTargetDir(dir string, force bool) error` in `internal/claude/init.go`.
2. Update `RunInitWithForce` and wire `--force` flag in `cmd/harnez/main.go`.
3. Enhance `findGoModules` in `internal/claude/gowork.go` to ignore dotdirs and gracefully skip unreadable subdirectories.
4. Add comprehensive unit and integration tests in `internal/claude/init_test.go` and `cmd/harnez/`.
5. Update `docs/CLIDesign.md`.
6. Run smoke tests and verify clean behavior when simulating home directory and empty directory initialization.

## 4. Acceptance Criteria

- [x] `harnez init` fails with a clear message when run in `$HOME` without `--force`.
- [x] `harnez init` fails with a clear message when run in root `/` without `--force`.
- [x] `harnez init` fails with a clear message when run in an empty non-coding directory without `--force`.
- [x] `harnez init --force` succeeds in home, root, or empty non-coding directories.
- [x] `harnez init` succeeds without `--force` in valid Git repositories or directories with project/source manifests.
- [x] `findGoModules` does not hard-fail on unreadable or permission-restricted subdirectories.
