# Claude Code Permission Model

## Two separate permission layers

Claude Code has two independent permission gates:

- **`Bash(...)`** — controls which shell commands Claude can run
- **`Read(...)`** — controls which paths Claude's built-in file-reading tool can access

These are evaluated independently. A `Read(//etc/**)` rule does not affect `Bash`, and a `Bash(grep *)` rule does not affect `Read`.

## Shell commands bypass Read path restrictions

Any shell command that reads files (`grep`, `cat`, `find`, `awk`, `sed`, etc.) operates at the OS level and is not subject to `Read` path rules. For example:

- `Bash(grep *)` permits `grep '' /etc/shadow` regardless of `Read` rules
- `Bash(cat *)` can read any file the shell user can access

This means: once you allow a general shell read command, your `Read` path allowlist becomes cosmetic from a security standpoint. On a personal machine this is usually fine — the main value of `Read` restrictions is limiting accidental broad reads by Claude's file tool, not enforcing hard security boundaries.

## Practical guidance

- Use `Read` path rules to scope Claude's file tool to relevant directories
- Use `Bash` rules to allow specific diagnostic/build commands
- Broad `Bash` globs (`grep *`, `cat *`, `find *`) implicitly grant read access to everything the shell user can see
- Narrow `Bash` rules (e.g. `Bash(sudo smartctl *)`) are meaningful because they limit a specific elevated-privilege command

## More Ideas for general (read) access
The following still needs assessment. 

### File Paths
- allow access to ~/claude or just ~/claude/settings.json, maybe add skills etc.
- allow access to .bashrc .userrc .zshrc in HOME
- allow access to git/*
- allow access to projects/*
- allow access to ~/Pictures/Screenhots
- allow access to other dirs in ~ -> do an assessment what is safe in a typical linux env
- allow access to .local  -> should not contain any secrets
- allow access to .config -> does it contain secrets usually?

### Commands
- tree
- pgrep

### Questions
- can we generally allow sed, grep, rg, ag, cat, and so on in any of the safe locations above
  or does Claude not support this?
