// guard implements `harnez guard`, providing diagnostic and dry-run verification
// for decide-based rule enforcement and safety guardrails. See issue 642.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/decide"
	"ubunatic.com/harnez/internal/guard"
)

func newGuardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Rule enforcer and safety guardrail commands",
		Long: `Inspect and verify decide-based rule enforcement and safety guardrails.

Use 'harnez guard status' to inspect current configuration and backend availability.
Use 'harnez guard check <file>' to dry-run rule checks on a file.`,
	}

	cmd.AddCommand(newGuardStatusCmd())
	cmd.AddCommand(newGuardCheckCmd())
	return cmd
}

func newGuardStatusCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Display decide guardrail status, backend, and key configuration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGuardStatus(cmd.OutOrStdout(), jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output status as JSON")
	return cmd
}

type guardStatusReport struct {
	Enabled      bool    `json:"enabled"`
	ToggleSource string  `json:"toggle_source"`
	BackendName  string  `json:"backend_name"`
	Protocol     string  `json:"protocol"`
	BaseURL      string  `json:"base_url"`
	Model        string  `json:"model"`
	APIKeyEnv    string  `json:"api_key_env,omitempty"`
	APIKeyStatus string  `json:"api_key_status"`
	Threshold    float64 `json:"threshold"`
}

func runGuardStatus(out io.Writer, jsonOutput bool) error {
	enabled, toggleSrc := guard.EnabledWithSource(nil)

	report := guardStatusReport{
		Enabled:      enabled,
		ToggleSource: toggleSrc,
		Threshold:    guard.DefaultThreshold,
	}

	spec, err := decide.LoadSpec()
	if err == nil && spec != nil {
		report.BackendName = spec.DefaultBackend
		if b, ok := spec.Backends[spec.DefaultBackend]; ok && b != nil {
			report.Protocol = b.Protocol
			report.BaseURL = b.BaseURL
			report.Model = b.Model
			report.APIKeyEnv = b.APIKeyEnv
			if b.APIKeyEnv != "" {
				if os.Getenv(b.APIKeyEnv) != "" {
					report.APIKeyStatus = "set"
				} else {
					report.APIKeyStatus = "unset (missing)"
				}
			} else {
				report.APIKeyStatus = "none required"
			}
		}
	}

	if jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Fprintln(out, "Decide Guardrail Status")
	fmt.Fprintln(out, "=======================")
	statusStr := "DISABLED"
	if report.Enabled {
		statusStr = "ENABLED"
	}
	fmt.Fprintf(out, "Status:       %s (%s)\n", statusStr, report.ToggleSource)
	fmt.Fprintf(out, "Backend:      %s (%s, %s)\n", report.BackendName, report.Protocol, report.Model)
	fmt.Fprintf(out, "Base URL:     %s\n", report.BaseURL)
	if report.APIKeyEnv != "" {
		fmt.Fprintf(out, "API Key:      $%s (%s)\n", report.APIKeyEnv, report.APIKeyStatus)
	}
	fmt.Fprintf(out, "Threshold:    %.2f\n", report.Threshold)
	return nil
}

func newGuardCheckCmd() *cobra.Command {
	var content string
	var contentFile string
	var repoRoot string
	var threshold float64
	var format string

	cmd := &cobra.Command{
		Use:          "check <file>",
		Short:        "Dry-run rule enforcement check on a file",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			var proposedContent string
			if cmd.Flags().Changed("content") {
				proposedContent = content
			} else if cmd.Flags().Changed("content-file") {
				data, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("read content file: %w", err)
				}
				proposedContent = string(data)
			} else {
				if data, err := os.ReadFile(filePath); err == nil {
					proposedContent = string(data)
				}
			}

			spec, err := decide.LoadSpec()
			if err != nil {
				return fmt.Errorf("load decide spec: %w", err)
			}
			backend, err := spec.Open("", nil)
			if err != nil {
				return fmt.Errorf("open decide backend: %w", err)
			}

			if repoRoot == "" {
				repoRoot, _ = os.Getwd()
			}

			err = runGuardCheck(cmd.Context(), cmd.OutOrStdout(), backend, repoRoot, filePath, proposedContent, threshold, format)
			return silenceIfExitCode(cmd, err)
		},
	}

	f := cmd.Flags()
	f.StringVar(&content, "content", "", "proposed file content string")
	f.StringVar(&contentFile, "content-file", "", "file containing proposed content")
	f.StringVarP(&repoRoot, "repo", "d", "", "repository root containing rules")
	f.Float64Var(&threshold, "threshold", 0, "confidence threshold (default: 0.85)")
	f.StringVar(&format, "format", "table", "output format: table or json")
	return cmd
}

func runGuardCheck(ctx context.Context, out io.Writer, backend decide.Backend, repoRoot, filePath, proposedContent string, threshold float64, format string) error {
	switch strings.ToLower(format) {
	case "table", "json":
	default:
		return fmt.Errorf("unknown format %q: must be table or json", format)
	}

	v, err := guard.CheckFileEdit(ctx, backend, repoRoot, filePath, proposedContent, threshold)
	if err != nil {
		return err
	}

	if strings.ToLower(format) == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return err
		}
	} else {
		if v.Blocked {
			fmt.Fprintf(out, "Decision:   BLOCKED\n")
			fmt.Fprintf(out, "Confidence: %.2f\n", v.Confidence)
			if v.Rule != "" {
				fmt.Fprintf(out, "Violated:   %s\n", v.Rule)
			}
			if v.Reason != "" {
				fmt.Fprintf(out, "Reason:     %s\n", v.Reason)
			}
		} else {
			fmt.Fprintf(out, "Decision:   Allowed\n")
			fmt.Fprintf(out, "Confidence: %.2f\n", v.Confidence)
		}
	}

	if v.Blocked {
		return &exitCodeError{Code: 1}
	}
	return nil
}
