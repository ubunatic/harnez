#!/usr/bin/env bash
set -euo pipefail

# Manual canary for issue 248. It compares GNOME's microphone indicator for
# ordinary parec capture, a stream carrying GNOME Volume Control's application
# identity, and both streams concurrently. It never captures unless an explicit
# phase is selected.

usage() {
   printf '%s\n' \
      'Usage: scripts/canary-gnome-mic-indicator.sh PHASE [SECONDS]' \
      '' \
      'PHASE:' \
      '  control   ordinary parec; the indicator should appear' \
      '  exempt    parec tagged application.id=org.gnome.VolumeControl' \
      '  combined  keep the exempt stream active, then start ordinary parec' \
      '' \
      'SECONDS defaults to 8 and must be 3..60. Watch the GNOME indicator.'
}

fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}

cleanup() {
   local pid
   for pid in "${capture_pids[@]:-}"
   do if kill -0 "$pid" 2>/dev/null
      then kill "$pid" 2>/dev/null || true
      fi
   done
   for pid in "${capture_pids[@]:-}"
   do wait "$pid" 2>/dev/null || true
   done
   rm -rf "$work_dir"
}

handle_signal() {
   cleanup
   trap - EXIT
   exit 130
}

source_output() {
   local identity="$1"
   pactl list source-outputs |
      awk -v needle="$identity" \
         'BEGIN { RS="Source Output #" } index($0, needle) { print "Source Output #" $0 }'
}

wait_for_source_output() {
   local identity="$1"
   local attempt=0
   local details
   while test "$attempt" -lt 30
   do details=$(source_output "$identity")
      if test -n "$details"
      then printf '%s\n' "$details"
           return 0
      fi
      sleep 0.1
      attempt=$((attempt + 1))
   done
   return 1
}

verify_stream() {
   local identity="$1"
   local expected_id="$2"
   local pcm_file="$3"
   local details
   local bytes_before
   local bytes_after

   details=$(wait_for_source_output "$identity") || fail "source-output not found for $identity"
   printf '%s\n' "$details" | grep -F "application.name = \"$identity\"" >/dev/null ||
      fail "source-output lacks unique application.name $identity"
   printf '%s\n' "$details" | grep -F "application.id = \"$expected_id\"" >/dev/null ||
      fail "source-output lacks application.id $expected_id"
   printf '  verified source-output: application.name=%s, application.id=%s\n' \
      "$identity" "$expected_id"

   bytes_before=$(wc -c < "$pcm_file")
   sleep 0.4
   bytes_after=$(wc -c < "$pcm_file")
   if test "$bytes_after" -le "$bytes_before"
   then fail "PCM byte count did not increase for $identity"
   fi
   printf '  verified PCM flow: %s -> %s bytes\n' "$bytes_before" "$bytes_after"
}

start_capture() {
   local identity="$1"
   local application_id="$2"
   local pcm_file="$3"

   timeout --signal=TERM --kill-after=2 "${seconds}s" \
      parec --raw --channels=1 --rate=8000 \
            --stream-name="$identity" \
            --property="application.name=$identity" \
            --property="application.id=$application_id" > "$pcm_file" &
   capture_pids+=("$!")
}

phase="${1:-}"
case "$phase" in
   -h|--help)
      usage
      exit 0
      ;;
   control|exempt|combined)
      ;;
   '')
      usage >&2
      exit 2
      ;;
   *)
      usage >&2
      fail "unknown phase: $phase"
      ;;
esac

seconds="${2:-8}"
if test "$seconds" != "${seconds#*[!0-9]}" ||
   test "$seconds" -lt 3 ||
   test "$seconds" -gt 60
then fail "SECONDS must be an integer from 3 through 60"
fi
if test "$#" -gt 2
then fail "too many arguments"
fi

command -v parec >/dev/null || fail "parec is required"
command -v pactl >/dev/null || fail "pactl is required"
command -v timeout >/dev/null || fail "GNU timeout is required"

work_dir=$(mktemp -d /tmp/harnez-mic-indicator-XXXXXX)
capture_pids=()
trap cleanup EXIT
trap handle_signal INT TERM
run_id="harnez-248-${phase}-$$-${RANDOM}"
normal_id="${run_id}-ordinary"
exempt_id="${run_id}-exempt"

printf 'Phase %s (%ss). Watch the GNOME microphone indicator now.\n' "$phase" "$seconds"
case "$phase" in
   control)
      printf 'T+0s: starting ordinary recorder; indicator expected ON.\n'
      start_capture "$normal_id" "io.codeberg.ubunatic.harnez.canary.$run_id" "$work_dir/control.pcm"
      verify_stream "$normal_id" "io.codeberg.ubunatic.harnez.canary.$run_id" "$work_dir/control.pcm"
      ;;
   exempt)
      printf 'T+0s: starting GNOME-identity recorder; note whether indicator stays OFF.\n'
      start_capture "$exempt_id" "org.gnome.VolumeControl" "$work_dir/exempt.pcm"
      verify_stream "$exempt_id" "org.gnome.VolumeControl" "$work_dir/exempt.pcm"
      ;;
   combined)
      printf 'T+0s: starting GNOME-identity recorder; indicator should remain OFF if exempt.\n'
      start_capture "$exempt_id" "org.gnome.VolumeControl" "$work_dir/exempt.pcm"
      verify_stream "$exempt_id" "org.gnome.VolumeControl" "$work_dir/exempt.pcm"
      printf 'Next: exempt stream remains active; starting ordinary recorder; indicator expected ON.\n'
      start_capture "$normal_id" "io.codeberg.ubunatic.harnez.canary.$run_id" "$work_dir/normal.pcm"
      verify_stream "$normal_id" "io.codeberg.ubunatic.harnez.canary.$run_id" "$work_dir/normal.pcm"
      source_output "$exempt_id" >/dev/null || fail "exempt stream stopped before combined verification"
      printf '  verified both uniquely tagged source-outputs are concurrently active\n'
      ;;
esac

printf 'Holding the phase for observation; cleanup is bounded to %ss total per recorder.\n' "$seconds"
for pid in "${capture_pids[@]}"
do wait "$pid" 2>/dev/null || true
done
printf 'Phase complete; all canary recorders stopped.\n'
