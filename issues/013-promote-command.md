# promote command — push improved project docs back to source

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
