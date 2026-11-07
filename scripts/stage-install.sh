#!/usr/bin/env bash
# Build Harnez and its man pages into one new private session directory.
set -euo pipefail

destination="${1:?Usage: scripts/stage-install.sh ABSOLUTE_NEW_DESTINATION}"

parent=$(dirname -- "$destination")
name=$(basename -- "$destination")
destination_clean=$(realpath -m -- "$destination")
if test "$destination" = "${destination#/}" || test "$destination_clean" != "$destination"
then printf 'ERROR: destination must be an absolute clean path\n' >&2
     exit 2
fi

if test -z "$name" || test "$name" = . || test "$name" = ..
then printf 'ERROR: destination must name a new directory\n' >&2
     exit 2
fi

parent_real=$(realpath -e -- "$parent")
if test "$parent_real" != "$parent" || ! test -d "$parent" || test -L "$parent"
then printf 'ERROR: parent must be a real directory without symlink components\n' >&2
     exit 2
fi

parent_mode=$(stat -c '%a' -- "$parent")
if test "$parent_mode" != 700
then printf 'ERROR: parent directory must have mode 0700\n' >&2
     exit 2
fi

if test -e "$destination" || test -L "$destination"
then printf 'ERROR: destination already exists\n' >&2
     exit 2
fi

if ! mkdir -m 700 -- "$destination"
then printf 'ERROR: cannot create destination %s\n' "$destination" >&2
     exit 1
fi

cleanup() {
   result=$?
   if test "$result" -ne 0
   then rm -rf -- "$destination"
   fi
   trap - EXIT
   exit "$result"
}
trap cleanup EXIT

repo_root=$(git -C "$(dirname -- "${BASH_SOURCE[0]}")/.." rev-parse --show-toplevel)
mkdir -m 755 -- "$destination/bin" "$destination/share" "$destination/share/man"
mkdir -m 755 -- "$destination/share/man/man1"

go build -C "$repo_root" -ldflags "${VERSION_LDFLAGS:-}" -o "$destination/bin/harnez" ./cmd/harnez
"$destination/bin/harnez" man --dir "$destination/share/man/man1"

trap - EXIT
printf 'Staged Harnez binary and man pages in %s\n' "$destination"
