#!/usr/bin/env bash
# Benchmark token savings of `harnez read -I` and `harnez read --auto` against
# a fixed set of key project docs.
# Run from the project root: scripts/benchmark-read-savings.sh [-h|--help]

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/benchmark-read-savings.sh [-h|--help]

Renders each file in a fixed set of key harnez docs with:
  harnez read -I --json      (forced image card, per-provider token stats)
  harnez read --auto --json  (provider-adaptive routing decision)

--auto is run once per HARNEZ_AGENT_HARNESS profile in PROFILES below
(default: unset, claude, gemini).

Prints a plain-text table to stdout:
  file  lines  raw_tokens  claude_img_tokens  claude_compression  auto:<profile>...

Requires: harnez (on PATH), jq.
Cleans up its own scratch PNGs on exit.
EOF
}

case "${1:-}" in
-h | --help)
	usage
	exit 0
	;;
esac

if ! command -v harnez >/dev/null 2>&1
then
	echo "error: harnez not found on PATH" >&2
	exit 1
fi

if ! command -v jq >/dev/null 2>&1
then
	echo "error: jq not found on PATH" >&2
	exit 1
fi

repo_root=$(git rev-parse --show-toplevel)

# Fixed set of representative docs: small/medium/large, prose + code.
candidates=(
	"AGENTS.md"
	"docs/AgenticLoop.md"
	"docs/CLIDesign.md"
	"docs/MicIndicators.md"
	"docs/lang/Bash.md"
	"docs/lang/Go.md"
	"cmd/harnez/main.go"
)

profiles=("" "claude" "gemini")

tmpdir=$(mktemp -d /tmp/harnez-bench.XXXXXX)
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT

files=()
skipped=()
for f in "${candidates[@]}"
do
	if test -f "$repo_root/$f"
	then files+=("$f")
	else skipped+=("$f")
	fi
done

if test ${#skipped[@]} -gt 0
then
	echo "Skipped (missing): ${skipped[*]}" >&2
fi

if test ${#files[@]} -eq 0
then
	echo "error: no candidate files found under $repo_root" >&2
	exit 1
fi

header="FILE\tLINES\tRAW_TOK\tIMG_TOK(claude)\tCOMPRESSION"
for p in "${profiles[@]}"
do
	label="auto:${p:-default}"
	header="$header\t$label"
done

rows=()
rows+=("$header")

for f in "${files[@]}"
do
	abs="$repo_root/$f"
	lines=$(wc -l <"$abs" | tr -d ' ')

	img_json=$(harnez read -I --json -o "$tmpdir/img_$(basename "$f").png" "$abs" 2>/dev/null || true)
	raw_tok=$(printf '%s' "$img_json" | jq -r '.token_stats.text_tokens // "n/a"')
	img_tok=$(printf '%s' "$img_json" | jq -r '.token_stats.claude_vision_tokens // "n/a"')
	compression=$(printf '%s' "$img_json" | jq -r '.token_stats.claude_compression_ratio // "n/a"')

	row="$f\t$lines\t$raw_tok\t$img_tok\t${compression}x"

	for p in "${profiles[@]}"
	do
		if test -n "$p"
		then auto_json=$(HARNEZ_AGENT_HARNESS="$p" harnez read --auto --json -o "$tmpdir/auto_${p}_$(basename "$f").png" "$abs" 2>/dev/null || true)
		else auto_json=$(harnez read --auto --json -o "$tmpdir/auto_default_$(basename "$f").png" "$abs" 2>/dev/null || true)
		fi

		decision=$(printf '%s' "$auto_json" | jq -r 'if has("primary_path") then "image" else "text" end')
		row="$row\t$decision"
	done

	rows+=("$row")
done

printf '%b\n' "${rows[@]}" | column -t -s "$(printf '\t')"
