#!/usr/bin/env bash
set -euo pipefail
shopt -s dotglob nullglob

test_platform=${HARNEZ_STAGE_TEST_PLATFORM:-$(uname -s)}
if test "$test_platform" != Linux
then printf 'SKIP: staging tests require Linux (found %s)\n' "$test_platform"
     exit 0
fi

if test "${HARNEZ_STAGE_TEST_COREUTILS:-}" = unsupported
then printf 'SKIP: staging tests require GNU or uutils coreutils realpath/stat\n'
     exit 0
fi

realpath_version=$(realpath --version 2>/dev/null || true)
stat_version=$(stat --version 2>/dev/null || true)
if ! printf '%s\n' "$realpath_version" | grep -Eq '(GNU|uutils) coreutils' || ! printf '%s\n' "$stat_version" | grep -Eq '(GNU|uutils) coreutils'
then printf 'SKIP: staging tests require GNU or uutils coreutils realpath/stat\n'
     exit 0
fi

repo_root=$(git -C "$(dirname -- "${BASH_SOURCE[0]}")/.." rev-parse --show-toplevel)
tmp=$(mktemp -d)
shell_marker=".stage-install-shell-marker-$$"
trap 'rm -rf -- "$tmp"; rm -f -- "$repo_root/$shell_marker"' EXIT

platform_skip=$(HARNEZ_STAGE_TEST_PLATFORM=Darwin bash "${BASH_SOURCE[0]}")
test "$platform_skip" = 'SKIP: staging tests require Linux (found Darwin)'
coreutils_skip=$(HARNEZ_STAGE_TEST_COREUTILS=unsupported bash "${BASH_SOURCE[0]}")
test "$coreutils_skip" = 'SKIP: staging tests require GNU or uutils coreutils realpath/stat'

private="$tmp/private space"
mkdir -m 700 -- "$private"
mkdir -m 700 -- "$tmp/bin" "$tmp/hostile home" "$tmp/gobin" "$tmp/data"
mkdir -m 700 -- "$tmp/hostile home/.local" "$tmp/hostile home/.local/share" "$tmp/hostile home/.local/share/man"
mkdir -m 700 -- "$tmp/hostile home/go" "$tmp/hostile home/go/bin"
printf 'sentinel\n' > "$tmp/gobin/keep"
printf 'sentinel\n' > "$tmp/data/keep"
printf 'sentinel\n' > "$tmp/hostile home/.local/share/man/keep"
printf 'sentinel\n' > "$tmp/hostile home/go/bin/keep"
printf 'alias sentinel\n' > "$tmp/gobin/⚙"

cat > "$tmp/bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
if test "${FAKE_GO_FAIL:-}" = 1
then printf 'build-attempted\n' >> "$FAKE_LOG"
     exit 41
fi
printf 'build-attempted\n' >> "$FAKE_LOG"
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
then printf 'man-attempted:%s\n' "$3" >> "$FAKE_LOG"
     exit 42
fi
if test "$1" != man || test "$2" != --dir
then exit 43
fi
printf 'generated man page\n' > "$3/harnez.1"
printf 'man-attempted:%s\n' "$3" >> "$FAKE_LOG"
STAGED
chmod 755 "$output"
GO
chmod 755 "$tmp/bin/go"

stage="$private/stage with spaces"
export FAKE_LOG="$tmp/fake.log"
PATH="$tmp/bin:$PATH" HOME="$tmp/hostile home" GOBIN="$tmp/gobin" GOPATH="$tmp/hostile home/go" XDG_DATA_HOME="$tmp/data" bash "$repo_root/scripts/stage-install.sh" "$stage"
test -x "$stage/bin/harnez"
test -f "$stage/share/man/man1/harnez.1"
test "$(cat "$FAKE_LOG")" = "$(printf 'build-attempted\nman-attempted:%s' "$stage/share/man/man1")"

assert_shared_contents() {
   for dir in "$tmp/gobin" "$tmp/data" "$tmp/hostile home/.local/share/man" "$tmp/hostile home/go/bin"
   do
      for entry in "$dir"/*
      do
         if ! test -f "$entry" || test -L "$entry"
         then printf 'FAIL: unexpected non-regular shared entry: %s\n' "$entry" >&2
              exit 1
         fi
         entry_name=${entry##*/}
         if test "$dir" = "$tmp/gobin"
         then if test "$entry_name" != keep && test "$entry_name" != ⚙
              then printf 'FAIL: unexpected GOBIN entry: %s\n' "$entry" >&2
                   exit 1
              fi
         elif test "$entry_name" != keep
         then printf 'FAIL: unexpected shared entry: %s\n' "$entry" >&2
              exit 1
         fi
      done
   done
   test -f "$tmp/gobin/keep" && ! test -L "$tmp/gobin/keep"
   test -f "$tmp/gobin/⚙" && ! test -L "$tmp/gobin/⚙"
   test "$(cat "$tmp/gobin/⚙")" = 'alias sentinel'
   test "$(cat "$tmp/gobin/keep")" = sentinel
   test "$(cat "$tmp/data/keep")" = sentinel
   test "$(cat "$tmp/hostile home/.local/share/man/keep")" = sentinel
   test "$(cat "$tmp/hostile home/go/bin/keep")" = sentinel
}
assert_shared_contents

printf 'hidden unexpected entry\n' > "$tmp/gobin/.unexpected"
if output=$(assert_shared_contents 2>&1)
then printf 'FAIL: shared destination check ignored a hidden file\n' >&2
     exit 1
else status=$?
fi
test "$status" -eq 1
printf '%s\n' "$output" | grep -Fq "FAIL: unexpected GOBIN entry: $tmp/gobin/.unexpected"
rm -- "$tmp/gobin/.unexpected"
assert_shared_contents

tick='`'
dollar='$'
hostile_stage="$private/stage 'single quote' ${tick}backtick${tick} ${dollar}(touch $shell_marker)"
PATH="$tmp/bin:$PATH" HOME="$tmp/hostile home" GOBIN="$tmp/gobin" GOPATH="$tmp/hostile home/go" XDG_DATA_HOME="$tmp/data" bash "$repo_root/scripts/stage-install.sh" "$hostile_stage" >/dev/null
test -x "$hostile_stage/bin/harnez"
test -f "$hostile_stage/share/man/man1/harnez.1"
test ! -e "$repo_root/$shell_marker"
assert_shared_contents

expect_rejected() {
   label=$1
   shift
   if "$@" > /dev/null 2>&1
   then printf 'FAIL: accepted %s\n' "$label" >&2
        exit 1
   fi
}

unsupported_bin="$tmp/unsupported-bin"
mkdir -m 700 -- "$unsupported_bin"
cat > "$unsupported_bin/uname" <<'UNAME'
#!/usr/bin/env bash
printf 'Darwin\n'
UNAME
chmod 755 "$unsupported_bin/uname"
if output=$(PATH="$unsupported_bin:$PATH" bash "$repo_root/scripts/stage-install.sh" "$private/unsupported" 2>&1)
then printf 'FAIL: installer accepted unsupported platform\n' >&2
     exit 1
else status=$?
fi
test "$status" -eq 2
printf '%s\n' "$output" | grep -q 'requires Linux and GNU or uutils coreutils'

unsupported_bin="$tmp/unsupported-coreutils-bin"
mkdir -m 700 -- "$unsupported_bin"
cat > "$unsupported_bin/uname" <<'UNAME'
#!/usr/bin/env bash
printf 'Linux\n'
UNAME
cat > "$unsupported_bin/realpath" <<'UNSUPPORTED'
#!/usr/bin/env bash
printf 'unsupported realpath\n'
UNSUPPORTED
cat > "$unsupported_bin/stat" <<'UNSUPPORTED'
#!/usr/bin/env bash
printf 'unsupported stat\n'
UNSUPPORTED
chmod 755 "$unsupported_bin/uname" "$unsupported_bin/realpath" "$unsupported_bin/stat"
if output=$(PATH="$unsupported_bin:$PATH" bash "$repo_root/scripts/stage-install.sh" "$private/unsupported" 2>&1)
then printf 'FAIL: installer accepted unsupported coreutils\n' >&2
     exit 1
else status=$?
fi
test "$status" -eq 2
printf '%s\n' "$output" | grep -q 'requires Linux and GNU or uutils coreutils'

expect_rejected relative destination bash "$repo_root/scripts/stage-install.sh" relative/path
expect_rejected unclean destination bash "$repo_root/scripts/stage-install.sh" "$private/../escape"
expect_rejected existing destination bash "$repo_root/scripts/stage-install.sh" "$stage"
ln -s "$private" "$tmp/link"
expect_rejected symlink parent bash "$repo_root/scripts/stage-install.sh" "$tmp/link/stage"
mkdir -m 755 -- "$tmp/permissive"
expect_rejected permissive parent bash "$repo_root/scripts/stage-install.sh" "$tmp/permissive/stage"

build_failure="$private/build failure"
printf '' > "$FAKE_LOG"
if PATH="$tmp/bin:$PATH" FAKE_GO_FAIL=1 bash "$repo_root/scripts/stage-install.sh" "$build_failure" > /dev/null 2>&1
then printf 'FAIL: build failure unexpectedly succeeded\n' >&2
     exit 1
else status=$?
fi
test "$status" -eq 41
test "$(cat "$FAKE_LOG")" = build-attempted
test ! -e "$build_failure"

man_failure="$private/man failure"
printf '' > "$FAKE_LOG"
if PATH="$tmp/bin:$PATH" FAKE_MAN_FAIL=1 bash "$repo_root/scripts/stage-install.sh" "$man_failure" > /dev/null 2>&1
then printf 'FAIL: man failure unexpectedly succeeded\n' >&2
     exit 1
else status=$?
fi
test "$status" -eq 42
test "$(cat "$FAKE_LOG")" = "$(printf 'build-attempted\nman-attempted:%s' "$man_failure/share/man/man1")"
test ! -e "$man_failure"

printf 'PASS: stage installer validation, isolation and failure cleanup\n'
