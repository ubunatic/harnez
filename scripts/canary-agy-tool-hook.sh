#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later

set -euo pipefail

# Canary hook for Antigravity PreToolUse event.
# Ingests JSON from stdin, logs tool invocations into ~/.harnez/tool_catalog.sqlite,
# and emits {"decision": "allow"} to stdout so tool execution continues unimpeded.

db_file="${HOME}/.harnez/tool_catalog.sqlite"
log_file="${HOME}/.harnez/agy-tool-hook-canary.log"

payload=$(cat)

if test -z "${payload}"
then printf '{"decision":"allow"}\n'
     exit 0
fi

mkdir -p "${HOME}/.harnez"

# Log raw JSON payload for inspection
printf '%s\n' "${payload}" >> "${log_file}"

# Parse tool name and metadata using jq or python3
tool_name=$(printf '%s' "${payload}" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    print(data.get("toolCall", {}).get("name", "unknown"))
except Exception:
    print("unknown")
')

session_id=$(printf '%s' "${payload}" | python3 -c '
import sys, json
try:
    data = json.load(sys.stdin)
    print(data.get("conversationId", "unknown"))
except Exception:
    print("unknown")
')

# Insert into telemetry sqlite if available
if test -f "${db_file}"
then sqlite3 "${db_file}" <<EOF || true
INSERT INTO tool_calls (
    session_id,
    agent_id,
    tool_name,
    score,
    call_type,
    created_at
) VALUES (
    '${session_id}',
    'agy',
    '${tool_name}',
    5.0,
    'exec',
    datetime('now')
);
EOF
fi

# Always return allow decision
printf '{"decision":"allow"}\n'
