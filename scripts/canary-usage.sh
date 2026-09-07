#!/usr/bin/env bash
set -euo pipefail

# Canary for harnez usage: Unified Token, Session & Quota Status Command
# Validates local harness configs and runs harnez usage checks without side-effects.

echo "==> Probing Agent Configuration Directories..."

if test -d "${HOME}/.claude"
then echo "  [FOUND] ~/.claude"
     if test -f "${HOME}/.claude/.credentials.json"
     then echo "    - credentials present"
     fi
     if test -f "${HOME}/.claude/stats-cache.json"
     then echo "    - stats-cache.json present"
     fi
else echo "  [NOT FOUND] ~/.claude"
fi

if test -d "${HOME}/.gemini/antigravity-cli"
then echo "  [FOUND] ~/.gemini/antigravity-cli"
     if test -f "${HOME}/.gemini/antigravity-cli/antigravity-oauth-token"
     then echo "    - oauth token present"
     fi
else echo "  [NOT FOUND] ~/.gemini/antigravity-cli"
fi

if test -d "${HOME}/.codex"
then echo "  [FOUND] ~/.codex"
     if test -f "${HOME}/.codex/auth.json"
     then echo "    - auth.json present"
     fi
     if test -f "${HOME}/.codex/config.toml"
     then echo "    - config.toml present"
     fi
else echo "  [NOT FOUND] ~/.codex"
fi

echo ""
echo "==> Running harnez usage in terminal mode..."
go run ./cmd/harnez usage

echo ""
echo "==> Running harnez usage in JSON mode..."
go run ./cmd/harnez usage --json | head -n 25

echo ""
echo "==> Canary completed successfully."
