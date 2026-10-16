#!/usr/bin/env bash
set -euo pipefail

# Issue 740 M1/M2: print setup, list endpoints, or send a test message.
# The user starts every Claude session manually. This script never launches one,
# installs MCP config, enables channels globally, or touches Claude's cc-socks.
# Usage: scripts/canary-740-channel.sh [setup|list|send SESSION_ID [MESSAGE]|agent-send ARGS...]
# HARNEZ_BIN may select a freshly built/installed binary.

harnez_bin="${HARNEZ_BIN:-harnez}"
action="${1:-setup}"

case "$action" in
   setup|list)
      if test "$#" -gt 1
      then printf 'Usage: %s [setup|list|send SESSION_ID [MESSAGE]|agent-send ARGS...]\n' "$0" >&2
           exit 2
      fi
      exec "$harnez_bin" channel "$action"
      ;;
   send)
      if test "$#" -lt 2 || test "$#" -gt 3
      then printf 'Usage: %s send SESSION_ID [MESSAGE]\n' "$0" >&2
           exit 2
      fi
      if test "$#" = 3
      then exec "$harnez_bin" channel send "$2" --message "$3"
      else exec "$harnez_bin" channel send "$2"
      fi
      ;;
   agent-send)
      # Pass --name NAME or --id SESSION_ID, the message, and optional --wait/--timeout.
      shift
      exec "$harnez_bin" agent send "$@"
      ;;
   *)
      printf 'Usage: %s [setup|list|send SESSION_ID [MESSAGE]|agent-send ARGS...]\n' "$0" >&2
      exit 2
      ;;
esac
