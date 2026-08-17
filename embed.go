package harnez

import "embed"

//go:embed config.yaml commands docs/lang docs/other docs/templates spec
var DefaultFS embed.FS
