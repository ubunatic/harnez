#!/usr/bin/env bash
set -euo pipefail

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}

command -v voxtype >/dev/null || fail "voxtype is required; this canary does not install it"
test "${XDG_SESSION_TYPE:-}" = "wayland" || fail "a Wayland session is required"

config="${HOME}/.config/voxtype/config-streaming.toml"
test -f "${config}" || fail "missing ${config}; see docs/VoiceInput.md for setup"

model_dir="${HOME}/.local/share/voxtype/models/parakeet-unified-en-0.6b"
test -d "${model_dir}" \
   || fail "missing ${model_dir}; run: voxtype setup --download --model parakeet-unified-en-0.6b --quiet"

printf '%s\n' \
       "This is a guided manual canary for OPT-IN local streaming (Parakeet)." \
       "It does NOT touch your default batch base.en config or voxtype.service." \
       "" \
       "1. Stop the batch daemon so the streaming daemon can bind the runtime socket:" \
       "     systemctl --user stop voxtype.service" \
       "2. Start the streaming daemon in this terminal (Ctrl-C to stop it later):" \
       "     voxtype -v -c ${config} daemon" \
       "   (PATH must include ~/.local/bin for the dotoolc fast path -- see" \
       "   ~/.config/systemd/user/voxtype.service's Environment=PATH= line if unsure.)" \
       "3. In another terminal, focus a disposable text editor, then run:" \
       "     voxtype record toggle" \
       "   speak, then run 'voxtype record toggle' again to stop." \
       "4. Confirm words appear incrementally (not all at once at the end) and that" \
       "   'Text typed via dotoolc' (not 'Text copied to clipboard') shows in the" \
       "   daemon's -v log for each partial." \
       "5. When done: Ctrl-C the streaming daemon, then:" \
       "     systemctl --user start voxtype.service" \
       "   to restore the default batch flow, and confirm it still works." \
       "" \
       "No package, service, or desktop setting is changed by this script."
