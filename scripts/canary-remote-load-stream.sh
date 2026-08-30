#!/usr/bin/env bash
set -euo pipefail

# Canary for issue 110: validates OpenSSH ControlMaster connection
# multiplexing as the transport for a remote Load streaming channel that a
# batch (usage) call can share, before any harnez feature code is built on
# top of it. See docs/Canary.md for the canary-first convention this
# follows, and docs/Bash.md for the shell style rules.
#
# Usage: scripts/canary-remote-load-stream.sh HOST

host="${1:?Usage: scripts/canary-remote-load-stream.sh HOST}"
ctl_dir=$(mktemp -d)
ctl="${ctl_dir}/cm-%r@%h:%p"

trap 'rm -rf "${ctl_dir}"' EXIT

echo "==> Starting a background ControlMaster connection to ${host}"
ssh -M -S "${ctl}" -o ControlPersist=60 -o ConnectTimeout=5 -fN "${host}"

echo "==> Confirming the control socket is live"
ssh -S "${ctl}" -O check "${host}"

echo "==> Timing a cold call (no multiplexing: fresh handshake)"
time ssh -o ControlMaster=no -o ConnectTimeout=5 "${host}" "echo cold-call" > /dev/null

echo "==> Timing a warm call reused over the master (should be far faster: no handshake)"
time ssh -S "${ctl}" "${host}" "echo warm-call" > /dev/null

echo "==> Starting a long-lived 'stream' child over the same master (simulates harnez load-stream)"
ssh -S "${ctl}" "${host}" "for i in 1 2 3; do echo sample-\${i}; sleep 0.2; done" &
stream_pid=$!

echo "==> Firing a concurrent 'batch' call over the same master while the stream child runs"
ssh -S "${ctl}" "${host}" "echo concurrent-batch-call"

wait "${stream_pid}"

echo "==> Tearing down the master"
ssh -S "${ctl}" -O exit "${host}"

echo "==> Confirming a call after teardown still succeeds via a plain (non-multiplexed) connection"
ssh -o ConnectTimeout=5 "${host}" "echo fallback-after-teardown"

echo ""
echo "==> Canary completed successfully."
