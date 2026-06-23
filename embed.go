package claudeconfig

import "embed"

//go:embed config.yaml commands docs/src docs/templates
var DefaultFS embed.FS
