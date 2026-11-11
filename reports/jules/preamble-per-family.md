## Target: Add a per-family preamble

This report proposes changes to add a start/resume preamble that differs per model family to the text `harnez` adds before an agent's first prompt on start and resume (which is assembled via `withProtocol` calling `agent.WithProtocol`).

1. **Add `preambles` mapping to `spec/agent.yaml` schema**
   `path:line` `spec/agent.yaml:340` `default_role: developer`
   - **Proposed change**: Add a new top-level `preambles` map (e.g. `preambles: { claude: "...", codex: "..." }`) alongside roles and models.
   - **Why it helps**: This provides a declarative and easily extensible way to define per-family instructions that models follow best, fulfilling the requirement of keeping it in `spec/`.
   - **Smallest step**: Edit `spec/agent.yaml` to add `preambles:` with some default mappings, then update the `agentSpec` struct in `internal/subagent/agentspec.go` to parse it.

2. **Parse `preambles` in `agentSpec` struct**
   `path:line` `internal/subagent/agentspec.go:42` `	KnownErrorChecks      []KnownErrorCheckSpec          ` + "`yaml:\"known_error_checks\"`" + `
   - **Proposed change**: Add `Preambles map[string]string` field to the `agentSpec` struct.
   - **Why it helps**: This allows `harnez` to unmarshal the newly added `preambles` map in `spec/agent.yaml` into the Go struct, making it available to the program.
   - **Smallest step**: Add `Preambles map[string]string ` + "`yaml:\"preambles\"`" + ` to `type agentSpec struct` in `internal/subagent/agentspec.go`.

3. **Expose provider preamble via `subagent` package function**
   `path:line` `internal/subagent/agentspec.go:722` `func RoleRules(role string) (string, error) {`
   - **Proposed change**: Create a new function `FamilyPreamble(provider string) (string, error)` that queries the parsed `agentSpec` for `spec.Preambles[provider]`.
   - **Why it helps**: Encapsulates the configuration read and gives `internal/agent/core.go` a way to fetch the provider-specific string.
   - **Smallest step**: Implement `func FamilyPreamble(provider string) (string, error) { spec, _ := agentSpecOnce(); return spec.Preambles[provider], nil }` in `internal/subagent/agentspec.go`.

4. **Update `WithProtocol` signature in `internal/agent/core.go` to accept provider**
   `path:line` `internal/agent/core.go:31` `func WithProtocol(prompt string, planFirst bool, role string, inferred bool) string {`
   - **Proposed change**: Add a `provider string` argument to `WithProtocol`.
   - **Why it helps**: The protocol assembly function needs to know the family (provider) to inject the correct preamble.
   - **Smallest step**: Change the signature to `func WithProtocol(prompt string, planFirst bool, role string, inferred bool, provider string) string`.

5. **Inject family preamble during `WithProtocol` assembly**
   `path:line` `internal/agent/core.go:32` `	pre := protocolPreamble`
   - **Proposed change**: Inside `WithProtocol`, fetch the family preamble (using `subagent.FamilyPreamble(provider)`) and append it to `pre`.
   - **Why it helps**: This applies the per-family preamble exactly where `harnez` adds text in front of an agent's first prompt on start and on resume.
   - **Smallest step**: Add logic `if familyPre, err := subagent.FamilyPreamble(provider); err == nil && familyPre != "" { pre += familyPre + "\n\n" }` inside `WithProtocol` after `protocolPreamble`.
