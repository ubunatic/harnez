# 631 — External skills: canary, then user registry (B), then vendored skills (A)

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: docs/Canary.md, internal/claude/apply.go (skillTargets)

---

## 1. Problem & Motivation
harnez can only install skills that are embedded in its binary and listed file by file in
`config.yaml` (`skills:` with `file` + `resources`). Third-party skills such as
https://github.com/nateherkai/scroll-craft cannot be installed, searched, or explored, and their
external tool needs (node, ffmpeg, playwright, API keys) are invisible.

## 2. Technical Specification / Findings
- A skill is a directory with `SKILL.md` (YAML frontmatter: `name`, `description`, optional
  `allowed-tools`) plus optional `references/`, `scripts/`, assets. Agents load the description
  at startup, the body on trigger, the rest on demand. Same format for Claude Code, Codex
  (`.agents/skills/`), Gemini.
- A Claude Code plugin (`.claude-plugin/plugin.json`, `marketplace.json`) is packaging; it may
  also bundle hooks, commands, agents, MCP servers, which run code unasked and must be gated.
- scroll-craft: no hooks/MCP. 457-line SKILL.md, ~12 references, `engine/` JS/CSS, Node scripts.
  Needs node, full ffmpeg, `playwright-core` + Chrome (verify pass), optional paid
  `KIE_AI_API_KEY`. `scripts/doctor.mjs` checks them.
- harnez today installs to `~/.claude/skills`, `~/.codex/skills`, `~/.gemini/skills`,
  `~/.prime/agent/skills`; no whole-tree copy, no git source, no search.

## 3. Implementation & Verification Plan
1. **Canary** (docs/Canary.md): manually install scroll-craft (pinned commit) into
   `~/.claude/skills` and `~/.codex/skills`; verify each agent lists it, and that its relative
   `scripts/` paths and `doctor.mjs` work from the installed location. Record results here.
2. **B, external registry** (default): `~/.harnez/skills.yaml` (url + pinned ref);
   `harnez skill install|search|explore|show`; clone to `~/.harnez/skills/<name>@<sha>`, link into
   agent targets (global or `-d <project>`); explore shows frontmatter, tree, doctor output,
   hooks/MCP (off unless `--allow-hooks`); "lazy" skills reachable only via a small
   `skill-finder` skill to save startup context.
3. **A, integrated/vendored** (curated only): `harnez skill vendor <url>@<ref>` into
   `third_party/skills/<name>/` with LICENSE; new `dir:` field in `config.yaml` skills so apply
   copies the whole tree to all targets.

## 4. Canary Results
2026-09-29, scroll-craft @ 0b81622, plain `cp -r` of `plugins/nateherk-design/skills/scroll-craft`:
- Claude Code: listed immediately in the running session (hot reload, no restart).
- Codex (`codex exec`): lists `scroll-craft` from `~/.codex/skills`.
- `node scripts/doctor.mjs` works from the installed dir: node, full ffmpeg (545 filters, libwebp)
  ok; playwright-core, Chrome, `KIE_AI_API_KEY` missing (optional).
- Finding: the skill's workspace resolves from **cwd**, so running scripts from the install dir
  creates `scrollcraft/` inside it. Registry (B) must keep install dirs read-only / run from the project.
- Finding: plugin manifests are not needed for non-Claude agents; the skill dir alone is portable.
- Gemini/Prime not tested. Canary copies removed afterwards.

## 5. Step B Done (2026-09-29)
`harnez skill install|list|update|remove|search|explore|show` (`internal/skillreg`,
`cmd/harnez/skill.go`). Registry: `~/.harnez/skills/registry.yaml` + `src/<name>@<sha12>` clones
(`$HARNEZ_SKILLS_HOME` overrides). Installs only the skill dir, staged with a `.harnez-external`
marker, into all `claude.SkillTargets`; refuses foreign dirs, unsafe names, `-`-prefixed
URLs/refs; skips symlinks and `.git`. Reviewer findings 1-10 addressed. E2E verified against
scroll-craft in scratch targets.
Lazy skills + finder: covered by explicit-only mode (hidden from model skill lists) and `harnez skill search` plus the ask rule. Dropped (user, 2026-09-29): online marketplace search; search stays local only.
Next: step A (vendored skills) and [[632-re-home-mattpocock-derived-skills-as-external-skills-issue-631-registry]].

## 6. Explicit-Only Mode (2026-09-29)
User requirement: agents must never trigger an external skill on their own; when several could
fit, the agent lists them and asks. Default install is now explicit-only (`--auto` opts out):
- Claude copy: `disable-model-invocation: true`. Verified live: hidden from the model's skill
  list, `/name` still runs it; without the flag it is listed and auto-triggers.
- Codex copy: `agents/openai.yaml` `policy.allow_implicit_invocation: false`. Verified live:
  not listed, not triggered; without it, triggered.
- Gemini, Prime: no copy; `harnez skill show <name>`.
- Rule "External Skills" in generated `.harnez/rules/Tools.md`: search, list matches, ask.
Reviewer findings (CRLF/empty frontmatter, shared-dir labels, wording, tests) addressed.

## 7. Name Conflicts (2026-09-29)
Decision: keep upstream names (skills reference each other by name); rename only on conflict.
- `harnez skill install --as <name>` rewrites the frontmatter `name:`, stores `upstream:`,
  survives `update`; warns which files still mention the upstream name.
- harnez-managed skill names (config `skills:` + `decommissioned.skills`) are reserved; every
  conflict error suggests `--as <prefix>-<name>`.
- `apply` never overwrites/removes a marked external dir under a managed name (prints `clash`);
  `diff`, `status`, `revert` skip it too.
Known gap: the diff/status skip is covered by a helper-level test, not an end-to-end `DiffAll` test.

## 8. Step A, Bundled Skills (2026-09-29)
`config.yaml` `bundled_skills` (url, commit, dir, skills); `harnez skill vendor` copies them with
LICENSE into `third_party/skills/`, refusing repos without a license, reporting changed/gone
(rename hints)/not-bundled skills; `embed.go` embeds them; `apply` syncs them explicit-only
(`skillreg.SyncBundled`), idempotent, removes delisted ones, never touches registry or foreign
dirs. Clash with an own skill name is a config error; registry install refuses bundled names.
First set: mattpocock grilling, grill-with-docs, domain-modeling, to-spec @ c55ee46; the
registry installs of those four were removed and replaced live. Not done: `diff`/`status` do
not report bundled-copy drift (apply repairs it).
