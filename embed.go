package harnez

import "embed"

//go:embed config.yaml commands docs/lang docs/other docs/practices docs/templates systemd
var DefaultFS embed.FS
