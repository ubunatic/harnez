// Package docs exposes the repository's linkable documentation for installed
// tools that need to materialize trusted local context.
package docs

import "embed"

// FS contains the original linkable Markdown documents.
//
//go:embed AgenticLoop.md practices/AgenticLoop.lite.md lang/Bash.md lang/Bash.lite.md
var FS embed.FS
