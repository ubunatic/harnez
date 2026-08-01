# 016 — `make smoke` convention: live-run targets in Make.md

**Status:** Open

## Context

The bundled `docs/Make.md` standardizes help/build/test targets, but has no
convention for *live verification* — briefly running the real binary against
real inputs. In proctop's initial build session (2026-08-01), `go test`
stayed green while two real bugs existed that only a live run caught:

- name-target resolution picked a childless Firefox `--dbus-service` stub
  over the real browser tree,
- `term.GetSize` returned 0×0 without an error on a fresh pty, silently
  dropping all TUI graph rows.

"Tests green" is weak evidence for tools that read live system state
(/proc, terminals, sockets). proctop documented its pty harness in its own
`docs/Testing.md` + `issues/007`, but the convention belongs in the managed
Make doc so every project gets it.

## Proposal

- Add a `smoke` section to `docs/Make.md`: a `smoke: ⚙️` target that runs
  the built binary for a few seconds against a real target and asserts
  basic output (grep for expected lines / exit code).
- Recommended shape: `smoke` depends on `build`, uses `timeout`, redirects
  to a file before asserting (pipes + timeout produce misleading empty
  output), and for TUIs drives a pty (Python `pty` harness — `script(1)`
  is not installed on this host).
- Convention note for agents: run `make smoke` (when present) before
  declaring a feature working; it complements `make check`, not replaces it.
- Optionally cross-ref from `docs/Canary.md` (probe-before-build) since the
  spirit is the same: verify against reality, not assumptions.

## Related

- proctop `docs/Testing.md` (pty harness, pitfalls) and proctop issue 007
  (make smoke target there)
- [010-smoke-test-agent-visibility.md](010-smoke-test-agent-visibility.md) —
  different "smoke" (claudeconfig's own install check), naming overlap only
