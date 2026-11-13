---
title: Bash Conventions
weight: 61
---

<!-- harnez:bundled -->
# Bash Conventions

House style for scripts, canaries, Make recipes and embedded shell. It overrides common Bash
habits; where it is silent, write normal careful Bash (quote expansions, check exit codes).
Reference material: grep it, don't read it.

## 1. Header

```bash
#!/usr/bin/env bash
set -euo pipefail
```

## 2. Conditionals: `if test`, never brackets (top rule)

`test` is a command like any other; use it after `if` and `while`. Blocks stay vertical: 3-line
`if-then-fi`, 4-line `if-then-else-fi`, 3-line loops.

```bash
if test -f "$file"
then printf 'found %s\n' "$file"
fi

if test "$a" = "$b" &&
   test -d "$dir"
then stmt1
     stmt2
else fail "not found"
fi

while test "$x" != "$y"
do step "$x"
done

for item in "${items[@]}"
do process_item "$item" || fail "item $item"
   log_item "$item"
done
```

| Don't | Do instead |
|---|---|
| `if [ "$x" = "$y" ]`, `if [[ "$x" == "$y" ]]` | `if test "$x" = "$y"` |
| `if test ...; then`, `for ...; do` | break the line before `then`/`do`; avoid `;` everywhere else too |
| `then` alone on a line, body indented below | first command on the `then`/`else`/`do` line, later commands aligned under it |
| `test ... && do_a \|\| do_b` | 4-line `if-then-else-fi`; logic operators are not control flow |
| long loop or branch bodies | a function called from a 3-line block |

Break or return on error inside loops: `errexit` does not reliably stop a loop body.

## 3. Continuation: align, no fixed indent

There is no general N-space indent. Break after `|`, `&&` and `||` and align under the start of
the chain. Ignore established indentation in older scripts when it conflicts with this.

```bash
result=$(some_command |
         grep "pattern" |
         awk '{print $2}')

process_item() {
   local item="$1"
   if test -n "$item" &&
      test -e "$item"
   then stmt1
        stmt2
   else stmt1
        stmt2
   fi

   log "item $item processed"
}
```

Function bodies indent by 3 spaces, never tabs. Alignment wins over the fixed indent, so statements after the first
one in a `then` block sit 5 columns in from the `if`.

## 4. Variables, functions, output

```bash
pattern="${1:?Usage: script.sh PATTERN}"

process_item() {
   local item="$1"
   local out
   out=$(tool "$item")
   printf '%s\n' "$out"
}

pass() {
   printf '  ✓ %s\n' "$*"
}

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}
```

- `local out` first, then assign: `local out=$(cmd)` returns the status of `local` and hides the command's exit code.
- `printf`, not `echo`. Errors and logs go to stderr.
- Functions return status (`return 0`/`return 1`), never printed booleans. Define them before the first call.
- `source file`, never `. file`: `source` is greppable and cannot be confused with `./script.sh` or lost in whitespace. This includes dotfiles (`source ~/.bashrc`).

## 5. Commands & traps

- `command -v tool`, not `which`.
- Temp files: `tmp=$(mktemp)` plus `trap 'rm -f "$tmp"' EXIT`.
- Wrap a command of uncertain duration that has no client-side timeout of its own (network probe,
  lock wait, external service call) instead of risking an indefinite hang:
  `timeout 30 curl -sf https://example.com/health`. Size the seconds to the command's expected
  duration, not one global value. `timeout` exits 124 when it kills the command; branch on that
  separately from the command's own exit codes. This covers single invocations. For waiting on a
  background job see "Blocking `sleep` Waits" and "Buffered Long-Running Output" in
  `docs/AgenticLoop.md`.

## 6. Directory scoping: `-C` over `cd`

If the command has a directory flag, use it; never `cd` only for scoping. The shell tool's cwd
persists across tool calls, so a `cd` in one call silently changes where the next unrelated call
runs (issue 222: `git status` reported on the wrong repo). Where the shell is shared with the
user's interactive terminal it also moves the human's prompt (issue 095).

| Command | Flag | Example |
|---|---|---|
| `git` | `-C <dir>` | `git -C ~/projects/foo status` |
| `make` | `-C <dir>` | `make -C ~/projects/foo test` |
| `go` | `-C <dir>` (Go 1.20+) | `go -C ~/projects/foo build ./...` |
| `npm` | `--prefix <dir>` | `npm --prefix ~/projects/foo install` |
| `cargo` | `--manifest-path <path>` | `cargo build --manifest-path ~/projects/foo/Cargo.toml` |

No directory flag: keep the `cd` and the command in one call, in a subshell, so the cwd is
restored.

```bash
(cd /some/dir && some-tool --flag)
```

A tool that only takes relative paths and cannot run in a subshell: capture and restore.

```bash
orig=$(pwd)
cd /some/dir
some-tool --relative-only-flag
cd "$orig"
```

Never issue a bare `cd` meant to carry into a later tool call. This is advisory and cannot be
enforced: the shell reaches anywhere the OS user can.

## 7. Background tasks & `HTO=0`

Set `HTO=0` on every command run as a background task or job (`HTO=0 make test-q1`,
`HTO=0 harnez agent wait <session>`); otherwise `harnez exec`'s default 60-second timeout kills
it. Then stop calling tools and wait for the host's completion notification: no status polling,
no `sleep` loops.

## 8. Awk portability

The default `awk` on Debian, Ubuntu and Raspberry Pi OS is mawk, not gawk. Avoid gawk extensions:

- 3-argument `match(str, /re/, arr)`: use `split()` or `sub()`/`gsub()`.
- `strtonum("0xff")`: write a manual `h2d()` function.
- `gensub()`: use `sub()`/`gsub()` with a temporary variable.
