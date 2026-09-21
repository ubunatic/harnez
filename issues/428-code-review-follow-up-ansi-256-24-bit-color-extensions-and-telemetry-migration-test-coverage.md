# 428 — Code review follow-up: ANSI 256/24-bit color extensions and telemetry migration test coverage

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement

---

## Context

During the review of the recent multimodal reading card and telemetry schema migration changes, several follow-up enhancements and edge-case testing opportunities were identified:

1. **`internal/readcard` ANSI SGR Expansion**:
   - The basic 3/4-bit ANSI colors (codes 30-37, 40-47, 90-97, 100-107) and standard reset/bold attributes are now parsed and rendered.
   - Investigate extended color sequence support (256-color palette via `\x1b[38;5;Nm` / `\x1b[48;5;Nm` and 24-bit truecolor via `\x1b[38;2;R;G;Bm` / `\x1b[48;2;R;G;Bm`) for rich CLI terminal rendering (e.g. diffs or stylized outputs passed into `harnez read -I`).

2. **Telemetry Migration Coverage**:
   - Verify that sequential migration steps (e.g. databases created at schema v1..v4 leaping directly to v9) undergo end-to-end integration tests with legacy fixtures.
   - Ensure `harnez apply` gracefully logs migrations in all verbosity modes.

## Proposed Actions

- [ ] Add support or explicit fallback handling for 256-color and 24-bit RGB ANSI sequences in `internal/readcard/ansi.go`.
- [ ] Add unit test fixtures for multi-version telemetry DB upgrades (e.g. v2 -> v9 and v5 -> v9).
- [ ] Verify test suite passing via `make test-q1`.

## Acceptance Criteria

- Extended ANSI SGR codes are handled safely without dropping or breaking text layout in rendered PNG cards.
- Multi-step telemetry schema migrations are covered by regression tests.
- All tests pass under Quota-1 rules (`make test-q1`).

## Sprint Status (lean-sprint, 2026-09-21)

Telemetry migration coverage is delivered (`235d7da`): authentic v2 and v5 upgrade fixtures with
column assertions, plus apply logging tests. Remaining: ANSI 256/24-bit color support.
