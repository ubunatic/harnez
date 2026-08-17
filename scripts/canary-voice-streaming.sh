#!/usr/bin/env bash
set -euo pipefail

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}

command -v voxtype >/dev/null || fail "voxtype is required; this canary does not install it"
command -v harnez >/dev/null || fail "harnez is required; this canary does not install it"
test "${XDG_SESSION_TYPE:-}" = "wayland" || fail "a Wayland session is required"

config="${HOME}/.config/voxtype/config-streaming.toml"
test -f "${config}" || fail "missing ${config}; see docs/VoiceInput.md for setup"

model_dir="${HOME}/.local/share/voxtype/models/parakeet-unified-en-0.6b"
test -d "${model_dir}" \
   || fail "missing ${model_dir}; run: voxtype setup --download --model parakeet-unified-en-0.6b --quiet"

printf '%s\n' \
       "This is a guided manual canary for OPT-IN local streaming (Parakeet)." \
       "It does NOT touch your default batch base.en config by default." \
       "" \
       "Normal way to switch (see 'harnez tools voice-input mode --help'):" \
       "     harnez tools voice-input mode streaming   # switch to streaming" \
       "     harnez tools voice-input mode             # show the active mode" \
       "     harnez tools voice-input mode batch        # switch back" \
       "This stops/starts the voxtype.service / voxtype-streaming.service systemd" \
       "user units for you; it never leaves both stopped, and refuses to switch into" \
       "streaming mode if the config or model above are missing." \
       "" \
       "To exercise it end to end:" \
       "1. harnez tools voice-input mode streaming" \
       "2. Focus a disposable text editor, then use the configured GNOME shortcut" \
       "   (Super+Ctrl+X) to dictate. Confirm words appear incrementally (not all" \
       "   at once at the end)." \
       "3. journalctl --user -u voxtype-streaming.service -f shows 'Text typed via" \
       "   dotoolc' (not 'Text copied to clipboard') for each partial." \
       "4. harnez tools voice-input mode batch, then confirm the batch flow still" \
       "   works too." \
       "" \
       "This script itself only reads state; the mutations above come from the" \
       "harnez commands you choose to run."
