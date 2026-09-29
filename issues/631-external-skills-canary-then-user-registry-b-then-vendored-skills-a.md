# 631 — External skills: canary, then user registry (B), then vendored skills (A)

**Status**: Open
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
- Gemini/Prime not tested. Canary copies left in place for use; remove with
  `rm -r ~/.claude/skills/scroll-craft ~/.codex/skills/scroll-craft`.
