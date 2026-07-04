package claudeconfig

import "embed"

//go:embed config.yaml commands docs/lang docs/other docs/templates
var DefaultFS embed.FS
