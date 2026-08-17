#!/usr/bin/env bash
set -euo pipefail

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}

command -v voxtype >/dev/null || fail "voxtype is required; this canary does not install it"
command -v eitype >/dev/null || fail "eitype is required; this canary does not install it"
test "${XDG_SESSION_TYPE:-}" = "wayland" || fail "a Wayland session is required"

printf '%s\n' \
       "This is a guided manual canary; it does not start recording or verify text." \
       "1. Focus a disposable text editor." \
       "2. Use the configured GNOME voice-input shortcut and dictate Unicode text." \
       "3. Confirm the text arrives, then return here and press Ctrl-C." \
       "The status stream below runs until Ctrl-C." \
       "No package, service, or desktop setting is changed by this script."
printf 'Press Enter to start the status stream, or Ctrl-C to stop: '
read -r _
voxtype status --follow
