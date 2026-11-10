## Verdict
done, because the stale embedded doc check was successfully moved before `ValidateCopyableDocCatalog` and all related tests were updated to pass.
## Changes
internal/claude/init.go
internal/claude/init_doc_preflight.go
internal/claude/init_doc_preflight_test.go
## Tests
`go test -v ./internal/claude -run TestCheckStaleHarnezSource`
=== RUN   TestCheckStaleHarnezSource
=== RUN   TestCheckStaleHarnezSource/project_without_sources
=== RUN   TestCheckStaleHarnezSource/stale_checkout_source
--- PASS: TestCheckStaleHarnezSource (0.01s)
    --- PASS: TestCheckStaleHarnezSource/project_without_sources (0.00s)
    --- PASS: TestCheckStaleHarnezSource/stale_checkout_source (0.00s)
PASS
ok  	ubunatic.com/harnez/internal/claude	0.034s

Failing test before changes:
`go test -v ./internal/claude -run TestInitDocPreflightStaleSourceOnlyInHarnezCheckout`
=== RUN   TestInitDocPreflightStaleSourceOnlyInHarnezCheckout
=== RUN   TestInitDocPreflightStaleSourceOnlyInHarnezCheckout/project_without_sources
=== RUN   TestInitDocPreflightStaleSourceOnlyInHarnezCheckout/stale_checkout_source
--- FAIL: TestInitDocPreflightStaleSourceOnlyInHarnezCheckout (0.00s)
    --- PASS: TestInitDocPreflightStaleSourceOnlyInHarnezCheckout/project_without_sources (0.00s)
    --- FAIL: TestInitDocPreflightStaleSourceOnlyInHarnezCheckout/stale_checkout_source (0.00s)
        init_doc_preflight_test.go:94: expected actionable stale-source error, got copyable doc "git" source "docs/lang/Git.md" assumes project destination "docs/"; make it optional and state a fallback
FAIL
FAIL	ubunatic.com/harnez/internal/claude	0.015s
FAIL
