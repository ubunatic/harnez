#!/usr/bin/env bash
# Check that every docs/commands/*.md has a matching entry in config.yaml.
# Run from the project root: scripts/lint.sh

set -euo pipefail

config="config.yaml"
fail=0

# Every docs/commands/*.md source must be registered as a skill/command file or
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
