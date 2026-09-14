#!/usr/bin/env bash
set -euo pipefail

# Quiet by design: agents call this regularly and `gh run watch`'s live
# redrawn job/step tree floods agent context with no added signal. This
# prints nothing while the run is in progress and a single summary line
# (plus a bounded failure excerpt on failure) when it's done.

repo="ubunatic/harnez"
workflow="macos-hello.yaml"
ref="main"
poll_interval=10

if ! command -v gh >/dev/null
then printf 'ERROR: gh CLI not found\n' >&2
     exit 1
fi

gh workflow run "$workflow" --repo "$repo" --ref "$ref" >/dev/null

run_id=""
attempt=0
while test -z "$run_id" && test "$attempt" -lt 10
do sleep 2
   run_id=$(gh run list --repo "$repo" --workflow "$workflow" --limit 1 --json databaseId -q '.[0].databaseId')
   attempt=$((attempt + 1))
done

if test -z "$run_id"
then printf 'ERROR: run did not register in time\n' >&2
     exit 1
fi

status=""
while test "$status" != "completed"
do sleep "$poll_interval"
   status=$(gh run view "$run_id" --repo "$repo" --json status -q '.status')
done

conclusion=$(gh run view "$run_id" --repo "$repo" --json conclusion -q '.conclusion')
url=$(gh run view "$run_id" --repo "$repo" --json url -q '.url')

if test "$conclusion" = "success"
then printf 'PASS: macos-hello run %s\n' "$run_id"
     exit 0
fi

printf 'FAIL: macos-hello run %s (%s)\n%s\n' "$run_id" "$conclusion" "$url"
gh run view "$run_id" --repo "$repo" --log-failed |
	grep -E '(FAIL|Error|error:)' |
	tail -n 20
exit 1
