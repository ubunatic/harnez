# Commands Pipeline

How slash commands get from `config.yaml` → `~/.claude/commands/<name>.md`.

## Registration requirement

Dropping a `.md` file into `commands/` is not enough. The command must be declared in
`config.yaml` under the `commands:` list, or `ApplyAll` will never touch it.

Two forms:

```yaml
commands:
  # inline content
  - name: standup
    description: "Summarize today's git activity"
    content: |
      Run `git log --since=midnight --oneline` and summarize what was done.

  # file reference — content lives in commands/<name>.md in the repo
  - name: evergreen
    description: "Create/update evergreen docs and issue files for the current session"
    file: commands/evergreen.md
```

`file:` is resolved relative to the config dir via `fs.ReadFile` on `cfg.FS` (the embedded
or OS filesystem). It overrides `content` if both are set.

## Apply flow

1. `ApplyAll` creates `~/.claude/commands/` if absent.
2. For each entry: generate content → compare with existing file → write if changed.
3. Front-matter (`description:`) is prepended as YAML if `cmd.Description != ""`:
   ```
   ---
   description: "..."
   ---
   <body>
   ```
4. All command names are listed in the apply summary under `commands:` regardless of
   whether they were changed in this run (bug prior to 2026-06-23: only unchanged
   commands appeared — see issue #008).

## Discovery pitfall

`make status` shows `missing` for a command that is registered but not yet applied.
`make apply` installs it. The apply summary `commands:` line is the ground truth for
what is currently installed.

## Adding a new command

1. Write `commands/<name>.md` in the repo.
2. Add an entry to `config.yaml` → `commands:` with `file: commands/<name>.md`.
3. Run `make apply`.
4. Verify with `make status` — the name should appear in the `commands:` line.
