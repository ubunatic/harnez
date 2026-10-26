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
| `init`  | Project (cwd / `-d`) | `AGENTS.md`, `CLAUDE.md` symlink, `.harnez/rules/*.md`, `docs/<name>.md` copy, `Makefile` |
| `diff`  | Global / Project | Preview of what `apply` would change; `--capture-docs` writes project drift to inbox |
| `scan-docs` | Workspace | Read-only scan of child projects for managed doc drift |
| `revert --managed` | Global | Remove managed keys / strip MD sections |
| `clean [procs] [q1]` | Runtime / Repository | Inspect or remove stale process groups and release verified incomplete quota-1 state |
| `status`| Global | Config summary + applied-state checks |
| `usage` | Multi-Agent (local/remote) | Zero-cost token counters, live quota tracking, project attribution (`--project`), procs (`-p`), remote host (`--host`) |
| `server` | Local / User Service | Run local services; install, inspect, and remove the systemd user collector service |
| `install <name>` | User toolchain | Install an optional Go tool from `spec/dependencies.yaml`, then run its bootstrap; `--dry-run` prints commands |
| `usage history` | Analytical / Logs | Timeline, remote fetch, stats & sparklines across `~/.claude/harnez/usage-history/` |
| `assess`| Local / Repo | Fast code/doc metrics, token estimation, and feasibility report (`--json`) |
| `dochistory` / `repo-history` | Local / Repo | Multi-track Git artifact evolution, Braille sparklines (`--tracks`), and diff breakdown (`--diff`) |
| `mode`  | Local / Repo | Switch ConciseMode terseness level in `.harnez/rules/Local.md` (migrating legacy `AGENTS.local.md` once) |
| `distill` | Shell / Hooks | Distill verbose command outputs for context conservation |
| `release` | Local / Repo | Language-agnostic version bump, build, minisign signing, and forge publishing |
| `find`  | Local / Repo | Fast repository entity discovery with short fuzzy-filter grammar (`issues`) |
| `read`  | Local / Agent Context | Bounded file reading and multi-column visual PNG context card rendering (`-I/--image`) |
| `log`   | Harnez's own invocation record | Chronological `git log`-shaped stream of harnez CLI invocations (`cli_invocations`) |
| `session extract <id\|path>` | Local Claude transcripts | Stream compaction summaries and owner messages as text or `--json`; session ID prefixes resolve under `~/.claude/projects/` |

Optional Neus installation and server indexing are described in [Neus integration](NeusIntegration.md).

`log` is a third axis alongside `find` and `stats`, not a synonym for either: `find`
reads **repository entities** (tickets, docs), `stats` and `dochistory` render
**aggregates** over tool calls and managed docs, and `log` replays **harnez's own
invocations** — which subcommand ran, when, where, by whom, and whether it succeeded.
It takes no subcommands by design, so it can never grow wrappers that duplicate
`harnez stats`, `harnez find issues history`, `harnez dochistory`, or
`git log -- docs/ issues/`.

`apply` and `init` operate on disjoint flag surfaces by design. `apply` takes `-t`
(Claude config dir); `init` takes `-d` (project dir). They cannot be confused.

For terminal layout mockups and implementation handoff, see
[TUI Design — Indicators, Panels & Keybinding Invariants](TUIDesign.md#ansi-visual-mockups).

## Interactive discovery & bounded ingestion

Commands designed for exploratory use in agentic loops or interactive terminals (e.g. `harnez find issues`) must respect context window budgets:

1. **Default Bounded Limits**: Commands should paginate by default (e.g. `-n 10` for the last/top items) rather than dumping unbounded records. Provide an explicit `--all` escape hatch to uncap results when needed.
2. **Forgiving Zero-Arg Defaults**: Running an exploratory command with zero arguments or filters should surface recent/high-level items instead of throwing a usage error.
3. **Deterministic Ranking & Output**: Keep output parsable (e.g. TSV without ANSI escapes) with stable tie-breaking.

## Exit codes: never `os.Exit` inside `RunE`

## Read output selection and compression

`harnez read` reads files or stdin. Text selection (`-L`, `--head`, `--tail`) happens
before display conversion. `-I` forces PNG output. `--auto` is retained as a
text-only compatibility flag and never selects images. Combining `-I` with
`--text` or `--raw` is an error. `-n` forces line-numbered text unless `-I` is
also supplied.

- `--line-numbers=all|off|none|N|every:N` controls image and explicit text gutters.
  Cadences mark the first selected line and original source multiples of N;
  intermediate lines have a dot. Compression retains original source anchors.
- `--compress=ws|ast` currently selects the same conservative token compaction.
  Go requires a complete parseable file and verifies the resulting token stream,
  preserving comments, literals, and implicit semicolons. JSON uses `json.Compact`,
  preserving numeric spelling and strings; its single output line anchors to the
  beginning of the selection. Shell preserves newlines and quotes while collapsing
  unquoted horizontal whitespace; heredocs, escapes, and complex expansions are
  left verbatim. Unsupported languages and incomplete Go/JSON return errors.
  Original files are never rewritten. JSON image metadata includes original text
  costs alongside compacted text/image costs.
- Image packing defaults to at most three columns. Each page is cropped after
  wrapping; up to 20 rows use one column, larger pages balance across available
  columns. `--columns=N` changes the maximum. JSON `pages` reports each page's
  dimensions; aggregate token estimates sum those pages rather than assuming
  that the last page is full.
- Provider detection and token estimates remain available for explicit analysis
  and benchmarks. They do not affect `harnez read` output selection.

`harnez subagent --doc-mode=auto --task '...' --dir DIR` stages documentation and
prints JSON. Optional `-- launcher args...` runs the explicit launcher with the
prompt on stdin and `HARNEZ_STYLE_GUIDE` pointing to the staged PNG. It does not
attach an image through a provider API; the launcher/agent opens that asset.
Explicit `full`, `lite`, and `vision` modes override provider defaults.

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
        ├── load and validate .harnez/repo.yaml, or detect the project layout and write it once
        │   └── explicit --issues-git, --quota-1, and --docs choices update their recorded values
        ├── warn on stderr when init will rewrite files with uncommitted changes
        ├── check for project drift (go.mod module vs. git origin vs. directory name)
        ├── create AGENTS.md (template) if absent
        ├── create CLAUDE.md symlink → AGENTS.md
        ├── ignore /issues/README.md.lock in .git/info/exclude when issues/ exists
        ├── install issue-index attributes, local merge driver, and hook only when
        │   repo.yaml says issues.tracker: harnez and issues.merge_driver: true;
        │   otherwise remove only Harnez-managed attributes, config, and hook blocks
        ├── auto-detect docs from config.yaml and record the initial layout in repo.yaml;
        │   later runs copy recorded optional docs and hint about newly detected ones
        ├── apply cfg.AgentsMD.Local sections (Language Conventions, etc.)
        ├── add container-test only when repo.yaml make.container_test is true
        ├── include quota rules only when repo.yaml rules.quota is true (or --quota-1 enables it)
        ├── add tracker/studies notices to .harnez/rules/Index.md when those features are disabled
        └── resolve explicit hard doc dependencies transitively
            (project-local only; never expands apply's global scope)
            └── for each selected doc (recorded + explicit + baseline + dependencies):
                ├── copy docs/<name>.md locally  (for @docs/ refs)
                ├── scaffold Makefile from template when make.scaffold is true
                └── inject Make targets when make.scaffold is true
```

All steps are idempotent. Running `init --docs golang` twice is safe. Running in non-coding repositories or `$HOME` is rejected unless `--force` (`-f`) is explicitly passed.

When stdin and stdout are terminals, `init` prompts for repository features with
detected/recorded defaults and saves the choices to `.harnez/repo.yaml` before
continuing. Enter or EOF keeps each default; invalid answers are asked again.
`-y`/`--yes`, `--no-wizard`, `--all`, CI (`CI` set to a nonempty value), or a
redirected stream skips automatic prompting. `--wizard` forces the prompts even
with redirected streams or CI; `-y` and `--all` still take precedence.
`--wizard` and `--no-wizard` are mutually exclusive. Explicit setup flags supply
wizard defaults; the answers are final for that invocation. Choices cover the
tracker, studies index, merge driver (available only when the Harnez tracker is on),
optional doc copies, Makefile scaffolding, and quota rules. Baseline docs remain
enabled; general rules have no config toggle
and are omitted. Answering yes to optional doc copies with no detected or recorded
copies leaves `docs.copies` empty; add a copy later with `init --docs <name>`.
The wizard rewrites `.harnez/repo.yaml`, so YAML comments are not preserved.
Disabling a feature records its existing config opt-out; it does
not delete existing user docs, Makefiles, or tracker files. Tracker adoption from a
managed issues index occurs only when `.harnez/repo.yaml` is absent; recorded opt-outs
remain in force on later noninteractive runs.

`.harnez/repo.yaml` is project-local configuration, separate from the embedded/global
`config.yaml` used by `apply`. On first init, Harnez detects the existing tracker format,
studies directory, installed Make targets, quota section, and language files, then writes
the result once. Later runs respect `docs.copies` for optional detected docs and print an
`init --docs <name>` hint for newly detected docs that are not recorded. Schema errors,
including unknown keys, stop init before project changes. Explicit setup flags update the
corresponding recorded choices. `make.scaffold` defaults to true when omitted for compatibility
with existing files; set it to false to copy the Make doc without editing or creating a Makefile.
The file-first Harnez issue tracker remains the recommended default. A repo detected without
that tracker or indexed studies gets a notice in `.harnez/rules/Index.md`, which the generated
rules header asks agents to read. `harnez index` regenerates issue indexes only when
`issues.tracker` is `harnez`, and regenerates the studies table only when `docs.studies` is
true. Those gates are independent. With neither enabled index available, it prints a note
and exits successfully without rewriting either index. In a fresh repo, `harnez issues new`
creates the `issues/` directory and records the Harnez tracker opt-in; the next `harnez init`
installs its Git integration and removes the disabled-tracker notice.

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
harnez apply [-t <dir>] [-d <name|profile>...] [--clean-docs] [--force-docs] [--debloat | --debloat-preset <minimal|aggressive>] [--codex-target <file>]
        │
        ├── merge managed keys into settings.json
        ├── write ~/.claude/commands/<name>.md for each command
        ├── write each skill's SKILL.md and declared resources to ~/.gemini/skills/<name>/
        ├── write each skill's SKILL.md and declared resources to ~/.codex/skills/<name>/ when configured
        ├── merge [hooks.harnez] into Codex config.toml when configured
        ├── write each skill's SKILL.md and declared resources to ~/.claude/skills/<name>/ (real Agent Skills, auto-loaded)
        ├── write ~/.prime/agent/prompts/<name>.md and skills/<name>/SKILL.md
        │   └── skill dirs: apply owns dirs with a .harnez-managed marker and rewrites them on every run;
        │       an unmarked dir holding only a SKILL.md with the same frontmatter name is adopted (849);
        │       any other unmarked dir is kept and reported by apply and diff with a takeover hint;
        │       SKILL.md gets one frontmatter block, source keys win over generated name/description (728)
        ├── install opt-in docs to ~/.claude/docs/ and ~/.prime/agent/docs/ if specified via -d/--docs (default: 0 global docs; profiles: core, dev, full)
        ├── if --clean-docs / --no-docs: prune unmanaged docs from ~/.claude/docs/ and ~/.prime/agent/docs/
        └── if --debloat / --debloat-preset: merge the selected Claude and Antigravity presets and spec-listed Codex features
```

Codex config ownership, status, revert, and the exact debloat boundary are
documented in [CodexSettings.md](CodexSettings.md). `config.yaml`'s
`debloat.codex_features` lists every Codex feature that debloat manages; an
unlisted feature is outside that set.

### Claude debloat presets

`harnez apply --debloat` selects `minimal`; `harnez apply --debloat-preset minimal`
or `--debloat-preset aggressive` selects the preset explicitly. This is global
configuration, affecting **every project** using the configured Claude settings
directory (default `~/.claude`), rather than a project-local setting. Plain `apply`
does not select a debloat preset.

The `debloat` section in `config.yaml` owns preset membership; its gating keys
are declared in `spec/schemas/config.schema.json`:

- `preset_disable_bundled_skills: true` sets `disableBundledSkills: true`.
- `preset_disable_claude_ai_connectors: true` sets
  `disableClaudeAiConnectors: true` under either preset. The standalone
  `--debloat-disable-claude-ai-connectors` flag also remains available.
- `preset_denied_mcp_servers: [harnez, codebase-memory-mcp]` adds named objects
  to `settings.json`'s `deniedMcpServers`. Claude stops loading those servers;
  their registrations stay intact. Harnez CLI commands remain available.
- `preset_skill_overrides` maps skill names to `user-invocable-only` (slash
  invocation remains available; the startup skill description is hidden) or
  `off` (the skill is disabled). These overrides apply under either preset.
- `aggressive_skill_overrides` adds `evergreen` and `lmcoder` as invoke-only
  under the aggressive preset. Tool deny lists remain controlled separately by
  `minimal_deny` and `aggressive_extra_deny`; neither preset denies `ToolSearch`.

The issue 968 additions make `gokay-sweep`, `sitegen-sweep`, `reverse-sprint`,
`epic`, `gokay`, `sitegen`, and `publish` invoke-only, alongside the existing
`lean-sprint-v1` and other slash workflows listed in the config.
The following skill names are `off`:

- Step-1 system skills: `google-workspace`, `xlsx`, `pptx`, `computer-use`,
  `built-in-browser`, `artifact-emulator`, `deep-research`, `mcp-builder`,
  `slack-gif-creator`, `import-memory`, `plugin-authoring`.
- Additional unused skills: `internal-comms`, `canvas-design`,
  `web-artifacts-builder`, `chrome-browser`, `codebase-memory`,
  `codebase-memory-scout`, `codebase-memory-auditor`.

`tool-feedback-protocol` remains available. Skill overrides do not remove Custom
Agents entries; the three codebase-memory agents still appeared in the live
context check. The separate harnez-channel plugin also retains its own MCP row.

`docs/commands/epic.md`, `reverse-sprint.md`, and `lean-sprint-v1.md` carry
`disable-model-invocation: true` in source frontmatter. Every plain `apply` also
installs these invocation gates to the configured skill targets, independently
of debloat. The two sweep skills have no Harnez source here and use Claude
overrides only. Edit the source frontmatter to change the three source gates.

Inspect with `harnez status --debloat`; undo with `harnez revert --debloat`.
The Claude ownership record is `~/.claude/.harnez-debloat.json` (or the selected
target directory). Revert restores the first recorded connector and skill
override values and removes only MCP server denials introduced by Harnez. It
preserves pre-existing denials, including command/URL entries, and later
unrelated entries. Revert does not remove invocation gates installed from
source frontmatter.

The live check on Claude Code 2.1.310, `claude-haiku-5-5`, removed every targeted
skill row and the harnez, codebase-memory, and Claude Docs connector tool rows.
Skills fell from 5.3k to 1.7k reported tokens; deferred MCP tools (11.8k) became
absent. All targeted synced skills were hidden with overrides, so no claude.ai
account changes were required. System tools rose from 521 to 4k while deferred
system tools stayed at 6.7k: this is evidence of skill/MCP removal, not a claim
of lower system-tool usage. Compare individual `/context` rows as well as totals;
the final before/after table and command evidence are in
[issue 968](../issues/968-skill-and-tool-gating-disable-system-skills-make-selected-skills-invoke-only-identify-system-tools.md).

### `settings.json` env is merged per key

`apply` merges the `env` block key by key: it sets the keys harnez manages and keeps
every other key that is already in the file. Before issue 717 (2026-10-06) the
whole `env` map was replaced, and a later step (the jev plugin, which adds
`CLAUDE_CODE_ENABLE_FUNCTION_HOOKS`) put a removed key back. Every run therefore
reported `env: changed` and rewrote a byte-identical file. A step that adds to a
managed map after the base merge must see the merged result, or `apply` stops being
idempotent. `scripts/smoke-test.sh` now checks the file's bytes and mtime across a
second run, not only the reported change count.

Trade-off: an env key that harnez stops managing is no longer removed by `apply`;
delete it from `~/.claude/settings.json` by hand.

## Managed sections in existing global instruction files

Issue 818 adds `global_sections` in `config.yaml`, with `name` as the stable marker ID.
`apply` reconciles one `<!-- harnez:begin NAME -->` / `<!-- harnez:end NAME -->` pair
per section, updating in place and removing retired Harnez-marked sections. Existing
Tool Feedback Protocol blocks use the same ID and update without duplication; the
feedback opt-out also removes that block. User text outside complete blocks remains
byte-identical. `diff` previews additions, edits and removals without writing.

The `global_section_targets` list covers `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`,
`~/.gemini/GEMINI.md`, `~/.prime/agent/AGENTS.md` and `~/.pi/agent/AGENTS.md`.
These files belong to the user: as established in issue 415, `apply` creates neither
missing global instruction files nor a `~/AGENTS.md` symlink. `init` remains
project-only. Keep global sections minimal; project rules still belong in
`agents_md.local` and `agents_md.rules`.

## Plural command words

`rewritePluralCommands` (`cmd/harnez/plural.go`) maps a plural word in the leading command path
to its singular command (`harnez channels list` runs `channel list`) before Cobra parses argv.
Plurals never appear in help or completion. An existing name or alias always wins (`skills`
stays its own command), and the walk stops at the first flag or unknown word, so flag values
and positional arguments are never rewritten. New commands get this without registration.

## Flag shorthands

`apply --docs` has shorthand `-d`. `init --docs` does **not** — `-d` is already taken by
`--dir`. When adding new flags to either command, check for shorthand conflicts before
committing to a letter.

## Testing `apply` safely: `-t`/`--target` does not fully isolate it

`-t <dir>` redirects the Claude config directory. It does **not** redirect the Codex
config (`~/.codex/config.toml`); `--codex-target <file>` redirects that file for
`apply`, `status`, and `revert`. Neither flag redirects the `⚙` binary symlinks
(`~/.claude/bin`, `~/go/bin`, `~/.local/bin`, `~/.prime/agent/bin`), the bash
shim/env script (`~/.harnez/shims`, `~/.harnez/env.sh`). Those paths remain global because `apply` is a global
operation (see "Why the separation matters" above). Running `harnez apply -t
/tmp/some-test-dir` for a "quick isolated smoke test" still mutates the real, live
harness install.

This bit a real session (issue 316, 2026-09-15): a debloat-feature smoke test using
`-t` pointed the real `⚙` symlinks at a throwaway temp binary, which was then deleted,
leaving them dangling until a real `harnez apply` (no `-t`) was re-run to fix them. If
you need a genuinely isolated `apply` test, either accept that it will re-sync your real
global state (safe if already applied and idempotent) or point `HOME`/relevant XDG vars
at a scratch directory for the whole test process, not just `-t`.

## RAMP Maturity Assessment (`init --ramp`, `assess --ramp`)

`harnez init --ramp` and `harnez assess --ramp` provide an offline, read-only RAMP-informed estimate based on the four maturity levels:

- **L1: Unconfigured** — No validated committed AI artifacts.
- **L2: Grounded Prompting** — AI behavior rules (`AGENTS.md`, `.cursorrules`, `.windsurfrules`), AI tool config (`.claude/settings.json`), or agent architecture/standards docs.
- **L3: Agent Augmented** — Named agents, reusable commands (`.claude/commands/`), or domain skills (`skills/*/SKILL.md`).
- **L4: Orchestration** — Multi-agent flows (`flows/*.yaml`) or committed agent session logs (`.system_generated/logs/transcript*.jsonl`).

### Scoring Rules and Boundaries:
1. **Committed Baseline vs. Projected**: Only clean, committed files in Git qualify for the baseline score. Uncommitted changes and scaffolding projected by `init` are reported separately as *projected* until committed. When Git metadata is absent, existing files are handled explicitly as uncommitted/projected.
2. **Highest Qualifying Level**: The profile reports the highest level evidenced by qualifying artifacts without capping the score when lower-level categories are missing.
3. **Coherence Alerts**: If higher-level artifacts exist without foundational lower levels (e.g. L3 or L4 without L2 baseline instructions), a coherence warning is emitted.
4. **No False Promotion**: Generic documentation (`README.md`), build files (`Makefile`), CI configs (`.github/workflows`), and service units (`*.service`) do not promote RAMP levels. Ambiguous paths are marked `unknown` with low confidence.
5. **Offline & Non-Executing**: Assessment strictly reads file content (bounded to 64KB) and inspects Git metadata without executing any project commands (`go`, `make`, tests, scripts, or service managers).

## Known gaps

- `diff` and `revert --managed` are global-only and have no awareness of project Makefiles — see issue #009.
- `status` does not check whether the project Makefile targets block is present.
