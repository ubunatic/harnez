# 384 — harnez assess directory validation and -d/--dir flag support

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Usability
**Related**: [[057-repo-assessment-and-code-metrics-command]], [[368-report-ramp-evidence-and-projected-changes-from-harnez-init]]

---

## 1. Problem & Motivation

1. When `harnez assess <path>` or `harnez assess <path> --ramp` is called with a non-existent directory (e.g. typos like `harnez assess ../psyc`), `AssessRAMP` silently ignores the `filepath.WalkDir` error, assumes absent Git metadata, and reports `L1 (Unconfigured)` with a notice instead of returning an actionable error stating the target path does not exist.
2. CLI consistency: commands like `harnez init`, `harnez find`, `harnez issues`, and `harnez index` accept `-d, --dir <path>`. However, `harnez assess` only accepts positional arguments `[path]`, failing with `unknown shorthand flag: 'd' in -d` when callers use the standard `-d` flag.

## 2. Scope & Design

- In `internal/assess/ramp.go` (`AssessRAMPWithProjected`), perform an explicit `os.Stat` on the root target directory and return `os.ErrNotExist` / actionable error if the target directory does not exist or is not a directory.
- In `cmd/harnez/main.go`, add `-d, --dir` flag to `assessCmd` matching other Harnez subcommands, while keeping positional `[path]` backward-compatible.

## 3. Exit Criteria

- [ ] `harnez assess /nonexistent` and `harnez assess /nonexistent --ramp` fail with an actionable error.
- [ ] `harnez assess -d <path>` works identically to `harnez assess <path>`.
- [ ] Unit tests cover missing directory error handling and flag parsing.
