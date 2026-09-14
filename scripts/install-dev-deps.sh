#!/usr/bin/env bash
set -euo pipefail

# Installs local dev/test tool dependencies not covered by `go build`/`go test`
# themselves (e.g. minisign, used by internal/release tests and the release
# pipeline). Candidate for a future `harnez install --dev` subcommand — kept
# as a standalone script for now so CI workflow YAML stays minimal.

os=$(uname -s)

install_minisign() {
	if command -v minisign >/dev/null
	then printf 'minisign already installed: %s\n' "$(command -v minisign)"
	     return 0
	fi

	case "$os" in
	Darwin)
		if ! command -v brew >/dev/null
		then printf 'ERROR: brew not found, cannot install minisign\n' >&2
		     return 1
		fi
		brew install minisign
		;;
	Linux)
		if command -v apt-get >/dev/null
		then sudo apt-get update -y
		     sudo apt-get install -y minisign
		else printf 'ERROR: no supported package manager found for minisign on Linux\n' >&2
		     return 1
		fi
		;;
	*)
		printf 'ERROR: unsupported OS %s for automatic minisign install\n' "$os" >&2
		return 1
		;;
	esac
}

install_minisign
