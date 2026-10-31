package harnez

import "embed"

//go:embed config.yaml docs/commands docs/lang docs/other docs/practices docs/templates systemd spec all:third_party/skills .pi/extensions/background-tasks.ts .pi/lib/background-task-runner.ts .pi/lib/background-task-capture.ts .pi/lib/background-task-store.ts
var DefaultFS embed.FS
