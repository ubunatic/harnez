package claudeconfig

import "embed"

//go:embed config.yaml commands docs/src
var DefaultFS embed.FS
