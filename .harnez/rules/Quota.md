# Quota-1 Guardrails

- **Recommended Single-Test Boundary**: Run test executions, including targeted `go test` and coverage runs, through the quota wrapper so they are recorded. Upgrading from a narrower tier (`package`/`-run` -> `-short` -> full suite) is the cleanest path. A repeat at the same or lower tier without source changes prints an advisory note and still runs (under lean-sprint §3, after fixing a failure, rerunning only touched packages is fine; report the full run owed to the host).
- **Code Modification Recommended**: After tests fail or complete, modify repository source before running tests again at the same or lower tier. This is advisory: the wrapper reports the repeat and runs it anyway.
- **Clean Tree First**: Before the quota run, check `git status` for changes that are not yours. If there are any, wait or report; do not run.
- **Recommended Test Target**: Use `make test-q1` when available; otherwise use `harnez exec --quota-1 -- <repo-test-command>`. Quota-1 no longer sandboxes runs (no bwrap or Landlock) and never blocks them. Pass `-coverprofile` through that target instead of running coverage as a separate test run.
- **Unauthorized Bypass Forbidden**: Bypassing guardrails via `QUOTA_BYPASS=1` or `HARNEZ_QUOTA_BYPASS=1` is strictly reserved for human developers and CI environments. Agent loops must not set or pass bypass flags.
- **Report Untested Edits**: If you edit code after your single test run, say in your report that the committed code is untested after that run and name the changed files, so the host runs the suite.
- **Media & Demo Verification Gate**: Get explicit user confirmation before publishing recordings or screenshots.
- **Owner Suspension**: Only the user can suspend these rules for a session, with `/disable` (see Owner Overrides in Tools.md).
