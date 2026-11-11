## Target: tool-only-plug-in

*   **Proposal 1:** Add a `ToolOnly` boolean flag to `Model` and `modelAlias` in `internal/subagent/driver.go:133`.
    *   **Citation:** `internal/subagent/driver.go:133` `type Model struct {`
    *   **Why it helps:** Passing a tool-only setting on the Model allows harnez to parse the option from `agent.yaml` without changing driver signatures (it is implicitly available to drivers in `RunOptions.Model`).
    *   **Smallest step:** Add a `ToolOnly bool` field to the `Model` struct.
*   **Proposal 2:** Pass a `--tool-only` flag to the `pi` CLI in `internal/subagent/pi.go:121`.
    *   **Citation:** `internal/subagent/pi.go:121` `func piRunArgs(model Model, prompt string, contextMode ...string) []string {`
    *   **Why it helps:** This translates the tool-only intent to the underlying Pi/lmcoder provider, instructing the model to produce a strict JSON envelope.
    *   **Smallest step:** Add a check for `model.ToolOnly` in `piRunArgs` and, if true, append an extra CLI flag like `--tool-only` to `args`.
*   **Proposal 3:** Re-run the turn with a corrective prompt on malformed JSON envelopes in `internal/subagent/resume.go:80`.
    *   **Citation:** `internal/subagent/resume.go:80` `return t.Driver.Run(ctx, t.Opts)`
    *   **Why it helps:** If a model produces invalid JSON in tool-only mode, crashing halts progress. Harnez needs to feed the syntax error back to let the model fix it.
    *   **Smallest step:** In the interactive loop or driver call, if `Opts.Model.ToolOnly` is true and parsing the response returns a JSON error, prepend the error to a new prompt and call `t.Driver.Resume` again.
*   **Proposal 4:** Expose the tool-only setting via `agent.yaml` in `spec/schemas/agent.schema.json:200`.
    *   **Citation:** `spec/schemas/agent.schema.json:200` `"models": {`
    *   **Why it helps:** Owners need to explicitly configure which models (like `qwen3.8-flash`) are bound to tool-only mode since the capability varies.
    *   **Smallest step:** Add a `"tool_only": { "type": "boolean" }` property under the `models` pattern properties in the JSON schema, matching the Go struct field mapping.
*   **Proposal 5:** Add `ToolOnly` to `ModelEntry` so the UI lists models capable of this mode in `internal/subagent/driver.go:359`.
    *   **Citation:** `internal/subagent/driver.go:359` `type ModelEntry struct {`
    *   **Why it helps:** Users running `harnez agent start` need to see whether a selected model operates in tool-only mode when checking model capabilities.
    *   **Smallest step:** Add `ToolOnly bool json:"tool_only"` to `ModelEntry` and populate it from the configured `Model`.
