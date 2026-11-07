#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git -C "$(dirname -- "${BASH_SOURCE[0]}")/.." rev-parse --show-toplevel)
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT

private="$tmp/private space"
mkdir -m 700 -- "$private"
mkdir -m 700 -- "$tmp/bin" "$tmp/hostile home" "$tmp/gobin" "$tmp/data"
mkdir -m 700 -- "$tmp/hostile home/.local" "$tmp/hostile home/.local/share" "$tmp/hostile home/.local/share/man"
printf 'sentinel\n' > "$tmp/gobin/keep"
printf 'sentinel\n' > "$tmp/data/keep"
printf 'sentinel\n' > "$tmp/hostile home/.local/share/man/keep"

cat > "$tmp/bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
if test "${FAKE_GO_FAIL:-}" = 1
then exit 41
fi
output=
while test "$#" -gt 0
do
   if test "$1" = -o
   then output=$2
        shift 2
   else shift
   fi
done
cat > "$output" <<'STAGED'
#!/usr/bin/env bash
set -euo pipefail
if test "${FAKE_MAN_FAIL:-}" = 1
then exit 42
fi
if test "$1" != man || test "$2" != --dir
then exit 43
fi
printf 'generated man page\n' > "$3/harnez.1"
STAGED
chmod 755 "$output"
GO
chmod 755 "$tmp/bin/go"

stage="$private/stage with spaces"
PATH="$tmp/bin:$PATH" HOME="$tmp/hostile home" GOBIN="$tmp/gobin" GOPATH="$tmp/hostile home/go" XDG_DATA_HOME="$tmp/data" bash "$repo_root/scripts/stage-install.sh" "$stage"
test -x "$stage/bin/harnez"
test -f "$stage/share/man/man1/harnez.1"
test "$(cat "$tmp/gobin/keep")" = sentinel
test "$(cat "$tmp/data/keep")" = sentinel
test "$(cat "$tmp/hostile home/.local/share/man/keep")" = sentinel
test ! -e "$tmp/hostile home/go/bin/harnez"

expect_rejected() {
   label=$1
   shift
   if "$@" > /dev/null 2>&1
   then printf 'FAIL: accepted %s\n' "$label" >&2
        exit 1
   fi
}

expect_rejected relative destination bash "$repo_root/scripts/stage-install.sh" relative/path
expect_rejected unclean destination bash "$repo_root/scripts/stage-install.sh" "$private/../escape"
expect_rejected existing destination bash "$repo_root/scripts/stage-install.sh" "$stage"
ln -s "$private" "$tmp/link"
expect_rejected symlink parent bash "$repo_root/scripts/stage-install.sh" "$tmp/link/stage"
mkdir -m 755 -- "$tmp/permissive"
expect_rejected permissive parent bash "$repo_root/scripts/stage-install.sh" "$tmp/permissive/stage"

build_failure="$private/build failure"
if PATH="$tmp/bin:$PATH" FAKE_GO_FAIL=1 bash "$repo_root/scripts/stage-install.sh" "$build_failure" > /dev/null 2>&1
then printf 'FAIL: build failure unexpectedly succeeded\n' >&2
     exit 1
fi
test ! -e "$build_failure"

man_failure="$private/man failure"
if PATH="$tmp/bin:$PATH" FAKE_MAN_FAIL=1 bash "$repo_root/scripts/stage-install.sh" "$man_failure" > /dev/null 2>&1
then printf 'FAIL: man failure unexpectedly succeeded\n' >&2
     exit 1
fi
test ! -e "$man_failure"

printf 'PASS: stage installer validation, isolation and failure cleanup\n'
