# Testing

Harnez uses several complementary verification layers. Start with the smallest
layer that exercises the change, then broaden checks when the change crosses a
boundary or depends on the real environment.

- **Package tests** — Go unit and component tests live beside the code in
  `*_test.go` files. Run `go test ./...` for the full suite or
  `make check-fast` for the same fast feedback loop.
- **Integration-style tests** — Tests such as
  `internal/claude/integration_test.go`, CLI tests under `cmd/harnez/`, and
  tests using temporary files, HTTP servers, subprocess seams, or PTYs cover
  interactions across package and operating-system boundaries.
- **Static checks** — `make check` runs `go vet ./...` and `go test ./...`
  with `GOWORK=off`; `make lint` checks registered command documentation.
- **Smoke tests** — `make smoke` runs `scripts/smoke-test.sh`, which builds the
  binary and exercises apply, idempotency, drift repair, and selected live
  output behavior.
- **Canaries and live checks** — `scripts/canary-*.sh` and the matching Go
  programs probe external CLIs, network/SSH behavior, microphones, and PTYs.
  Use these when a fake cannot establish that the real mechanism works. The
  canary guidance is in [other/Canary.md](other/Canary.md).
- **Manual or visual verification** — TUI, terminal, media, and desktop
  integration changes may need a real installed binary (`make install`) and a
  live observation such as `harnez usage --watch`. Tests prove invariants;
  they do not prove every terminal or desktop rendering outcome.

Common entry points:

```sh
make check       # vet plus the complete Go suite
make smoke       # build plus live smoke checks
make install     # install the current binary to ~/go/bin
```

For advanced cases, inspect the nearest package tests, the relevant canary, and
the Makefile recipe before adding a new test layer.
