# Codex Settings Management

Harnez manages several Codex files through `harnez apply`. The Codex config
target is `codex_hooks_target` in `config.yaml` (normally
`~/.codex/config.toml`). `--codex-target <path>` overrides that file for
`apply`, `status`, and `revert`; `-t/--target` selects the Claude directory.

| Codex surface | Harnez source | When written |
|---|---|---|
| `config.toml` `[hooks.harnez]` | Native hook integration | Plain `harnez apply` |
| `config.toml` `[features]` | `debloat.codex_features` | `harnez apply --debloat` or an explicit debloat preset |
| `~/.codex/skills/<name>/` | `skills:` and `codex_skills_target` | Plain `harnez apply` |
| `~/.codex/AGENTS.md` | `agents_md.agents.codex` | Plain `harnez apply` when its parent exists |

## Debloat ownership

```sh
harnez apply --debloat
harnez status --debloat
harnez revert --debloat
```

`config.yaml` is the source of truth for the **complete Codex debloat set**:

```yaml
debloat:
  codex_features:
    apps: false
    plugins: false
```

The same Codex set applies to both the default `minimal` and explicit
`aggressive` debloat presets. Unlisted Codex features, skills, MCP servers,
hooks, and other settings are outside debloat. Plain `harnez apply` keeps
Codex feature values as they are. The two listed flags disable Codex app
integrations and plugins, respectively. See the [OpenAI Docs configuration
reference](https://learn.chatgpt.com/docs/config-file/config-reference) for
their current product meaning.

`harnez apply --debloat` records each listed key's original value or absence
in `config.toml.harnez-debloat.json`, beside the target config. Repeating apply
keeps that original value. Removing a key from `codex_features` and reapplying
restores its original value and ends Harnez ownership of that key. Removing
every key clears the Codex debloat record. `harnez revert --debloat` restores
the remaining recorded Codex values as well as Claude's recorded changes.
`harnez status --debloat` shows current values, preset values, and ownership;
it also identifies keys removed from the current spec but still awaiting
reapply. A missing Codex record is harmless when reverting an older
Claude-only debloat installation.

Harnez preserves the *values* of unlisted TOML settings when it merges the
config. Its TOML encoder may normalize formatting and remove comments in
`config.toml`, including during plain apply's hook merge.

## Measurement and safe checks

The retained [Codex context canary](../scripts/canary-codex-context/results.md)
measured the two flags together on Codex CLI 0.154.0: first-turn input in the
Harnez repo fell from a 17,511-token baseline mean to 14,635 tokens, a
2,876-token (16.4%) reduction. This is one minimal-prompt measurement; the
individual flag savings overlap and longer tasks may differ. The canary uses
per-run `-c` overrides and does not edit the live Codex config.

Use `harnez status --debloat` to inspect the live settings. For a write test,
`--codex-target` redirects the Codex config only. `harnez apply` also syncs
other global agent files and launcher links, so a disposable Codex target
alone does not isolate an entire apply run; see [CLIDesign.md](CLIDesign.md).
