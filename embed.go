package harnez

import "embed"

//go:embed config.yaml commands docs/commands docs/lang docs/other docs/practices docs/templates systemd spec
var DefaultFS embed.FS
