#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later

set -euo pipefail

# canary-clean-workspace-docs — probes what documentation each agent harness
# (claude, codex, agy) sees in an isolated, clean workspace with zero local docs.
#
# Validates:
# 1. Global templates (~/.claude/CLAUDE.md, ~/.prime/agent/AGENTS.md) contain no eager @docs/ macros.
# 2. Each agent running in a clean directory receives zero inlined repository docs.
# 3. Probes whether each agent sees doc reference links (Bash, AgenticLoop, Go) and can load them on demand.

echo "=== 1. Checking Global Templates for Eager @docs/ Macros ==="

violations=0
for template in "${HOME}/.claude/CLAUDE.md" "${HOME}/.prime/agent/AGENTS.md"
do if test -f "${template}"
   then if grep -n -E '(^|\s)@docs/\S+' "${template}"
        then echo "FAIL: Found eager @docs/ macro in ${template}"
             violations=$((violations + 1))
        else echo "PASS: No eager @docs/ macros in ${template}"
        fi
   fi
done

if test "${violations}" -ne 0
then echo "ERROR: Eager doc include directives detected in global template(s)." >&2
     exit 1
fi

tmpdir=$(mktemp -d /tmp/harnez-canary-clean-XXXXXX)
trap 'rm -rf "${tmpdir}"' EXIT

echo ""
echo "=== 2. Probing Inlined Docs in Initial Context ==="

prompt_inlined="Do you see any repo docs (like Go.md, Bash.md, AgenticLoop.md) inlined in your initial context? Reply with YES or NO, followed by a 1-sentence explanation."

printf '%-12s | %-8s | %s\n' "Agent" "Status" "Explanation"
printf '%-12s-+-%-8s-+-%s\n' "------------" "--------" "------------------------------------------------------------"

# Probe Claude Code (Inlined)
if command -v claude >/dev/null 2>&1
then claude_reply=$(cd "${tmpdir}" && claude -p "${prompt_inlined}" 2>/dev/null | tr '\n' ' ' | sed 's/  */ /g')
     if echo "${claude_reply}" | grep -q -i "^NO"
     then printf '%-12s | %-8s | %s\n' "claude" "CLEAN" "${claude_reply}"
     else printf '%-12s | %-8s | %s\n' "claude" "INLINED" "${claude_reply}"
          violations=$((violations + 1))
     fi
else printf '%-12s | %-8s | %s\n' "claude" "SKIP" "claude binary not found in PATH"
fi

# Probe Codex (Inlined)
if command -v codex >/dev/null 2>&1
then codex_output=$(codex exec --ephemeral --skip-git-repo-check -C "${tmpdir}" "${prompt_inlined}" 2>/dev/null || true)
     codex_reply=$(echo "${codex_output}" | awk '/^codex/{flag=1; next} /^tokens used/{flag=0} flag' | tr '\n' ' ' | sed 's/  */ /g')
     if test -z "${codex_reply}"
     then codex_reply=$(echo "${codex_output}" | grep -E -i '^(NO|YES)' | head -1 || true)
     fi
     if test -z "${codex_reply}"
     then codex_reply="No output captured from codex"
     fi
     if echo "${codex_reply}" | grep -q -i "NO"
     then printf '%-12s | %-8s | %s\n' "codex" "CLEAN" "${codex_reply}"
     else printf '%-12s | %-8s | %s\n' "codex" "INLINED" "${codex_reply}"
          violations=$((violations + 1))
     fi
else printf '%-12s | %-8s | %s\n' "codex" "SKIP" "codex binary not found in PATH"
fi

# Probe Antigravity / Gemini CLI (Inlined)
if command -v agy >/dev/null 2>&1
then agy_reply=$(cd "${tmpdir}" && agy -p "${prompt_inlined}" 2>/dev/null | tr '\n' ' ' | sed 's/  */ /g')
     agy_clean=$(echo "${agy_reply}" | tr -d '*' | sed 's/^[ \t]*//')
     if echo "${agy_clean}" | grep -q -i "^NO"
     then printf '%-12s | %-8s | %s\n' "agy" "CLEAN" "${agy_reply}"
     else printf '%-12s | %-8s | %s\n' "agy" "INLINED" "${agy_reply}"
          violations=$((violations + 1))
     fi
else printf '%-12s | %-8s | %s\n' "agy" "SKIP" "agy binary not found in PATH"
fi

echo ""
echo "=== 3. Probing Doc References & On-Demand Loadability (Bash, AgenticLoop, Go) ==="

prompt_refs="Do you see references or links to documentation files (such as Bash, AgenticLoop, or Go) in your instructions, and could you load them on demand using your tools or filesystem paths? Reply with YES or NO, followed by a 1-sentence explanation."

printf '%-12s | %-8s | %s\n' "Agent" "Refs Seen" "Explanation"
printf '%-12s-+-%-8s-+-%s\n' "------------" "---------" "------------------------------------------------------------"

# Probe Claude Code (Refs & Loadability)
if command -v claude >/dev/null 2>&1
then claude_refs=$(cd "${tmpdir}" && claude -p "${prompt_refs}" 2>/dev/null | tr '\n' ' ' | sed 's/  */ /g')
     seen_status="NO"
     if echo "${claude_refs}" | grep -q -i "^YES"
     then seen_status="YES"
     fi
     printf '%-12s | %-9s | %s\n' "claude" "${seen_status}" "${claude_refs}"
else printf '%-12s | %-9s | %s\n' "claude" "SKIP" "claude binary not found in PATH"
fi

# Probe Codex (Refs & Loadability)
if command -v codex >/dev/null 2>&1
then codex_refs_out=$(codex exec --ephemeral --skip-git-repo-check -C "${tmpdir}" "${prompt_refs}" 2>/dev/null || true)
     codex_refs=$(echo "${codex_refs_out}" | awk '/^codex/{flag=1; next} /^tokens used/{flag=0} flag' | tr '\n' ' ' | sed 's/  */ /g')
     if test -z "${codex_refs}"
     then codex_refs=$(echo "${codex_refs_out}" | grep -E -i '^(NO|YES)' | head -1 || true)
     fi
     if test -z "${codex_refs}"
     then codex_refs="No output captured from codex"
     fi
     seen_status="NO"
     if echo "${codex_refs}" | grep -q -i "YES"
     then seen_status="YES"
     fi
     printf '%-12s | %-9s | %s\n' "codex" "${seen_status}" "${codex_refs}"
else printf '%-12s | %-9s | %s\n' "codex" "SKIP" "codex binary not found in PATH"
fi

# Probe Antigravity / Gemini CLI (Refs & Loadability)
if command -v agy >/dev/null 2>&1
then agy_refs=$(cd "${tmpdir}" && agy -p "${prompt_refs}" 2>/dev/null | tr '\n' ' ' | sed 's/  */ /g')
     seen_status="NO"
     if echo "${agy_refs}" | grep -q -i "YES"
     then seen_status="YES"
     fi
     printf '%-12s | %-9s | %s\n' "agy" "${seen_status}" "${agy_refs}"
else printf '%-12s | %-9s | %s\n' "agy" "SKIP" "agy binary not found in PATH"
fi

echo ""
if test "${violations}" -eq 0
then echo "Canary SUCCESS: Clean workspace documentation verification completed."
     exit 0
else echo "Canary FAILURE: Inlined docs or eager directives detected." >&2
     exit 1
fi
