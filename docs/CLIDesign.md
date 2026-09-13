---
title: CLI Design: apply vs init
weight: 30
---

# CLI Design — apply vs init separation

Documents the command structure, the design decision behind it, and the pitfalls it avoids.

## Command responsibilities

| Command | Scope | What it touches |
|---------|-------|-----------------|
| `apply` | Global harnesses | Claude settings/rules/commands plus Prime Agent rules/prompts/skills/docs |
| `init`  | Project (cwd / `-d`) | `AGENTS.md`, `CLAUDE.md` symlink, `docs/<name>.md` copy, `Makefile` |
| `diff`  | Global / Project | Preview of what `apply` would change; `--capture-docs` writes project drift to inbox |
| `scan-docs` | Workspace | Read-only scan of child projects for managed doc drift |
| `clean` | Global | Remove managed keys / strip MD sections |
| `status`| Global | Config summary + applied-state checks |
| `usage` | Multi-Agent (local/remote) | Zero-cost token counters, live quota tracking, procs (`-p`), remote host (`--host`) |
| `usage history` | Analytical / Logs | Timeline, remote fetch, stats & sparklines across `~/.claude/harnez/usage-history/` |
| `assess`| Local / Repo | Fast code/doc metrics, token estimation, and feasibility report (`--json`) |
| `mode`  | Local / Repo | Switch ConciseMode terseness level and sync AGENTS.local.md overlay |
| `distill` | Shell / Hooks | Distill verbose command outputs for context conservation |
| `release` | Local / Repo | Language-agnostic version bump, build, minisign signing, and forge publishing |
| `find`  | Local / Repo | Fast repository entity discovery with short fuzzy-filter grammar (`issues`) |
| `log`   | Harnez's own invocation record | Chronological `git log`-shaped stream of harnez CLI invocations (`cli_invocations`) |

`log` is a third axis alongside `find` and `stats`, not a synonym for either: `find`
reads **repository entities** (tickets, docs), `stats` and `dochistory` render
**aggregates** over tool calls and managed docs, and `log` replays **harnez's own
invocations** — which subcommand ran, when, where, by whom, and whether it succeeded.
It takes no subcommands by design, so it can never grow wrappers that duplicate
`harnez stats`, `harnez find issues history`, `harnez dochistory`, or
`git log -- docs/ issues/`.

`apply` and `init` operate on disjoint flag surfaces by design. `apply` takes `-t`
(Claude config dir); `init` takes `-d` (project dir). They cannot be confused.

## Interactive discovery & bounded ingestion

Commands designed for exploratory use in agentic loops or interactive terminals (e.g. `harnez find issues`) must respect context window budgets:

1. **Default Bounded Limits**: Commands should paginate by default (e.g. `-n 10` for the last/top items) rather than dumping unbounded records. Provide an explicit `--all` escape hatch to uncap results when needed.
2. **Forgiving Zero-Arg Defaults**: Running an exploratory command with zero arguments or filters should surface recent/high-level items instead of throwing a usage error.
3. **Deterministic Ranking & Output**: Keep output parsable (e.g. TSV without ANSI escapes) with stable tie-breaking.

## Exit codes: never `os.Exit` inside `RunE`

`os.Exit` terminates the process immediately, so nothing after it in the call stack ever
runs — including `main()`'s `executeAndRecord` (issue 326), the one place that writes a
`cli_invocations` row for `harnez log`. Every command that called `os.Exit` directly from
inside `RunE` (`index --check`, `issues --check`, `diff --exit-code`, `exec`, `distill`'s
subprocess wrapper) was invisible to `harnez log` until fixed, not merely mis-recorded —
see `docs/studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md`.

The rule, and the mechanism that makes it easy to follow, live in
`cmd/harnez/exitcode.go`:

- A `RunE` that needs a specific process exit code (not the plain 0/1 a returned error
  otherwise maps to — a `git diff`-style drift signal, or a wrapped subprocess's own exit
  code) returns `&exitCodeError{Code: n}` instead of calling `os.Exit`.
- `main()`, after `root.Execute()` returns, is the **only** place that calls `os.Exit`,
  via `exitCodeFromRunError` (do not confuse with `exec.go`'s own `exitCodeFromError`,
  which extracts a *wrapped subprocess's* shell-convention code from an `*exec.ExitError` —
  a different mapping over a different kind of error).
- `silenceIfExitCode(cmd, err)` sets `cmd.SilenceErrors`/`cmd.SilenceUsage` only when `err`
  is the sentinel, so Cobra prints nothing extra for a signal the command already reported
  itself. This works because Cobra reads both fields at print time, after `RunE` returns —
  not at command-construction time — so setting them from deep inside `RunE`, immediately
  before returning the sentinel, leaves the same command's other, genuine error returns
  printing exactly as before. Do not set `SilenceErrors`/`SilenceUsage: true` on the command
  struct itself just to cover this one path; that would also swallow real errors.

When adding a new command that needs a non-1 exit code, use this pattern from the start —
do not reach for `os.Exit`.

## Why the separation matters

Before the split, `apply` accepted both `-t <claude-dir>` and `-p <project-dir>`.
The flags look symmetric but have opposite blast radii:

- `-t .` would write `settings.json`, `CLAUDE.md`, and `commands/` into the **cwd**,
  silently corrupting a project directory.
- `-p /wrong/path` would scaffold Makefile targets in the wrong repo.

A user asking "do I need `-p .`?" revealed this footgun. The fix was structural:
project-local work lives in `init`, which defaults to cwd and has no `-t` flag at all.

## Design evolution

1. **Phase 1** — `apply -p <dir>` did everything: global sync + project-local wiring.
2. **Phase 2** — Added `--setup` flag to gate Makefile injection (opt-in).
3. **Phase 3** — Recognised the deeper issue: `apply` mixes two orthogonal concerns.
   Evaluated three options:
   - A: Validate-only (error if `--setup` without `-p`)
   - B: Merge project work into `init` ← **chosen**
   - C: New `project` subcommand
4. **Current** — `apply` is global-only; `init -l <lang>` owns all project setup.

Option B was chosen because `init` already existed as a "bootstrap this project" command
and extending it was natural, while `apply` became a clean "maintain my Claude install"
command. Issue #001 (diff/clean wrong path when `--project` set) was resolved as a
by-product: there is no `--project` flag anymore.

## init flow

```
harnez init [-d <dir>] [--docs <name>...] [--issues-git[=false]] [--force]
        │
        ├── validate target directory (refuses $HOME, root, or non-coding dirs without --force)
        ├── check for project drift (go.mod module vs. git origin vs. directory name)
        ├── create AGENTS.md (template) if absent
        ├── create CLAUDE.md symlink → AGENTS.md
        ├── ignore /issues/README.md.lock in .git/info/exclude when issues/ exists
        ├── enable issue-index attributes, local merge driver, and hook only with --issues-git
        │   └── --issues-git=false narrowly removes only the harnez-managed integration
        ├── auto-detect docs (default:auto/true entries in config.yaml)
        ├── apply cfg.AgentsMD.Local sections (Language Conventions, etc.)
        └── resolve explicit hard doc dependencies transitively
            (project-local only; never expands apply's global scope)
            └── for each --docs <name> (explicit + auto-detected + dependencies):
                ├── copy docs/<name>.md locally  (for @docs/ refs)
                ├── scaffold Makefile from template  (if no Makefile)
                └── inject targets block into existing Makefile
```

All steps are idempotent. Running `init --docs golang` twice is safe. Running in non-coding repositories or `$HOME` is rejected unless `--force` (`-f`) is explicitly passed.

## Copyable-doc contract

- `depends_on` names normative sibling docs required to follow a rule. `init` copies the
  deterministic, dependency-first transitive closure and rejects unknown dependencies or cycles.
- Illustrative examples and case studies are not dependencies. Copyable docs must summarize their
  lesson inline or use a stable external link; they must not leave repository-relative links that
  only resolve in harnez's source tree.
- A copyable doc may name `docs/feedback/`, `docs/studies/`, or another destination only as an
  optional convention with an explicit fallback when that directory is absent.
- `capabilities` documents an opt-in architecture governed by a whole doc (for example
  `remote-deployment`). Generic docs must state such rules conditionally; `init` does not generate
  per-project prose variants or infer architecture from directory names.
The lock-file rule is local to each clone and leaves the project's shared
`.gitignore` untouched; rerun `init` after cloning a harnez-managed tracker.

## apply flow

```
harnez apply [-t <dir>] [-d <name>...] [--force-docs]
        │
        ├── merge managed keys into settings.json
        ├── write ~/.claude/CLAUDE.md managed sections
        ├── create ~/AGENTS.md symlink → ~/.claude/CLAUDE.md
        ├── write ~/.claude/commands/<name>.md for each command
        ├── write each skill's SKILL.md and declared resources to ~/.gemini/skills/<name>/
        ├── write each skill's SKILL.md and declared resources to ~/.codex/skills/<name>/ when configured
        ├── write each skill's SKILL.md and declared resources to ~/.claude/skills/<name>/ (real Agent Skills, auto-loaded)
        ├── write ~/.prime/agent/AGENTS.md managed sections
        ├── write ~/.prime/agent/prompts/<name>.md and skills/<name>/SKILL.md
        ├── write agents_md.agents[<id>] managed sections into that agent's own target
        │       (e.g. ~/.codex/AGENTS.md) — skipped if the target's parent dir is absent
        └── install docs to ~/.claude/docs/ and ~/.prime/agent/docs/
```

## Agent-specific instruction profiles (`agents_md.agents`)

`agents_md.global` and `agents_md.local` are shared: every section they carry lands in
every target agent reads (`~/.claude/CLAUDE.md`, and via the `~/AGENTS.md` symlink,
Codex, Gemini, and Prime Agent too). That's the wrong shape for a correction that is
inert or actively wrong for agents other than the one it's about — see issue 149. Codex
subagents have no host-notified background-job/subagent-completion callback the way
Claude Code does; telling Claude Code or agy "don't poll" would just be noise, since they
already behave correctly.

`agents_md.agents` is a `map[string]AgentsMDTarget` (same struct `global`/`local` use —
`target`, `symlink`, `template`, `content`, `sections`) keyed by agent id. Each entry owns
a real file that only that agent reads; nothing here is filtered into a shared file. `apply`
skips an entry whose target's parent directory doesn't exist on disk, so a user who
doesn't run Codex never gets a `~/.codex` directory created for them. `diff`, `clean`, and
`status` all know about this map too — an agent profile is a fully managed target, not a
one-off write.

**Rule of thumb**: shared behavior (applies the same way to every agent) goes in
`global`. A correction that would be inert or wrong for another agent goes in
`agents.<id>`.

Current agent-owned targets:

| Target file | Agent(s) | Managed via |
|---|---|---|
| `~/.claude/CLAUDE.md` | Claude Code | `agents_md.global` |
| `~/AGENTS.md` (symlink → `~/.claude/CLAUDE.md`) | Codex, Gemini (shared content only) | `agents_md.global.symlink` |
| `~/.prime/agent/AGENTS.md` | Prime Agent | `agents_md.global` (mirrored target) |
| `~/.codex/AGENTS.md` | Codex only | `agents_md.agents.codex` |

## Flag shorthands

`apply --docs` has shorthand `-d`. `init --docs` does **not** — `-d` is already taken by
`--dir`. When adding new flags to either command, check for shorthand conflicts before
committing to a letter.

## Known gaps

- `diff` and `clean` are global-only and have no awareness of project Makefiles — see issue #009.
- `status` does not check whether the project Makefile targets block is present.
