package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

// julesReaderFor returns the Jules driver behind --model jules. Any other model
// is refused before a request or a write (issue 1092).
func julesReaderFor(modelSpec, dir string) (subagent.JulesSessionReader, error) {
	if strings.TrimSpace(modelSpec) == "" {
		return nil, fmt.Errorf("--id is a Jules session ID; add --model jules")
	}
	model, err := subagent.ResolveModel(modelSpec)
	if err != nil {
		return nil, err
	}
	if model.Provider != "jules" {
		return nil, fmt.Errorf("--id is a Jules session ID; --model %s is not jules", modelSpec)
	}
	reader, ok := agentDriver(model, dir).(subagent.JulesSessionReader)
	if !ok {
		return nil, fmt.Errorf("provider jules cannot read a session by ID")
	}
	return reader, nil
}

// adoptJulesForResume records a Jules session that another machine started and
// returns the record's name, so the normal resume path runs on that record.
func adoptJulesForResume(cmd *cobra.Command, ac *agentCLI, id string) (string, error) {
	reader, err := julesReaderFor(ac.modelSpec, ac.workDir)
	if err != nil {
		return "", err
	}
	s, err := ac.store()
	if err != nil {
		return "", err
	}
	dir, err := filepath.Abs(ac.workDir)
	if err != nil {
		return "", err
	}
	sess, err := subagent.AdoptJulesSession(cmd.Context(), s, reader, id, ac.name, dir)
	if err != nil {
		return "", err
	}
	return sess.Name, nil
}

// statusJulesID prints one read of a Jules session by ID. It writes no record.
func statusJulesID(cmd *cobra.Command, ac *agentCLI, id string) error {
	if ac.name != "" {
		return fmt.Errorf("status --id cannot be combined with --name")
	}
	if err := subagent.ValidateJulesSessionID(id); err != nil {
		return err
	}
	reader, err := julesReaderFor(ac.modelSpec, ac.workDir)
	if err != nil {
		return err
	}
	status, err := reader.InspectJulesSession(cmd.Context(), id)
	if err != nil {
		return err
	}
	if ac.jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), status.Summary())
	return err
}
