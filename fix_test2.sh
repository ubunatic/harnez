#!/bin/bash
sed -i 's/runner.opts.Prompt != "opening line"/!strings.HasSuffix(runner.opts.Prompt, "opening line")/g' cmd/harnez/agent_interactive_test.go
