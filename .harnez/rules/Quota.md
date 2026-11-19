# Quota-1 Guardrails

- **Single-Test Boundary**: Every test execution, including targeted `go test` and coverage runs, counts toward the one-run allowance and must use the quota wrapper. Further runs require an intervening source modification; under lean-sprint §3, after fixing a failure from that run, the developer may rerun only touched packages and must report the full run owed to the host.
- **Code Modification Required**: If tests fail or complete, you MUST modify repository source files before running tests again. Repeated test runs without intermediate code modifications are blocked.
- **Clean Tree First**: Before the quota run, check `git status` for changes that are not yours. If there are any, wait or report; do not run.
- **Enforced Test Target**: Use `make test-q1` when available; otherwise use `harnez exec --quota-1 -- <repo-test-command>`. Pass `-coverprofile` through that target instead of running coverage as a separate test run.
- **Unauthorized Bypass Forbidden**: Bypassing guardrails via `QUOTA_BYPASS=1` or `HARNEZ_QUOTA_BYPASS=1` is strictly reserved for human developers and CI environments. Agent loops must not set or pass bypass flags.
- **Report Untested Edits**: If you edit code after your single test run, say in your report that the committed code is untested after that run and name the changed files, so the host runs the suite.
- **Media & Demo Verification Gate**: Get explicit user confirmation before publishing recordings or screenshots.
- **Owner Suspension**: Only the user can suspend these rules for a session, with `/disable` (see Owner Overrides in Tools.md).
