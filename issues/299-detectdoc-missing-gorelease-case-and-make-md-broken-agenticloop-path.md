# 299 — detectDoc missing gorelease case and Make.md broken AgenticLoop path

**Status**: Closed — resolved in b5013ab: gorelease auto-detection and consumer-safe AgenticLoop reference fixed; internal/claude tests pass
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `internal/claude/init.go`, `config.yaml`, `docs/lang/Make.md`, `docs/lang/Go.md`, `docs/practices/GoRelease.md`

---

## 1. Problem & Findings

During verification of `harnez init` in a consumer Go repository (`loom`), two doc-bundling issues were identified:

### Issue A: `detectDoc` lacks a case for `gorelease`, creating dead `@docs/GoRelease.md` references in Go projects

1. In `config.yaml` (lines 583–592), `gorelease` is configured with `default: auto`:
   ```yaml
   gorelease:
     name: "Release Pipeline"
     ref: "@docs/GoRelease.md"
     hint: "harnez release, version.yaml spec, GoReleaser v2, non-interactive minisign (-W), Forgejo has_releases, language-agnostic (Go/Python/Zig/Rust/scripted)"
     source: docs/practices/GoRelease.md
     target: ~/.claude/docs/GoRelease.md
     local: ./docs/GoRelease.md
     template: docs/templates/version.yaml
     default: auto   # go.mod (opt-in / auto for Go projects)
   ```
2. In `internal/claude/init.go` (`detectDoc`, lines 307–326), doc detection is implemented as:
   ```go
   func detectDoc(dir, name string) bool {
       switch name {
       case "golang":
           return fileExists(filepath.Join(dir, "go.mod"))
       case "bash":
           return globExists(dir, "*.sh") || globExists(filepath.Join(dir, "scripts"), "*.sh")
       case "make":
           return fileExists(filepath.Join(dir, "Makefile"))
       case "rust":
           return fileExists(filepath.Join(dir, "Cargo.toml"))
       case "zig":
           return fileExists(filepath.Join(dir, "build.zig")) ||
               fileExists(filepath.Join(dir, "build.zig.zon")) ||
               globExists(dir, "*.zig")
       case "cpp":
           return globExists(dir, "*.c") || globExists(dir, "*.cpp") || globExists(dir, "*.cc") ||
               globExists(dir, "*.h") || globExists(dir, "*.hpp") || fileExists(filepath.Join(dir, "CMakeLists.txt"))
       }
       return false
   }
   ```
3. Because `detectDoc` has no `case "gorelease"`, `detectDoc(dir, "gorelease")` unconditionally returns `false`.
4. As a result, running `harnez init` in a Go repository does not copy `docs/practices/GoRelease.md` to `./docs/GoRelease.md`.
5. Simultaneously, `docs/lang/Go.md` (which *is* copied for Go repositories) explicitly references `@docs/GoRelease.md` at line 72:
   ```markdown
   - **Releases**: Provide a thin `release: check ⚙️` recipe that delegates to `harnez release`. See `@docs/GoRelease.md`.
   ```
   This leaves `@docs/GoRelease.md` as a dangling/missing reference in every Go repository initialized by `harnez init` unless `--docs gorelease` is manually passed.

### Issue B: Hardcoded nested path `docs/practices/AgenticLoop.md` in `docs/lang/Make.md`

In `docs/lang/Make.md` line 148:
```markdown
- Every target here queries or mutates a real remote host — treat it like `make smoke` (see
  `docs/practices/AgenticLoop.md`): safe to define, but only run when you intend the live effect.
```
In consumer repositories, `AgenticLoop.md` is installed by `harnez init` to `./docs/AgenticLoop.md` (directly under `./docs/`). `docs/practices/AgenticLoop.md` does not exist in consumer repos. It should reference `@docs/AgenticLoop.md` or `docs/AgenticLoop.md`.

---

## 2. Proposed Remediation

1. In `internal/claude/init.go`: Add `case "gorelease"` to `detectDoc`:
   ```go
   case "gorelease":
       return fileExists(filepath.Join(dir, "go.mod"))
   ```
2. In `docs/lang/Make.md`: Change `see docs/practices/AgenticLoop.md` to `see @docs/AgenticLoop.md` or `see docs/AgenticLoop.md`.
3. Add a unit test in `internal/claude/docs_test.go` verifying that `detectDoc(dir, "gorelease")` returns true when `go.mod` is present.
