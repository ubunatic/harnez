#!/usr/bin/env bash
# Check that every commands/*.md has a matching entry in config.yaml.
# Run from the project root: scripts/lint.sh

set -euo pipefail

config="config.yaml"
fail=0

# Extract names declared under the commands: block only (stops at next top-level key).
cmd_names=$(awk '
    /^commands:/ { in_block=1; next }
    in_block && /^[a-z]/ { exit }
    in_block && /- name:/ { print $3 }
' "${config}")

# Every commands/*.md must be registered in config.yaml.
# (The reverse — every file: entry must exist — is enforced by apply itself.)
for f in commands/*.md
do
    name=$(basename "$f" .md)
    if ! echo "${cmd_names}" | grep -qx "${name}"
    then echo "lint: unregistered command file: ${f}"
         fail=1
    fi
done

# Every docs/commands/*.md source must be registered as a skill file or
# supporting resource. This keeps embedded command categories discoverable.
for f in docs/commands/*.md
do
    registered=0
    if grep -Fq "file: ${f}" "${config}"
    then registered=1
    elif grep -Fq "source: ${f}" "${config}"
    then registered=1
    fi
    if test "${registered}" -eq 0
    then echo "lint: unregistered docs command source: ${f}"
         fail=1
    fi
done

# Check JavaScript syntax for GNOME extensions if node is present
for js in contrib/*/*.js
do
    if test -f "${js}"
    then if ! node -c "${js}"
         then echo "lint: JS syntax error in ${js}"
              fail=1
         fi
    fi
done

# Check GNOME extension metadata and package validity if directory exists
if command -v gnome-extensions >/dev/null 2>&1 && test -d contrib/gnome-shell-extension
then if ! gnome-extensions pack --force --out-dir=/tmp contrib/gnome-shell-extension >/dev/null 2>&1
     then echo "lint: gnome-extensions pack validation failed"
          fail=1
     fi
fi

test "${fail}" -eq 0 && echo "lint: ok"
exit "${fail}"
