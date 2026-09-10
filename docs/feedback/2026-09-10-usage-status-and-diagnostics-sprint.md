# Usage status and diagnostics sprint

- Reusing the existing Astra advisor session worked for this sprint and kept the plan aligned with the earlier #105/#030 audit.
- Stable commits after #105 and the diagnostics view made review and rollback straightforward.
- The first implementation put status markers into every aggregate quota label. Narrow layout tests exposed the alignment cost, so markers moved to agent titles and degraded aggregate rows.
- Independent review caught two state and tracker gaps: empty model groups were still hidden, and `l` did not switch away from the Controls overlay. Both were fixed before completion.
- The bounded in-memory diagnostics view is useful for fetch-stage visibility while leaving richer error capture and scrolling in #255 for a later decision.

Validation completed with `go test -count=1 ./...`, `go vet ./...`, `git diff --check`, and `make install`.
