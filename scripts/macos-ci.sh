#!/usr/bin/env bash
set -euo pipefail

repo="ubunatic/harnez"
workflow="macos-hello.yaml"
ref="main"

if ! command -v gh >/dev/null
then printf 'ERROR: gh CLI not found\n' >&2
     exit 1
fi

printf 'Dispatching %s on %s (%s)...\n' "$workflow" "$repo" "$ref"
gh workflow run "$workflow" --repo "$repo" --ref "$ref"

printf 'Waiting for run to register...\n'
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

printf 'Watching run %s...\n' "$run_id"
gh run watch "$run_id" --repo "$repo" --exit-status
