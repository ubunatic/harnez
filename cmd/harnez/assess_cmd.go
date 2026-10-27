package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/assess"
	"ubunatic.com/harnez/internal/usage"
)

// newAssessCmd builds `harnez assess`.
func newAssessCmd() *cobra.Command {
	var assessJSON bool
	var assessTokens bool
	var assessHistory bool
	var assessRAMP bool
	var assessHotspots bool
	var assessGlossary bool
	assessCmd := &cobra.Command{
		Use:   "assess [path]",
		Short: "Fast code/doc metrics, token estimation, and repository feasibility report",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}

			if assessGlossary {
				return runAssessGlossary(cmd.OutOrStdout(), targetPath, assessJSON)
			}

			if assessHotspots {
				rep, err := assess.AssessHotspots(targetPath)
				if err != nil {
					return fmt.Errorf("assess --hotspots %s: %w", targetPath, err)
				}
				if assessJSON {
					data, err := json.MarshalIndent(rep, "", "  ")
					if err != nil {
						return fmt.Errorf("render json: %w", err)
					}
					fmt.Println(string(data))
					return nil
				}
				fmt.Print(assess.RenderHotspotsText(rep))
				return nil
			}

			if assessRAMP {
				profile, err := assess.AssessRAMP(targetPath)
				if err != nil {
					return fmt.Errorf("assess --ramp %s: %w", targetPath, err)
				}
				if assessJSON {
					out, err := assess.RenderRAMPJSON(profile)
					if err != nil {
						return fmt.Errorf("render json: %w", err)
					}
					fmt.Println(out)
					return nil
				}
				fmt.Print(assess.RenderRAMPText(profile))
				return nil
			}

			if assessTokens {
				res, err := usage.CollectProjectUsage(targetPath)
				if err != nil {
					return fmt.Errorf("assess --tokens %s: %w", targetPath, err)
				}
				if assessJSON {
					data, err := json.MarshalIndent(res, "", "  ")
					if err != nil {
						return fmt.Errorf("render json: %w", err)
					}
					fmt.Println(string(data))
					return nil
				}
				fmt.Print(usage.RenderProjectUsageCard(res))
				return nil
			}

			if assessHistory {
				res, err := assess.ExtractMultiTrackHistory(targetPath)
				if err != nil {
					return fmt.Errorf("assess --history %s: %w", targetPath, err)
				}
				if assessJSON {
					data, err := json.MarshalIndent(res, "", "  ")
					if err != nil {
						return fmt.Errorf("render json: %w", err)
					}
					fmt.Println(string(data))
					return nil
				}
				fmt.Print(assess.RenderMultiTrackCard(res))
				return nil
			}

			report, err := assess.AssessPath(targetPath)
			if err != nil {
				return fmt.Errorf("assess %s: %w", targetPath, err)
			}
			if assessJSON {
				out, err := assess.RenderJSON(report)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			fmt.Print(assess.RenderText(report))
			return nil
		},
	}
	assessCmd.Flags().BoolVar(&assessJSON, "json", false, "output report in JSON format")
	assessCmd.Flags().BoolVar(&assessTokens, "tokens", false, "display project-level lifetime AI token attribution and cost of change")
	assessCmd.Flags().BoolVar(&assessHistory, "history", false, "display multi-track repository evolution history")
	assessCmd.Flags().BoolVar(&assessRAMP, "ramp", false, "display RAMP repository AI maturity profile and evidence inventory")
	assessCmd.Flags().BoolVar(&assessHotspots, "hotspots", false, "list largest Go files and functions, import fan-out, and untested packages (git-tracked files)")
	assessCmd.Flags().BoolVar(&assessGlossary, "glossary", false, "inspect repository wording against the machine-readable glossary")
	return assessCmd
}
