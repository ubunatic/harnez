<!-- harnez:variant=lite -->
# Bash Rules (Lite)

House style. It overrides common Bash habits; where it is silent, write normal careful Bash.
Rationale and a do/don't table: `harnez docs variant bash full`.

## 1. Header

```bash
#!/usr/bin/env bash
set -euo pipefail
```

## 2. Conditionals: `if test`, never brackets (top rule)

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

- Never `[ ... ]` or `[[ ... ]]`; `test` is a command like any other.
- Never `; then` or `; do`: break the line. Avoid `;` everywhere else too.
- The first command sits on the `then`/`else`/`do` line and later commands align under it. No `then` alone on a line.
- No `a && b || c` as control flow; write the 4-line `if-then-else-fi`.
- Aim for 3-line `if` blocks and loops; move a longer body into a function.

## 3. Continuation: align, no fixed indent

```bash
result=$(some_command |
         grep "pattern" |
         awk '{print $2}')
```

- Break after `|`, `&&` and `||`, then align under the start of the chain.
- Function bodies indent by 3. Alignment wins over the fixed indent.

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

- `local out` first, then assign: `local out=$(cmd)` hides the exit code.
- `printf` over `echo` for variables. Errors and logs go to stderr.
- Functions return status (`return 0`/`return 1`), never printed booleans. Define them before the first call.
- `source file`, never `. file`.

## 5. Commands & traps

- `command -v tool`, not `which`.
- Temp files: `tmp=$(mktemp)` plus `trap 'rm -f "$tmp"' EXIT`.
- Wrap a command of uncertain duration that has no timeout of its own (network probe, lock wait, external call): `timeout 30 curl -sf URL`. Size the seconds per command. Exit 124 means `timeout` killed it.

## 6. Directory scoping: `-C` over `cd`

The shell's cwd persists across tool calls and may be the user's own terminal, so a stray `cd` silently retargets later calls.

- `git -C DIR`, `make -C DIR`, `go -C DIR`, `npm --prefix DIR`, `cargo --manifest-path DIR/Cargo.toml`.
- No such flag: subshell, `(cd DIR && tool)`.
- Never a bare `cd` that carries into a later call. If one is unavoidable, `cd` back before the call ends.

## 7. Background tasks & `HTO=0`

Set `HTO=0` on every background task or job (`HTO=0 make test-q1`, `HTO=0 harnez ...`); otherwise `harnez exec` kills it after 60 seconds. Then wait for the completion notification: no polling, no `sleep` loops.

## 8. Awk portability

The default `awk` is mawk, not gawk: no 3-argument `match()`, no `strtonum()`, no `gensub()`.

<!-- harnez:stop -->
