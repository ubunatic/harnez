# 964 — Add harnez-selftest-extended skill for quick read-only smoke checks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #665 (base self-test), #666 (HTO verification), #953 (interactive host role), #760 (self-test bench task)

---

## 1. Problem & Motivation
The base `/harnez-selftest` verifies native background execution and command progression, but not basic read/search/error handling or actual model-agent dispatch. The owner requested an extended skill that gives quick answers without becoming a full integration test.

**Goal:** Add `/harnez-selftest-extended` with bounded smoke checks, clear pass/fail/skip results and batch feedback ratings, while preserving repository state; verify the skill and supported installation paths, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker or no progress, clear the goal and record attempts and remaining work.

## 2. Technical Specification / Findings
### Default quick checks

- Run the existing background self-test by following its returned instructions, preserving `HTO=0` and cleanup guarantees. Do not duplicate or guess its sequence.
- Read the first lines of `AGENTS.md`, e.g. `harnez read -L 1:10 -n AGENTS.md`. Skip clearly if absent, or use an isolated fixture for the read mechanism.
- Run bounded documentation queries such as `harnez find docs -k 3 'install'` and `'test'`; use repository-appropriate known content or a fixture, since those terms are common rather than universal.
- Search code for a known repository symbol; do not assume every repository has `main`. Use the default/configured finder, not an assumed `--via rg` finder name.
- Parse supported `--json` output and verify its expected structure, not merely exit status.
- Probe a deliberately missing fixture path: verify a useful error and nonzero exit, recording it as an expected negative test.
- When configured, run read-only index consistency and tracker checks: `harnez index --check`, `harnez find issues next --json`. Assert allocation preview does not reserve a ticket. Tracker lint may be an optional diagnostic, with unsupported legacy statuses reported rather than silently repaired.
- Record successful and failed command batches with `harnez rate`; distinguish expected negative probes, real failures and unavailable optional features.

### Optional actual agent-dispatch check

Launch one minimal read-only reviewer through the supported host background runner, verify output and termination, and clean up the specific worker. Make this optional and time-bounded because it consumes model inference; disclose the selected model and actual provider. The existing timer-based background self-test alone does not prove reviewer spawning works. Keep genuine leaf-worker restrictions intact and never override roles to turn a rejection into a pass.

### Bounds and state

Set explicit per-check/overall bounds with a quick default path; report actual timings. No builds, installs, full suites, repository policy edits, releases or publishing during normal skill execution. Isolate fixtures in temporary scratch space and remove them; any deliberate telemetry/self-test state writes should be identified. Do not claim the entire flow is filesystem read-only when `harnez rate` and self-test state necessarily persist records.

## 3. Implementation & Verification Plan
Implement at the canonical Harnez skill source and verify supported generated/installed copies. Exercise the quick path in a representative configured repo and a minimal repo without tracker/AGENTS assumptions. Verify valid JSON, expected-error classification, optional skips, time bounds and fixture cleanup, with repository tracked/staged changes unchanged before/after.

Keep read/search/state-machine tests local and deterministic. Opt-in agent-dispatch verification must report separately whether it actually ran and passed, rather than inheriting the base self-test's PASS. Report independent check results so one missing optional capability does not hide successful diagnostics or an actual failure.

Session evidence: the base self-test initially failed at `hello` under the advisor guard, then later completed its 30-second background flow successfully. Bounded read/docs/code searches, allocation preview and index consistency passed; `--via rg` was an invalid finder name, and Cati tracker lint rejected a legacy Archived status. These are useful distinct failure/skip cases for the extended report, not permission to change the target repository.
