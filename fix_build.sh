#!/bin/bash
sed -i 's/role, inferred, err := startRole(/role, _, err := startRole(/g' cmd/harnez/agent_interactive.go
sed -i 's/opts.Prompt = withProtocol(req.Prompt, false, role, inferred)/opts.Prompt = withProtocol(req.Prompt, false, role, false)/g' cmd/harnez/agent_interactive.go

sed -i 's/role, inferred, err := startRole(/role, _, err := startRole(/g' cmd/harnez/agent_async.go
sed -i 's/req.Prompt = withProtocol(req.Prompt, req.PlanFirst, role, inferred)/req.Prompt = withProtocol(req.Prompt, req.PlanFirst, role, false)/g' cmd/harnez/agent_async.go
