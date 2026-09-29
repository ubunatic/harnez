package harnez

import "embed"

//go:embed config.yaml docs/commands docs/lang docs/other docs/practices docs/templates systemd spec all:third_party/skills
var DefaultFS embed.FS
