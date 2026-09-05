# promote command — push improved project docs back to source

**Status:** Open  
**Severity:** Feature — workflow gap

## Problem

`init` copies docs from harnez source into projects. Projects improve those
local copies over time (stronger guidance, corrected examples, etc.). There is no
mechanism to promote those improvements back to the harnez source, so they
are lost unless manually copied.

## Proposed interface

```
harnez promote [-c <config>] [--agent <cmd>] <file>
```

- `<file>` — path to a local doc in the project (e.g. `docs/Bash.md`)
- `-c <config>` — path to the harnez `config.yaml`; source dir is inferred
  from the config file's directory
- `--agent <cmd>` — agent command to run before promoting (overrides config default)

## Source dir resolution

`promote` writes the file back to `lang.Source` relative to the harnez source
dir. The source dir is the directory containing the `-c <config>` file. The embedded
binary has no source dir, so `-c` is required when using promote.

## Agent integration

`config.yaml` gains a new top-level section:

```yaml
promote:
  agent: claude        # command; empty = no agent, just copy
  prompt: |
    Review and improve this doc for use as a coding-agent reference.
    Keep it concise and actionable. No preamble.
```

`--agent <cmd>` on the CLI overrides `promote.agent` for that run. If agent is empty
(or `--agent` is not set and config has none), promote is a plain copy-back.

The agent receives the file content and the prompt; its output replaces the file
content before writing back to source.

## Flow

```
harnez promote -c ~/projects/harnez/config.yaml docs/Bash.md
        │
        ├── resolve source: ~/projects/harnez/
        ├── look up config entry by matching lang.Local = ./docs/Bash.md
        ├── if agent configured: run agent on file content → improved content
        ├── write improved content to lang.Source (e.g. docs/lang/Bash.md)
        └── print: promoted docs/Bash.md → ~/projects/harnez/docs/lang/Bash.md
```

## Status

Open — not yet started.

---

## Implementation Plan

### What already exists (2026-09-04)

The *detection* half of this workflow shipped since the ticket was filed:

- `harnez diff --capture-docs [--out]` → `CaptureDocsDrift` (`internal/claude/docs_capture.go:149`)
  writes a Markdown drift report of source-vs-local managed docs into `issues/inbox/`.
- `harnez scan-docs <dir>` → `ScanDocs` (`:32`) does the same across sibling repos.
- The `harnez-sync` skill triages those reports; `harnez dochistory` tracks doc evolution.
- `compareConfiguredDocs` (`:275`) already resolves every `lang.Source` ↔ `lang.Local` pair,
  extracts the managed portion via `markdown.ExtractManagedDocContent`, and produces a
  unified diff per file.

What is still missing is exactly the ticket's premise: a **write-back**. Today a valuable
local improvement must be copied into `docs/lang/*.md` by hand. So the ticket stands, but its
implementation is now mostly *reuse*, not new machinery.

### Steps

1. **`cmd/harnez/promote.go`** — `newPromoteCmd()`, registered in `main.go`'s
   `root.AddCommand(...)` list.
   ```
   harnez promote -c <config> [-d <project dir>] [--agent <cmd>] [--dry-run] [-y] <file>...
   ```
   `-c` is **required** (the embedded FS has no writable source dir — error out with
   "promote needs -c <path to harnez config.yaml>: the embedded config has no source tree").
   `-d` defaults to cwd, matching `init`. Accept multiple `<file>` args; `--all` promotes
   every `changed` doc found by the existing comparison.

2. **`internal/claude/promote.go`** — `Promote(repoDir string, cfg *Config, files []string, opts PromoteOptions) error`:
   - Resolve each `<file>` against the configured pairs by reusing the resolution logic in
     `compareConfiguredDocs` — extract a small helper
     `resolveDocPair(repoDir, cfg, localPath) (name string, lang Language, sourcePath string, ok bool)`
     so both `docs_capture.go` and `promote.go` share one matcher. Unknown file → error
     listing the promotable names.
   - **Promote only the managed portion.** Read the local file, take
     `markdown.ExtractManagedDocContent(local)` — everything from a stop marker onward is
     project-local by definition and must never reach the shared source. This is the single
     most important correctness rule in this command.
   - Preserve the source's own front matter / `<!-- harnez:bundled -->` marker: splice the
     promoted body under the source's existing header rather than overwriting the whole file
     (compare `docs/practices/IssueTracking.md`, which carries a `title:`/`weight:` front
     matter block the project copy does not).

3. **Agent pass (optional)** — `promote:` block in `config.yaml` as specified in the ticket
   (`agent`, `prompt`). Reuse the existing `claudeP(dir, prompt)` helper in
   `internal/claude/init.go:~90` and the `sanitizeContent`/`stripMetaCommentary` cleaning it
   already applies to LLM output — do not write a second LLM invocation path. Empty agent (or
   no `--agent`) ⇒ plain copy-back.

4. **Always show the diff and confirm.** Before writing, print the unified diff
   (`unifiedDocDiff`, already in `docs_capture.go:391`) source→promoted and prompt unless
   `-y`. `--dry-run` prints and exits. Promotion writes into the user's *other* repo; a
   silent write is the wrong default.

5. **Output** — one line per file:
   `promoted docs/Bash.md → ~/projects/harnez/docs/lang/Bash.md (+12/-3)`.
   Remind the reader that the source repo now has uncommitted changes; do **not** git-add or
   commit anything.

6. **Tests** — `internal/claude/promote_test.go`:
   - round-trip: temp source dir + temp project dir, edit the local copy, promote, assert the
     source file now matches the edited managed content **and** retains its front matter.
   - stop-marker case: local doc with content after `<!-- harnez:stop -->`; assert the tail is
     *not* promoted.
   - unresolvable file → error naming the valid targets.
   - `--dry-run` leaves the source byte-identical.
   - agent configured but binary missing → clear error, source untouched.

7. **Docs** — add `promote` to `docs/CLIDesign.md`'s command table with its scope
   ("Project → harnez source; requires `-c`"), and a short README section pairing it with
   `diff --capture-docs` (detect → promote).

### Design decisions / tradeoffs

- **Reuse `compareConfiguredDocs`' pair resolution** rather than reimplementing
  `lang.Local` matching — two matchers would drift and produce "promotes to the wrong file",
  the worst possible failure for this command.
- **Managed-portion-only promotion** — non-negotiable; the stop-marker protocol is what makes
  `init` safe to re-run, and promoting past it would leak project-specific content fleet-wide.
- **`-c` required, source dir = config's dir** — as the ticket specifies; keeps `promote`
  from ever guessing which harnez checkout to write to.
- **Agent pass is opt-in and off by default.** A plain copy-back is predictable and reviewable;
  an LLM rewrite between two repos with no diff shown would be unauditable.
- **No commit, no push.** `promote` writes files; git is the user's (or `uman`'s) job.

### Risks / open questions

- Front-matter splicing is the fiddly part: source docs under `docs/practices/` and
  `docs/lang/` have a Hugo-style `---` header that project copies lack. Verify against
  `docs/practices/IssueTracking.md` and `docs/lang/Go.md` before settling on the splice rule.
- `promote` writing into a *different* repo than cwd is unusual for this CLI; the required
  `-c` plus mandatory diff confirmation are the guardrails.
- Open: should `promote` refuse when the source repo's working tree is dirty for that file?
  Leaning yes-with-`--force`, to avoid stacking an LLM rewrite on top of uncommitted edits.
- Overlap check: if the `harnez-sync` skill grows a write-back step, it should shell out to
  `harnez promote` rather than duplicating it.

### Scope

**Medium-to-large** — new command + new package file (~250 LOC), but the diff, pair
resolution, managed-content extraction and LLM-invocation pieces are all existing helpers.
The agent-pass and front-matter splicing are the genuinely new logic.
