#!/usr/bin/env bash
set -euo pipefail

# Static/non-audio checks for the issue 248 manual canary.

canary="scripts/canary-gnome-mic-indicator.sh"
fake_bin=$(mktemp -d /tmp/harnez-mic-static-XXXXXX)
fake_log="$fake_bin/invoked"
trap 'rm -rf "$fake_bin"' EXIT

for command_name in parec pactl timeout
do printf '#!/usr/bin/env bash\nprintf "%%s\\n" "$0" >> "$CANARY_FAKE_LOG"\nexit 99\n' > "$fake_bin/$command_name"
   chmod +x "$fake_bin/$command_name"
done

bash -n "$canary"
CANARY_FAKE_LOG="$fake_log" PATH="$fake_bin:$PATH" bash "$canary" --help | grep -F 'control' >/dev/null

if CANARY_FAKE_LOG="$fake_log" PATH="$fake_bin:$PATH" bash "$canary" >/dev/null 2>&1
then printf 'ERROR: no-argument invocation unexpectedly succeeded\n' >&2
     exit 1
fi

if CANARY_FAKE_LOG="$fake_log" PATH="$fake_bin:$PATH" bash "$canary" unknown >/dev/null 2>&1
then printf 'ERROR: unknown phase unexpectedly succeeded\n' >&2
     exit 1
fi

if CANARY_FAKE_LOG="$fake_log" PATH="$fake_bin:$PATH" bash "$canary" control 2 >/dev/null 2>&1
then printf 'ERROR: invalid duration unexpectedly succeeded\n' >&2
     exit 1
fi

if test -e "$fake_log"
then printf 'ERROR: a capture dependency was invoked during static checks\n' >&2
     exit 1
fi

grep -F 'application.id=org.gnome.VolumeControl' "$canary" >/dev/null
grep -F 'source_output "$exempt_id"' "$canary" >/dev/null

printf 'mic indicator canary static checks: ok\n'
