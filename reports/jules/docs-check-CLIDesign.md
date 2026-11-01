Taken from the last message of Jules session 13138884223905224117. The run wrote its report, then stopped to ask whether to finalize it, so no PR was opened and the report's line references are lost. Each item below was checked against `harnez init --help`, `harnez apply --help` and `docs/CLIDesign.md` at `c6832711`.

## Summary
10 statements checked: 5 hold and 5 are wrong or out of date.

## Wrong or out of date
1. `docs/CLIDesign.md:146` says `init -l <lang>`. That flag does not exist; it is `init --docs <lang>`.
2. The `init` usage block is missing options, for example `--all`, `--gowork` and `--wizard`.
3. The `apply` usage block is missing options, for example `--components`, `--no-docs`, `--remove-docs` and `--shell`.
4. `docs/CLIDesign.md:253` treats `--clean-docs` and `--no-docs` as the same pruning step, but `--no-docs` also stops installing docs, and `--remove-docs` was added.
5. `docs/CLIDesign.md:276` mentions `--debloat-disable-claude-ai-connectors` and misses the other `--debloat-*` flags.

## Missing
`--remove-docs`, `--components` and `--agy-target` on `apply`, and `--repo-mode` on `init`.

Correction from checking: Jules also listed `--quota-1` on `init`, but `docs/CLIDesign.md:160` and `:173` describe it. It is only absent from the usage line at `:156`. Likewise, item 2 is about the usage line: lines 188-191 do describe `--all` and `--wizard`.

## Verified
`--debloat` selecting presets, `-d` on `apply`, and `-t` and `--codex-target`.
