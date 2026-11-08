#!/bin/bash
sed -i 's/opts, err := interactiveOptions(cmd, store, sess, d.storeDir, withProtocol(req.Prompt, false, role, inferred))/opts, err := interactiveOptions(cmd, store, sess, d.storeDir, withProtocol(req.Prompt, false, role, false))/g' cmd/harnez/agent_interactive.go

perl -pi -e 's/opts, err := interactiveOptions\(cmd, store, sess, d.storeDir, withProtocol\(req.Prompt, false, role, false\)\)/opts, err := interactiveOptions\(cmd, store, sess, d.storeDir, withProtocol\(req.Prompt, false, role, inferred\)\)/' cmd/harnez/agent_interactive.go
