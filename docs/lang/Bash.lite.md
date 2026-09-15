<!-- harnez:variant=lite -->
# Bash Conventions (Lite)

## Header & Strict Mode

```bash
#!/usr/bin/env bash
set -euo pipefail   # -e: exit on any non-zero status
                    # -u: exit on unset variable reference
                    # -o pipefail: pipeline returns last non-zero status in the chain
```

## Sourcing — always `source`, never `.`

```bash
# ✅ explicit, greppable ("source " finds all inclusions), unambiguous vs ./script.sh
source ~/.bashrc
source ~/.zshrc
source "$script_dir/lib.sh"

# ❌ standalone dot: lost in whitespace, confusable with path prefixes, ungreppable
. ~/.bashrc
. "$script_dir/lib.sh"
```

## Conditionals — always `if test`, never `[[ ]]` or `[ ]`

Most important rule. Forget all legacy bracket usages.

```bash
# ✅ 3-line if-then-fi: 1st cmd on the `then` line, no semicolons
if test -f "$file"
then printf 'Found %s\n' "$file"
fi

# ✅ 4-line if-then-else-fi
if test "$a" = "$b"
then printf 'Equal\n'
else printf 'Not equal\n'
fi

# ✅ multi-command then block: 1st cmd on `then` line, rest aligned under it
if test -d "$dir"
then printf 'Entering %s\n' "$dir"
     process_dir "$dir"
fi

# ✅ while loop: 1st cmd on the `do` line
while test "$x" != "$y"
do process "$x"
done
```

Anti-patterns (the ✅ forms are the code block above):

| ❌ DON'T | Why |
|---|---|
| `if [ "$x" = "$y" ]; then` | Single brackets forbidden |
| `if [[ "$x" == "$y" ]]; then` | Double brackets forbidden |
| `if test "$x" = "$y"; then` | No semicolon before `then`/`do` — break the line |
| `if test …` / `then` (empty) / `  do_work` / `fi` | No dangling `then` line; 1st cmd goes on the `then` line |
| `. ~/.bashrc`, `. "$lib"` | Standalone `.` sourcing forbidden; use `source` |

## Variables & Local Scope

```bash
printf '%s\n' "$var" "${var}"              # always double-quote every expansion
pattern="${1:?Usage: script.sh PATTERN}"   # required-arg error pattern

f() {
   local name="$1"      # literal values: assign directly
   local result         # command substitutions: declare FIRST, then assign —
   result=$(command args)   # otherwise `local` masks the command's exit code
}
```

## Output Discipline

```bash
printf '%s\n' "$var"              # prefer printf over echo for variables
printf 'ERROR: %s\n' "$msg" >&2   # errors go to stderr

pass() {
   printf '  ✓ %s\n' "$*"
}

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}
```

## Line Breaks, Continuation & Indentation

No arbitrary fixed indentation for command blocks — use alignment continuation.

```bash
# long pipelines: break after `|`, align under the start of the chain
result=$(some_command |
         grep "pattern" |
         awk '{print $2}')

# long conditions: break after `&&`/`||`, align under the first test
if test -f "$a" &&
   test -d "$b"
then stmt1
     stmt2
else fail "not found"
fi

# then/else/do blocks: 1st cmd on the keyword line, subsequent cmds aligned
for item in "${array[@]}"
do process_item "$item" || fail "err"
   log_item "$item"
done

# function bodies: base indent level 3 inside { … };
# alignment continuation wins over fixed indents for conditionals
```

## Commands & Traps

```bash
command -v tool          # prefer over `which`
out=$(cmd 2>&1)          # capture output cleanly
cmd > file               # write
cmd >> file              # append
cmd 2>/dev/null          # suppress errors
tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT   # temp files always trap-cleaned

# wrap uncertain-duration commands with no client-side timeout of their own
# (network probe, lock wait, external service call):
timeout 30 curl -sf https://example.com/health
# size seconds per-command, not one fixed global value; timeout exits 124 on kill —
# branch on that distinctly from the wrapped command's own failure codes.
# Per-invocation only; for long-running background jobs see AgenticLoop.md
# "Blocking sleep Waits" / "Buffered Long-Running Output".
```

## Functions

```bash
# define before first invocation
f() {
   test -n "$1" || return 1   # status via return 0 / return 1 — never printed booleans
   return 0
}
```

## Directory Scoping — prefer `-C` over `cd`

Rule: if the command has a directory flag, use it — never `cd` purely for scoping. The shell tool's cwd persists across tool calls, so a stray `cd` silently retargets the *next* unrelated call (e.g. `git status` on the wrong repo) with no error.

| Command | Directory flag | Example |
|---------|---------------|---------|
| `git`   | `-C <dir>`    | `git -C ~/projects/foo status` |
| `make`  | `-C <dir>`    | `make -C ~/projects/foo test` |
| `go`    | `-C <dir>` (Go 1.20+) | `go -C ~/projects/foo build ./...` |
| `npm`   | `--prefix <dir>` | `npm --prefix ~/projects/foo install` |
| `cargo` | `--manifest-path <path>` | `cargo build --manifest-path ~/projects/foo/Cargo.toml` |

```bash
# no directory flag? keep cd + command in ONE call, prefer the subshell form
# ✅ subshell: cwd restored when it exits
(cd /some/dir && some-tool --flag)

# ⚠️ inline: cwd leaks through the rest of this call (but not into the next call)
cd /some/dir && some-tool --flag

# ❌ never a bare `cd` meant to carry into a LATER separate tool call
```

### Restore-cwd convention (shared shell environments)

Where the shell session is shared with the user's interactive terminal, an agent's `cd` outlives the tool call and moves the *human's* prompt too. Advisory only — not mechanically enforceable (issue 095); the shell reaches anywhere the OS user can, regardless of project scope.

```bash
# 1st choice: git -C / make -C, absolute paths, or (cd dir && cmd) — auto-restoring
# unavoidable bare cd (tool takes relative paths only)? capture and restore:
orig=$(pwd)
cd /some/dir
some-tool --relative-only-flag
cd "$orig"
```

See also: issue 095 (cwd-leaking incident, trust boundary), issue 222 (multi-repo wrong-repo failure from a stray `cd`).

## Appendix — Awk Portability

```awk
# Default awk on Debian/Ubuntu/Raspberry Pi OS is mawk, not gawk. No gawk extensions:
# ❌ match(str, /re/, arr)   3-arg form — use split() or sub()/gsub()
# ❌ strtonum("0xff")        — write a manual h2d() converter
# ❌ gensub()                — use sub()/gsub() with a temporary variable
```
