package harnez

import "embed"

//go:embed config.yaml commands docs/lang docs/other docs/practices docs/templates
var DefaultFS embed.FS
