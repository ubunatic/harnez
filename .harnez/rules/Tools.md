# Harnez Tools

If `harnez` is not installed or available in PATH, install it via:
```bash
go install ubunatic.com/harnez/cmd/harnez@latest
```

- Prefer structured patch tools (`apply_patch`) or whole-block replacements over narrow string substitution edits.
- For medium/large files, prefer `harnez read -L <range>` / `harnez read -n`; use targeted native reads only when appropriate.
- Before broad shell searches, use `harnez find code|docs` or MCP `harnez_find`; see `@docs/Search.md`.
