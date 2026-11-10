## Summary
Checked 10 statements. 5 hold, 5 are wrong or out of date, and 0 could not be verified.

## Wrong or out of date
`docs/CLIDesign.md:146`: "`apply` is global-only; `init -l <lang>` owns all project setup."
Today: `init` has no `-l` flag; it uses `--docs` or `-d` for docs setup (`cmd/harnez/init.go:78`).
Correction: Change `init -l <lang>` to `init --docs <lang>`.

`docs/CLIDesign.md:156`: `harnez init [-d <dir>] [--docs <name>...] [--issues-git[=false]] [--force]`
Today: Missing options. `harnez init --help` shows additional flags including `--all` (`cmd/harnez/init.go:76`).
Correction: Update the usage string to include missing flags or note their existence.

`docs/CLIDesign.md:239`: `harnez apply [-t <dir>] [-d <name|profile>...] [--clean-docs] [--force-docs] [--debloat | --debloat-preset <minimal|aggressive>] [--codex-target <file>]`
Today: Missing options. `harnez apply --help` shows additional flags including `--systemd` (`cmd/harnez/apply_cmd.go:200`).
Correction: Update the usage string to include missing flags or note their existence.

`docs/CLIDesign.md:253`: "if --clean-docs / --no-docs: prune unmanaged docs from ~/.claude/docs/ and ~/.prime/agent/docs/"
Today: `harnez apply --help` specifies `--clean-docs` removes unmanaged global docs and `--no-docs` removes any previously installed global docs. Also `--remove-docs` removes all global docs (`cmd/harnez/apply_cmd.go:196`). The document is inaccurate.
Correction: Differentiate `--clean-docs`, `--no-docs`, and `--remove-docs`.

`docs/CLIDesign.md:275`: "`--debloat-disable-claude-ai-connectors` flag also remains available."
Today: The flag exists, but many more `--debloat-*` flags were added like `--debloat-cron` (`cmd/harnez/apply_cmd.go:209`).
Correction: List the other `--debloat-*` flags alongside `--debloat-disable-claude-ai-connectors`.

## Missing
- `--remove-docs` on `apply`
- `--components` on `apply`
- `--agy-target` on `apply`
- `--repo-mode` on `init`
- `--quota-1` on `init`

## Verified
`docs/CLIDesign.md:264`: `harnez apply --debloat` selects `minimal`; `harnez apply --debloat-preset minimal` or `--debloat-preset aggressive` selects the preset explicitly. (`cmd/harnez/apply_cmd.go:204`)
`docs/CLIDesign.md:367`: `apply --docs` has shorthand `-d`. (`cmd/harnez/apply_cmd.go:194`)
`docs/CLIDesign.md:367`: `init --docs` does not have shorthand `-d`, `-d` is taken by `--dir`. (`cmd/harnez/init.go:78`)
`docs/CLIDesign.md:371`: `-t <dir>` redirects the Claude config directory. (`cmd/harnez/apply_cmd.go:190`)
`docs/CLIDesign.md:373`: `--codex-target <file>` redirects that file for apply, status, and revert. (`cmd/harnez/apply_cmd.go:192`)
