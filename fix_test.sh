#!/bin/bash
sed -i 's/runner.opts.Prompt != "continue interactively"/!strings.HasSuffix(runner.opts.Prompt, "continue interactively")/g' cmd/harnez/agent_interactive_test.go
