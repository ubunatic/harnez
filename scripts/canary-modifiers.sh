#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

printf '%s\n' \
    "================================================================" \
    "  CANARY: Physical Modifier Key Probe" \
    "================================================================" \
    "This canary probes physical keyboard modifier states directly" \
    "via Linux evdev ioctl (EVIOCGKEY)." \
    ""

cd "${repo_root}"

if test "${EUID}" -ne 0
then
    if test -r /dev/input/event0
    then
        go run ./scripts/canary_modifiers "$@"
    else
        printf '%s\n' \
            "Note: Direct evdev access usually requires 'input' group or sudo." \
            "Attempting unprivileged run..." \
            ""
        go run ./scripts/canary_modifiers "$@" || true
    fi
else
    go run ./scripts/canary_modifiers "$@"
fi
