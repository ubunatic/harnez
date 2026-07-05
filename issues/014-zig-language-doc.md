# 014 — Bundle a Zig language doc

**Status:** Open

## Context

The doc registry covers golang, bash, make, rust, cpp, markdown, git, gtk4,
canary, spec — but not Zig. emojig (Zig app) maintains its own
`docs/Zig.md` outside the managed set, and the books/ project has
`src/lang/zig.md` ("Zig — The Good Parts"). Auto-detection for Zig is trivial
(`build.zig` present).

## Proposal

- Adopt emojig's `docs/Zig.md` as `docs/lang/Zig.md` (reconcile with the books
  chapter so the two don't drift), add a `zig:` entry with
  `default: auto` + `build.zig` detection in `detectDoc`.
- Add title/weight frontmatter like the other lang docs (weight 68).
