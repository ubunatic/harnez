#!/bin/bash
sed -i 's/opts, err := interactiveOptions(cmd, store, sess, d.storeDir, req.Prompt)/opts, err := interactiveOptions(cmd, store, sess, d.storeDir, withProtocol(req.Prompt, false, role, inferred))/g' cmd/harnez/agent_interactive.go
