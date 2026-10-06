#!/usr/bin/env bash
# In-container smoke test (issue 723): harnez apply and status, then Pi and
# Codex against the lmcoder model server in the same pod. Every check must
# pass; a check that cannot run counts as a failure.

set -uo pipefail

base_url="${HARNEZ_AGENT_CANARY_BASE_URL:-http://127.0.0.1:8734/v1}"
server_url="${base_url%/v1}"
wait_seconds="${HARNEZ_SMOKE_WAIT_SECONDS:-900}"
pi_home="${PI_CODING_AGENT_DIR:-${HOME}/.pi/agent}"
out=/tmp/harnez-smoke
failures=0
summary=()

mkdir -p "${out}"

pass() {
   printf 'PASS  %s\n' "$1"
   summary+=("PASS  $1")
}

fail() {
   printf 'FAIL  %s\n' "$1"
   summary+=("FAIL  $1")
   failures=$((failures + 1))
}

check() {
   local label="$1"
   shift
   if "$@"
   then pass "${label}"
   else fail "${label}"
   fi
}

step() {
   printf '\n=== %s\n' "$1"
}

# hook_runs prints how often harnez recorded the given subcommand, e.g.
# "codex-telemetry" or "distill hook". Agents run hooks as harnez calls.
hook_runs() {
   harnez log -500 | grep -c -- " $1 "
}

# skill_dirs prints the names of the skill directories under $1.
skill_dirs() {
   find "$1" -mindepth 2 -maxdepth 2 -name SKILL.md -printf '%h\n' | xargs -n1 basename | sort
}

# all_skills_in checks that every skill installed under $1 appears in file $2
# in the form printf "$3" <name>.
all_skills_in() {
   local dir="$1" file="$2" format="$3" name missing=0
   while read -r name
   do if ! grep -qF -- "$(printf -- "${format}" "${name}")" "${file}"
      then printf '  not listed: %s\n' "${name}"
           missing=1
      fi
   done < <(skill_dirs "${dir}")
   test "${missing}" -eq 0
}

replied_pong() {
   grep -qi 'pong' "$1"
}

server_healthy() {
   curl -sf "${server_url}/health" >/dev/null
}

step "1. Installed components"
check "harnez runs" harnez --version
check "git runs" git --version
check "git-lfs runs" git lfs version
check "node runs" node --version
check "pi runs" pi --version
check "codex runs" sh -c 'codex --version 2>/dev/null'

step "2. harnez apply and status"
check "harnez apply" harnez apply
harnez status > "${out}/status.txt" 2>&1
status_exit=$?
check "harnez status exits 0" test "${status_exit}" -eq 0
not_ok="$(grep -E 'SKILL\.md|harnez-distill|\[hooks|hooks\.json' "${out}/status.txt" | grep -v ' ok$' || true)"
if test -z "${not_ok}"
then pass "harnez status: all skills and hooks ok"
else fail "harnez status: entries not ok:"
     printf '%s\n' "${not_ok}"
fi

step "3. Skills and hooks installed"
pi_skills="$(skill_dirs "${pi_home}/skills" 2>/dev/null | wc -l)"
codex_skills="$(skill_dirs "${HOME}/.codex/skills" 2>/dev/null | wc -l)"
check "Pi skills installed (${pi_skills})" test "${pi_skills}" -gt 0
check "Codex skills installed (${codex_skills})" test "${codex_skills}" -gt 0
check "Pi issue skill installed" test -f "${pi_home}/skills/issue/SKILL.md"
check "Pi Distill extension installed" test -f "${pi_home}/extensions/harnez-distill.ts"
check "Codex hooks installed" grep -q 'harnez codex-telemetry' "${HOME}/.codex/config.toml"

step "4. lmcoder model server"
start="${SECONDS}"
until server_healthy || test $((SECONDS - start)) -ge "${wait_seconds}"
do sleep 5
done
check "model server healthy at ${server_url} (waited $((SECONDS - start))s)" server_healthy
model_id="$(curl -sf "${base_url}/models" | jq -r '.data[0].id // empty')"
check "model server lists a model (${model_id:-none})" test -n "${model_id}"

step "5. Pi"
start="${SECONDS}"
pi-launch --provider llamacpp --model local-model --mode json \
   -p "Reply with exactly one word: PONG" > "${out}/pi-pong.jsonl" 2> "${out}/pi-pong.err"
pi_status=$?
pi_seconds=$((SECONDS - start))
jq -r 'select(.type == "message_end" and .message.role == "assistant") | .message.content[]?.text // empty' \
   "${out}/pi-pong.jsonl" > "${out}/pi-pong.txt" 2>/dev/null
check "Pi exits 0 (${pi_seconds}s)" test "${pi_status}" -eq 0
check "Pi replies PONG" replied_pong "${out}/pi-pong.txt"
check "Pi lists every Harnez skill" \
   all_skills_in "${pi_home}/skills" "${out}/pi-pong.jsonl" '<name>%s</name>'

before="$(hook_runs 'distill hook')"
HARNEZ_DISTILL_AUTOPIPE=true pi-launch --provider llamacpp --model local-model --mode json --tools bash \
   -p "Use the bash tool to run this exact command: echo harnez-hook-probe" \
   > "${out}/pi-hook.jsonl" 2> "${out}/pi-hook.err"
check "Pi model called bash" grep -q '"type":"tool_execution_start".*"toolName":"bash"' "${out}/pi-hook.jsonl"
check "Pi fired the Harnez Distill hook" test "$(hook_runs 'distill hook')" -gt "${before}"

step "6. Codex"
before="$(hook_runs codex-telemetry)"
start="${SECONDS}"
# Hook trust bypass: these hooks were just written by harnez apply in this
# throwaway container, the vetted-automation case the flag is meant for.
codex-launch exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox \
   --dangerously-bypass-hook-trust --json "Reply with exactly one word: PONG" \
   < /dev/null > "${out}/codex-pong.jsonl" 2> "${out}/codex-pong.err"
codex_status=$?
codex_seconds=$((SECONDS - start))
jq -r 'select(.type == "item.completed" and .item.type == "agent_message") | .item.text' \
   "${out}/codex-pong.jsonl" > "${out}/codex-pong.txt" 2>/dev/null
check "Codex exits 0 (${codex_seconds}s)" test "${codex_status}" -eq 0
check "Codex replies PONG" replied_pong "${out}/codex-pong.txt"
codex_session="$(find "${HOME}/.codex/sessions" -name '*.jsonl' 2>/dev/null | sort | tail -1)"
# Pi's set is Harnez's own skills; the bundled third-party skills that apply
# also writes to ~/.codex/skills are explicit-only and hidden from Codex.
check "Codex lists every Harnez skill" \
   all_skills_in "${pi_home}/skills" "${codex_session:-/dev/null}" '- %s: '
check "Codex fired the Harnez hooks" test "$(hook_runs codex-telemetry)" -gt "${before}"

step "Versions"
printf '  harnez   %s\n' "$(harnez --version)"
printf '  git      %s\n' "$(git --version)"
printf '  git-lfs  %s\n' "$(git lfs version)"
printf '  node     %s\n' "$(node --version)"
printf '  pi       %s\n' "$(pi --version 2>&1)"
printf '  codex    %s\n' "$(codex --version 2>/dev/null)"
printf '  model    %s\n' "${model_id:-unknown}"
printf '  timings  pi %ss, codex %ss (PING PONG, CPU)\n' "${pi_seconds}" "${codex_seconds}"

step "Summary"
printf '%s\n' "${summary[@]}"
if test "${failures}" -gt 0
then printf '\n%s check(s) failed; logs are in %s\n' "${failures}" "${out}"
     exit 1
fi
printf '\nAll checks passed.\n'
