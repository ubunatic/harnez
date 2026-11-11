## Verdict
done, implemented `CostCheckSpec`, tests, and the `CheckCost` pure function logic.
## Changes
internal/subagent/agentspec.go
internal/subagent/cost_check.go
internal/subagent/cost_check_test.go
spec/agent.yaml
spec/schemas/agent.schema.json
## Tests
`go test ./internal/subagent/` -> ok  	ubunatic.com/harnez/internal/subagent	15.422s
`go test ./cmd/harnez/` -> ok  	ubunatic.com/harnez/cmd/harnez	87.681s
