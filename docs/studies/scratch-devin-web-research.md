# Devin CLI web research (scratch, issue #1063)

Scope: public Cognition sources only. Date of research: 2026-10-31.
Pages read in full: `docs.devin.ai/cli.md`, `cli/extensibility.md`, `cli/acp/zed.md`, `cli/reference/commands.md`, `cli/subagents.md`, `cli/sandbox.md`, `cli/ssh.md`, `cognition.com/blog/local-fusion`.
Not read: `cli/models.md`, `cli/reference/permissions.md`, MCP/hooks/skills/plugins sub-pages, `cli/cloud.md`, Outposts reference.

## Key corrections to the study premise

1. **"DevInfusion" does not appear in any source.** Every search and page returned nothing under that name. The closest real term is **Devin Fusion** (Cognition's multi-model harness, see below). If the study refers to "DevInfusion" from another source, that source should be checked.
2. **"DRS" is not "Devin Remote Sandbox" in the official docs.** `devin cloud drs` expands to **Declarative Repo Setup**. It covers blueprints (`blueprint-create`, `blueprint-write`, `build`, `build-wait`, `build-logs`, `secret-create`), and `sandbox-create` / `run` for cloud sandboxes. The name "Devin Remote Sandbox" does not occur.
3. **Name collision:** a PyPI package `devin-cli` (and a GitHub repo `revanthpobala/devin-cli`) is an unofficial, non-Cognition client for the Devin API. Do not mix its flags (`--token`, `--org`, `--devin-mode`, `--bypass-approval`) with the official CLI.

## 1. Install and identity

- Official installer: `curl -fsSL https://cli.devin.ai/install.sh | bash`. Also a Homebrew cask. Windows uses PowerShell; native Windows sandboxing is not supported (use WSL 2).
- Official docs describe it as a local coding agent in the terminal. Cloud Devin is a separate VM-based engineer.
- Known gap: the CLI "does not yet support Knowledge, Playbooks, or Secrets from your Devin account."

## 2. Invocation and global flags (`devin [OPTIONS] [prompt]`)

| Flag | Short | Env | Notes |
|---|---|---|---|
| `--model <MODEL>` | | `DEVIN_MODEL` | |
| `--permission-mode <MODE>` | | `DEVIN_PERMISSION_MODE` | `normal` (alias `auto`, default), `accept-edits`, `smart`, `dangerous` (aliases `yolo`, `bypass`), `autonomous` (requires `--sandbox`) |
| `--sandbox` | | `DEVIN_SANDBOX` | Research preview; OS isolation (macOS seatbelt, Linux bwrap + seccomp) |
| `--cloud` | | | Drives Devin Cloud sessions instead of local agent |
| `--continue` | `-c` | | Resume latest session in cwd |
| `--resume <ID>` | `-r` | | With `--cloud`, accepts ID or URL |
| `--print [PROMPT]` | `-p` | | Non-interactive; fails in untrusted dirs unless `--respect-workspace-trust false` |
| `--prompt-file`, `--config <PATH>`, `--export [PATH]` (ATIF per turn), `--respect-workspace-trust [true\|false]` (default true) | | | |

Permission modes: **Normal** (default, full autonomy with checks), **Plan** (read-only), **Smart** (fast model auto-approves safe actions; gradual rollout), **Bypass** (approves everything), **Autonomous** (needs `--sandbox`; OS sandbox is the boundary).

## 3. Subcommands

- `devin auth login|logout|status` (`login --force-manual-token-flow` for SSH sessions). `devin setup`, `devin doctor [--json]` (lists loaded custom subagent profiles), `devin update`, `devin version`, `devin uninstall`.
- `devin mcp add|list|get|remove|login|logout|enable|disable`. `add` takes `-t stdio|http`, `-s local|project|user` (default `local`), `--url`, `--command`, `-e`, `-H`, `--scopes`, `--oauth-resource`.
- `devin models list [--format json]`
- `devin rules list|show|paths` (`--provider cursor|windsurf`)
- `devin skills list [--trigger user|model] | show | paths`
- `devin plugins install <source> [-y] | list | info | update | remove [--force] | prune`. Source: `owner/repo`, git URL, or local path, with `#subpath` for a plugin inside a repo.
- `devin migrate hooks|workflows [--scope all|workspace|global]`
- `devin list|ls [--format json|csv]`
- `devin acp [--agent-type] [--model]`: ACP server over stdio (JSON-RPC), launched by an editor.
- `devin connect <CODE> [--public]`: connects this machine to a cloud session so it can run commands locally (foreground).
- `devin ssh <session> [--gateway host[:port]] [ssh opts]`: passthrough options, e.g. `-L 8080:localhost:8080`. Plain alternative: `ssh devin-<id>@ssh.devin.ai` (browser approval URL printed). `scp` works the same.
- `devin forward <session> <port>... [--gateway]`: `3000` maps to `localhost:3000`; `8080:3000` maps local 8080 to VM port 3000. Runs until Ctrl-C. No limits stated.
- `devin sandbox setup`: local sandbox prerequisites (Linux needs `bwrap` and `socat`).
- `devin cloud drs whoami | sandbox-create --repo | run --devin-id --command [--timeout] | blueprint-list | blueprint-create | blueprint-write | build | build-start | build-wait | build-logs | secret-create`.
- `devin worker` (hidden, Outposts). `devin airgap doctor` (air-gapped builds only).

## 4. Interactive slash commands

- Mode/model: `/mode [normal|accept-edits|smart|plan|bypass]`, `/plan`, `/ask <q>`, `/bypass`, `/autonomous`, `/model`, `/fast`, `/theme`.
- Session: `/clear` (`/new`), `/continue`, `/fork [step]`, `/steps`, `/revert <step>`, `/resume`, `/ls`, `/rename`, `/rm-session`, `/export`.
- Workspace: `/workspace`, `/add-dir <path>`, `/undo-add-dir`, `/remove-dir`.
- Automation: `/btw <prompt>` (side question in a sidechain).
- Extensibility: `/hooks`, `/mcp`.
- Cloud: `/cloud`, `/handoff [task]` (local to cloud), `/pickup` (bring cloud PR branch back), `/repo`, `/platform`, `/open [web|desktop]`, `/ssh`, `/archive`.
- Utilities: `/compact`, `/context`, `/usage`, `/session-stats`, `/config`, `/bug`, `/login`, `/org`.

## 5. Local vs cloud

- Local by default. `--cloud` or `/cloud` switches to Devin Cloud.
- `/handoff` moves a local task to a cloud session; `/pickup` brings the result back.
- `devin connect` gives a cloud session access to the local machine. `devin ssh`/`forward` reach the cloud VM.

## 6. Subagents and multi-agent (official `cli/subagents.md`)

- Built-in profiles: `subagent_explore` (read-only, web search; default subagent model; cannot edit or fetch arbitrary URLs) and `subagent_general` (full tools foreground; only pre-approved tools in background; parent's model).
- Foreground: parent waits. Background: runs in parallel; unapproved tools are auto-denied. Ctrl+B backgrounds a foreground subagent. In the subagent panel, `f` foregrounds, `x` cancels.
- Interrupting the parent does not kill subagents; they park and resume on the next message.
- Cancelled, failed, or completed subagents can be resumed (resumed runs are always foreground).
- Nesting: off by default. A custom profile can set `max-nesting` (e.g. 3).
- No numeric concurrency limit stated.
- Each subagent is a separate session with its own context window and is billed separately.
- Disable: `"subagents_enabled": false` in `~/.config/devin/config.json` (org policy can override). Config key `agent.subagent_model` sets the default.
- Custom profiles (experimental): `.devin/agents/`, `.agents/agents/`, `~/.config/devin/agents/`; layout `<name>.md` or `<name>/AGENT.md`. Frontmatter: `name`, `description`, `model`, `allowed-tools` (alias `tools`; `ask_user_question` always withheld), `max-nesting`.
- Tools: `run_subagent`, `read_subagent`.

## 7. Fusion (the likely "DevInfusion" target)

- Cognition blog "Introducing Fusion in Devin Desktop & CLI", dated 2026.09.11 on the page. Search snippets instead say Fusion was announced 2026-06-29 as a Devin Cloud preview. **Dates conflict; verify before citing.**
- Pattern: two models per session. **Lead** = frontier model for planning and review. **Sidekick** = cheaper execution model (explores, implements, runs tests). They exchange briefs, results, and feedback, not full conversations, so the lead's context stays cached.
- Recommended pairing on the page: Opus 5.5 lead + GPT-6 Luna sidekick. Other named models: Fable 5.1, Astra, SWE-2, GPT-5.6 Luna, Sonnet 5.5, GPT-6.1 Sol.
- Claims: "up to 39% more efficient" than other harnesses (Cognition's own benchmark); ~35% cost cut on its FrontierCode benchmark (self-reported).
- The lead/sidekick split is a model-routing pattern on top of subagents, not a separate protocol.

## 8. Rules, skills, plugins, hooks, MCP (official `cli/extensibility.md`)

- Project config in `.devin/` at repo root:
  - `.devin/config.json` (permissions, imports); `config.local.json` (gitignored)
  - `.devin/mcp_config.json`; `mcp_config.local.json` (gitignored; personal servers and secrets)
  - `.devin/hooks.v1.json` (lifecycle hooks, Claude Code compatible)
  - `.devin/skills/<name>/SKILL.md`
  - `.devin/agents/...` (subagent profiles)
  - `AGENTS.md` at root = rules
- User config: `~/.config/devin/` (Windows `%APPDATA%\devin\`).
- Imports (`read_config_from`, on by default; keys `cursor`, `windsurf`, `claude`, `copilot`, `opencode`, `zed`): reads `AGENTS.md`, `AGENT.md`, `CLAUDE.md`, `.cursor/rules/*.md[c]`, `.windsurf/rules/*.md`, `.claude/` (commands, subagents, hooks), `.github/skills/`, `opencode.json` (MCP), `.zed/settings.json` (MCP).
- Plugins bundle skills, rules, hooks, MCP servers, subagents. Not related to Windsurf plugins.

## 9. ACP

- `devin acp` = ACP server over stdio. Credentials from `WINDSURF_API_KEY`, stored login, or the ACP `authenticate` request.
- Zed: install "Devin" from the ACP registry, pick in Threads Sidebar agent dropdown, authenticate with browser login. Model from CLI config `agent.model`.
- Slash commands advertised over ACP: `/login`, `/status`, `/ask`, `/plan`, `/compact`, `/mcp`, `/bug`. `/login` and `/logout` hidden when the host manages auth.
- Devin Desktop can also launch `devin acp` as a third-party agent (Agent Command Center).
- Not verified: other editors beyond Zed/Desktop; plan availability.

## 10. Sandbox (official `cli/sandbox.md`)

- `--sandbox` gives OS-level isolation. Writable paths = granted `Write(...)` scopes + workspace; everything else read-only. `Read(...)` deny hides paths.
- Fail-closed: if sandbox setup fails, the CLI refuses to start rather than running unsandboxed.
- Linux needs `bubblewrap` (`bwrap`) and `socat`. Windows unsupported.
- Network filtering (marked unstable): `sandbox.allowed_domains`, `denied_domains`, `network_mode` (`full` default, or `limited` = GET/HEAD/OPTIONS only). Domain forms: `example.com`, `*.example.com`, `**.example.com`.
- `sandbox.excluded` takes `allow`/`ask`/`deny` `Exec(...)` rules. Commands in the persistent PTY shell always stay sandboxed.
- Enterprise enforcement: `Optional` (default) or `Required`; a `Strict` mode is mentioned as future.

## 11. Open items / unverified

- Official sub-pages not read: `cli/models.md`, MCP configuration, hooks, skills, plugins, `cli/cloud.md`, `reference/permissions.md`, Outposts reference, `read-config-from`.
- Fusion date conflict (2026-06-29 vs 2026-09-11).
- Third-party claims not checked against Cognition: Devin Local rewritten in Rust; "30% more token-efficient"; Agent Command Center Kanban.
- Fast.io and Daytona pages (Outposts/sandbox details) are vendor blogs; not used as authority.

## Sources

- https://docs.devin.ai/cli.md
- https://docs.devin.ai/cli/reference/commands.md
- https://docs.devin.ai/cli/extensibility.md
- https://docs.devin.ai/cli/subagents.md
- https://docs.devin.ai/cli/sandbox.md
- https://docs.devin.ai/cli/ssh.md
- https://docs.devin.ai/cli/acp/zed
- https://cognition.com/blog/local-fusion
- https://docs.devin.ai/onboard-devin/repo-setup (search hit, not read)
- https://www.therundown.ai/tools/devin-fusion (search snippet, third party)
- https://www.eesel.ai/blog/devin-fusion (search snippet, third party)
