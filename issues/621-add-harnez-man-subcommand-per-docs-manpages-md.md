# 621 — Add harnez man subcommand per docs/ManPages.md

**Status**: Closed — delivered in 591e4b85: harnez man, man --install, make man/install/install-system
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Usability / Documentation
**Related**: #620

---

/goal Harnez follows its own `docs/ManPages.md`: `harnez man` prints roff, `harnez man --install`
installs the man tree, `make install` installs the pages; stop and report when blocked on a user
decision or denied permission.

## 1. Problem & Motivation

Harnez ships `docs/ManPages.md` (the man-page practice for Go CLIs) but does not follow it: there is
no `harnez man` command, no `cobra/doc` dependency, and `make install` installs only the binary.
Found while closing #620, which could not update a man page.

## 2. Technical Specification / Findings

- Implement per `docs/ManPages.md` §3–§6 (`cobra/doc` `GenMan`/`GenManTree`, XDG path, `--dir`).
- Harnez has persistent flags (`-d/--dir` on several subtrees): check shorthand collisions (§5);
  `man` may need a different long flag than `--dir` if it collides.
- `go mod tidy` after adding `cobra/doc`.

## 3. Implementation & Verification Plan

- Add `cmd/harnez/man.go` with a test that generates the tree into a temp dir and finds
  `harnez-agent-start.1` containing `--interactive`.
- Wire `./harnez man --install` into `make install`; ignore `/*.1`.
- Verify `man harnez-agent-start` after `make install`.
