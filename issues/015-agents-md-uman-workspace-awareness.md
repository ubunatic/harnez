# 015 — AGENTS.md must teach agents about uman (workspace glue)

**Status:** Open

## Context

No managed AGENTS.md mentions `uman`, yet it is the load-bearing workspace
tool: cross-project publishing (`uman website sync <project>`), push
orchestration, LFS manifests (`uman manifests`), and the `~/projects/.uman.toml`
config with `main_repo` and post-sync hooks.

Evidence (books session, 2026-07-05): an agent building the books library
website wrote a custom deploy script targeting `../ubunatic.com` directly and
had to be redirected by the user to `uman website sync` — the convention was
undiscoverable from any AGENTS.md, CLAUDE.md, or docs/ file, in a workspace
whose own doctrine is canary-first ("probe external mechanisms before
building features on them"). See books
`docs/adr/0002-website-publishing-via-uman-sync.md` for the outcome.

## Proposal

- Bundle a short "Workspace" section (or a registry doc, e.g.
  `docs/Workspace.md`) into managed AGENTS.md files, roughly:
  > Cross-project operations (publish, push, status, LFS manifests) go
  > through `uman` — see `~/projects/.uman.toml`. Project websites publish
  > via `uman website sync <project>` from the project's `website/` dir.
- Keep it one paragraph: enough for an agent to probe `uman --help` before
  inventing its own mechanism.
- Until bundled, projects carry the note manually (books and ubunatic.com
  AGENTS.md were adjusted by hand on 2026-07-05 — reconcile when bundling).

---

## Implementation Plan

Still valid: `docs/templates/AGENTS.md` and `config.yaml`'s `agents_md.*` sections
contain no `uman` mention (only `docs/Website.md` references `uman website sync`,
and that doc is `default: false` since 130). The workspace note currently lives
only in the hand-written `~/projects/CLAUDE.md`, which harnez does not manage.

### Design decision

Do **not** put this in `agents_md.global.sections` (the global file's own
"Minimal Global Docs" rule forbids content that isn't relevant to every project)
and do **not** hardcode it into `docs/templates/AGENTS.md` (not every harnez user
has `uman`). Instead mirror the existing, proven opt-in `repo_modes` mechanism:
a small named map in `config.yaml` selected by an `init` flag, emitting one
managed `<!-- harnez:begin Workspace -->` section into the project's `AGENTS.md`.
This keeps `apply` (global-only) untouched, per `docs/CLIDesign.md`.

### Steps

1. `config.yaml` — add a sibling of `agents_md.repo_modes`:
   ```yaml
   workspaces:
     uman:
       name: "uman workspace"
       content: |
         ## Workspace (uman)
         Cross-project operations (publish, push, status, LFS manifests) go
         through the `uman` CLI — see `~/projects/.uman.toml` for the managed
         projects, the main publishing repo, and post-sync hooks. Project
         websites publish from their `website/` dir via
         `uman website sync <project>`. Probe `uman --help` before building
         any cross-repo mechanism yourself.
   ```
   Keep it one paragraph — the goal is only "probe before inventing".
2. `internal/claude/config.go` — add `Workspaces map[string]RepoMode` (reuse
   `RepoMode`'s `{name, content}` shape; no new type needed) under `AgentsMD`.
3. `internal/claude/init.go` — in `RunInit` (~line 305, next to the `repoMode`
   block), accept a `workspace string` parameter and append
   `MDSection{Name: "Workspace", Content: ws.Content}` with the same
   unknown-key error path. Thread the parameter through `RunInitAll` (~line 441).
4. `cmd/harnez/init.go` — add `--workspace/-w <name>` (mirroring `--repo-mode/-m`),
   documented in the command's Long help as project-local only.
5. Docs: one line in `docs/CLIDesign.md`'s `init` flag list; mention the new
   section in `docs/README.md` if it enumerates managed sections.
6. Tests: extend the existing repo-mode init test in
   `internal/claude/init_test.go` with a workspace case — assert the section is
   written with markers, that an unknown name errors, and that omitting the flag
   writes nothing (idempotency).
7. Reconcile: books and ubunatic.com carry the note by hand (per Context above);
   after this lands, re-run `harnez init -w uman` there so the hand-written
   paragraph becomes a managed section. Do this as a follow-up, not in this ticket.

### Risks / open questions

- Naming: `workspaces:` vs. reusing `repo_modes:` with a `uman` entry. Separate
  key is better — repo mode and workspace glue are orthogonal (a fork repo can
  still be in the uman workspace).
- If a project already has a hand-written `## Workspace (uman)` heading, the
  marker-based `applySectionMD` will add a *second* section rather than adopt it.
  Either accept manual cleanup on the two known projects, or handle it the same
  way `migrateLegacyMarkers` does — prefer manual cleanup (two files).

### Scope

Small (one config map, one flag, one section append, one test).
