# Worktrees — learnings & TODOs

Observations from running parallel `isolation: "worktree"` agents on a Go/Wails project
(instamoji, 2026-05) — two Haiku agents implementing separate `cmd/` binaries in parallel.

---

## What worked well

- Both agents completed independently without obvious collision.
- `go.mod` was reconciled automatically — both dependency sets (gotk4 + gioui) landed
  in the final go.mod with no manual merge step required.
- The `cmd/instamoji-gtk/` and `cmd/instamoji-gio/` output files appeared in the main
  working tree ready for review.

---

## Problems & TODOs

### TODO: Document go.mod + parallel worktrees risk

Two agents running `go get` concurrently in separate worktrees both modify `go.mod`.
Go modules are module-scoped (one `go.mod` per repo root), so parallel writes are a
race.  In this run it merged cleanly, but this is not guaranteed.

**Mitigation options (evaluate & document in Go.md):**
- Use `go.work` (Go workspaces) to give each `cmd/` its own module — each worktree
  then has an independent `go.mod`, no conflict possible.
- Or: serialize `go get` operations (run them in the parent agent before handing off
  to sub-agents, with dependency scaffolding already in place).
- Or: pre-add framework deps to `go.mod` before spawning agents so they don't need
  to run `go get` at all.

### TODO: Add `make deps-gtk` / `make deps-gio` targets for system libraries

Agents (and humans) couldn't build the new binaries without first installing system
packages.  The current Makefile has no `deps` target and no documentation of which
`apt` packages are required.

**Action:** Add to Makefile:
```makefile
deps-gtk: ⚙️  ## install system deps for instamoji-gtk (Debian/Ubuntu)
	sudo apt-get install -y libgtk-4-dev libgirepository1.0-dev

deps-gio: ⚙️  ## install system deps for instamoji-gio (Debian/Ubuntu)
	sudo apt-get install -y libvulkan-dev libwayland-dev libxkbcommon-dev libgl-dev
```

Document required packages in AGENTS.md §Build so sub-agents can install them before
trying `go build`.

### TODO: Pin framework API version in agent prompts for Haiku

Haiku used a stale Gio API (pre-v0.10 style: `app.NewWindow()`, `w.Events()`,
`system.FrameEvent`, `material.NewTheme(collection)`) that was incompatible with the
version `go get gioui.org@latest` installed (v0.10.0).

**Lessons:**
- Small models (Haiku) use training-data API knowledge, not the installed version.
- For any framework that has had breaking API changes (Gio, Wails v2→v3, etc.),
  the agent prompt must either:
  a) Include exact, version-correct API signatures (copy from installed source), OR
  b) Instruct the agent to read `~/go/pkg/mod/<pkg>@<ver>/*.go` before coding, OR
  c) Use a larger model (Sonnet/Opus) with better up-to-date knowledge.
- Alternatively, put framework version + key API snippets in AGENTS.md so they are
  always in context.

**Concrete check:** After `go get`, have the agent run
`grep -n "func.*<symbol>" ~/go/pkg/mod/<pkg>@<ver>/*.go` to verify the API before
writing code.

### TODO: Enforce no package-level mutable state in Go agents

The GTK agent used package-level globals (`var appWindow *gtk.ApplicationWindow`,
`var emojiSource emoji.Source`, etc.) — a known anti-pattern in Go.

**Action:** Add to `docs/Go.md`:
```
## State management
- No package-level mutable variables.
  Pass state explicitly via function parameters or a struct.
  Package-level vars create hidden coupling and hinder testing.
```

And mention it in AGENTS.md so agents see it in every project.

### TODO: Verify worktree write-back behaviour before spawning parallel agents

In this run, both agents appeared to write directly to
`~/projects/instamoji/...` (the main working tree path) rather than
to separate worktree paths.  This suggests either:
a) The harness applies isolation differently than expected, OR
b) Worktree isolation was nominal — the isolation path overlapped with the main tree.

**Action:** Test by spawning a single worktree agent that writes a sentinel file and
check whether it appears in the main working tree mid-run.  Document the actual
isolation model so future agents can rely on it.

### TODO: Consider per-cmd module layout when cmds have disjoint heavy deps

`cmd/instamoji-gtk` (CGO + libgtk-4) and `cmd/instamoji-gio` (CGO + Vulkan) now share
the same `go.mod` as the Wails root.  This means `go vet ./...` and `go test ./...`
pull in CGO for all three, making the CI/dev environment heavier.

**Options (document in Go.md §Project layout):**
- `go work` workspace: each `cmd/` has its own `go.mod`; `go.work` links them.
  Allows independent `go get` without root-level collision.  Slightly more ceremony.
- Keep flat module, add build tag `//go:build ignore` to `cmd/` files until deps
  are installed — but this breaks `go build ./...`.
- Accept the heavier module for now; revisit when CI is set up.

### TODO: Warn about CGO compile time before spawning agents on heavy-binding frameworks

gotk4 has 25 000+ lines of CGO-backed auto-generated bindings.  First `go build` takes
several minutes and saturates all CPU cores because each CGO file forks a gcc process.
Gio by contrast compiles in seconds (minimal C, no binding layer).

**Actions:**
- Document build time cost in AGENTS.md so the developer knows what to expect.
- Consider running `go build -x ./cmd/instamoji-gtk` once in the parent agent to warm
  the module cache before handing off to a sub-agent (avoids wasted agent time
  waiting for compilation).
- Add a `## Build time` section to AGENTS.md when CGO deps are involved:
  ```
  ## Build time (first build, cold cache)
  instamoji-gtk:  ~3-5 min  (gotk4 CGO bindings)
  instamoji-gio:  ~20 s     (minimal CGO)
  instamoji:      ~30 s     (Wails + webkit2gtk headers)
  ```

---

## Summary checklist for next parallel-agent Go task

Before spawning parallel worktree agents for Go:
- [ ] Pre-run `go get <frameworks>` in parent agent so go.mod is stable.
- [ ] Include exact API signatures (from installed source) in agent prompts.
- [ ] Add `make deps-<target>` targets for system packages.
- [ ] Add "no package-level mutable state" to Go.md and AGENTS.md.
- [ ] Verify worktree isolation model on this machine.
