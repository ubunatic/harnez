# GitHub CI: parallel jobs, timing summary, nightly flake hunt

Status: Closed — parallel lint/unit/race jobs, gotest-report summaries, nightly flake workflow; green on main (run 39149787911)

Scope: .github/ only.

## Changes
- `.github/gotest-report.go` (stdlib, shared verbatim with loom, cati, voxi):
  reads `go test -json`; writes failing tests with output tail, test time and
  the slowest 10 packages to the job summary; `::error` annotations; `-flaky`
  lists tests that passed and failed across `-count` runs. Replaces
  `scripts/gotest-summary` (removal left to the local dev agent).
- `ci.yaml`: parallel jobs `lint` (`go build`, `make lint`), `unit`
  (`make test-short`) and `race`;
  `report` writes the CI wall time and job results. Each job caches modules and
  its build cache under its own key (race builds differ), saved per commit.
- `flake.yaml`: nightly and `workflow_dispatch` (`count`, default 5) run of
  `go test -race -count`; opens or comments on one `Flaky test: <pkg> <Test>`
  issue (label `flaky`) per intermittent test.

## Verification
- actionlint clean; gotest-report checked on real, flaky and non-JSON input.
- ci: workflow_dispatch run on the ccr branch shows the timing summaries.
- flake: first dispatch after merge (new workflows only dispatch from main).
