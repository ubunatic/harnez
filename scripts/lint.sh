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
    then
        echo "lint: unregistered command file: ${f}"
        fail=1
    fi
done

test "${fail}" -eq 0 && echo "lint: ok"
exit "${fail}"
