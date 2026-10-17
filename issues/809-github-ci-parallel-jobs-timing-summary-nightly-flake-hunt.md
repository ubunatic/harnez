# GitHub CI: parallel jobs, timing summary, nightly flake hunt

Status: Open — owned by the Claude cloud agent (ccr branch); placeholder reserved by host

Scope: .github/ only — shared .github/gotest-report.go (go test -json summary: failures, slowest packages, wall time, flake mode), parallel lint/unit/race jobs, nightly + dispatch flake workflow (-race -count=5) filing GitHub issues.
