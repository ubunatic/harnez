package main

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/distill"
)

func newDistillReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read",
		Short: "Bound authorized native read text or prepare attachable PNG output (JSON stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var req distill.ReadOutputRequest
			if err := json.NewDecoder(io.LimitReader(cmd.InOrStdin(), 16<<20)).Decode(&req); err != nil {
				return err
			}
			result, err := distill.DistillReadOutput(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}
