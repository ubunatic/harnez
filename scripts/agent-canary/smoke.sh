#!/usr/bin/env bash
# In-container smoke test for harnez, Pi, Codex, and lmcoder model integration.

set -euo pipefail

echo "=== Harnez Container Agent Smoke Test ==="

echo "--- 1. System and Harnez Verification ---"
harnez status -c /work/config.yaml
git --version
git-lfs --version

echo "--- 2. Installed Agent CLI Versions ---"
if command -v pi >/dev/null 2>&1; then
    printf "Pi version: "
    pi --version || true
else
    echo "Pi CLI: not found"
fi

if command -v codex >/dev/null 2>&1; then
    printf "Codex version: "
    codex --version || true
else
    echo "Codex CLI: not found"
fi

echo "--- 3. Apply Harnez Skills & Hooks ---"
mkdir -p "${HOME}" /tmp/harnez-target
harnez apply -c /work/config.yaml -t /tmp/harnez-target
harnez status -c /work/config.yaml -t /tmp/harnez-target

echo "--- 4. Verify Installed Skills and Extensions ---"
test -f "${PI_CODING_AGENT_DIR:-${HOME}/.pi/agent}/extensions/harnez-distill.ts" && echo "Pi extension hook: installed"
test -d "${PI_CODING_AGENT_DIR:-${HOME}/.pi/agent}/skills" && echo "Pi skills target: present"

echo "--- 5. Agent Model PING PONG (if model server reachable) ---"
proxy_port="${HARNEZ_AGENT_CANARY_PROXY_PORT:-8735}"
base_url="${HARNEZ_AGENT_CANARY_BASE_URL:-http://localhost:${proxy_port}/v1}"

if curl -sf "${base_url}/models" >/dev/null 2>&1; then
    echo "Model proxy reachable at ${base_url}."
    if command -v pi-launch >/dev/null 2>&1; then
        echo "Testing Pi PING PONG..."
        pi-launch --provider llamacpp --model local-model -p "Reply with exactly one word: PONG"
    fi
    if command -v codex-launch >/dev/null 2>&1; then
        echo "Testing Codex PING PONG..."
        codex-launch exec --model local-model "Reply with exactly one word: PONG"
    fi
else
    echo "Model proxy at ${base_url} is not currently running (skipping live LLM PING PONG)."
fi

echo "=== Smoke Test Completed Successfully ==="
