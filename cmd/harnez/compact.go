// compact implements `harnez compact`, providing fast decision-model-powered
// verbatim transcript compaction. See issue 635.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/decide"
)

type compactOpts struct {
	transcript     string
	output         string
	backend        string
	model          string
	pinRecent      int
	truncateLength int
	threshold      float64
	maxStateBytes  int
	format         string
	inPlace        bool
}

func newCompactCmd() *cobra.Command {
	var o compactOpts
	cmd := &cobra.Command{
		Use:   "compact [transcript.jsonl]",
		Short: "Verbatim transcript compaction powered by decision models (Jev)",
		Long: `Fast, verbatim agent transcript compaction powered by decision models.

Evaluates historical tool calls and prunes or truncates obsolete outputs while
preserving conversation messages and recent context verbatim.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.transcript == "" && len(args) > 0 {
				o.transcript = args[0]
			}

			if o.inPlace && o.output != "" {
				return fmt.Errorf("cannot combine --in-place (-i) with --output (-o)")
			}

			if cmd.Flags().Changed("threshold") {
				if o.threshold < 0.0 || o.threshold > 1.0 {
					return fmt.Errorf("threshold %.2f out of bounds: must be between 0.0 and 1.0", o.threshold)
				}
			}

			spec, err := decide.LoadSpec()
			if err != nil {
				return fmt.Errorf("load decide spec: %w", err)
			}
			backend, err := spec.Open(o.backend, nil)
			if err != nil {
				return fmt.Errorf("open decide backend %q: %w", o.backend, err)
			}

			return runCompact(cmd.Context(), cmd.OutOrStdout(), backend, &o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.transcript, "transcript", "t", "", "input transcript file (- for stdin)")
	f.StringVarP(&o.output, "output", "o", "", "output file (defaults to stdout)")
	f.StringVar(&o.backend, "backend", "", "backend from spec/decide.yaml (default: jev)")
	f.StringVar(&o.model, "model", "", "model name sent to backend")
	f.IntVar(&o.pinRecent, "pin-recent", 5, "number of recent entries to pin verbatim")
	f.IntVar(&o.truncateLength, "truncate-length", 200, "maximum character length for truncated outputs")
	f.Float64Var(&o.threshold, "threshold", 0.50, "decision confidence threshold (0.0 - 1.0)")
	f.IntVar(&o.maxStateBytes, "max-state-bytes", 102400, "maximum byte size for decision request context state")
	f.StringVar(&o.format, "format", "jsonl", "output format: jsonl, summary, or json")
	f.BoolVarP(&o.inPlace, "in-place", "i", false, "overwrite the input transcript file in-place")
	return cmd
}

func runCompact(ctx context.Context, out io.Writer, backend decide.Backend, o *compactOpts) error {
	if o.format == "" {
		o.format = "jsonl"
	}
	switch strings.ToLower(o.format) {
	case "jsonl", "summary", "json":
	default:
		return fmt.Errorf("unknown format %q: must be jsonl, summary, or json", o.format)
	}

	if o.inPlace && (o.transcript == "" || o.transcript == "-") {
		return fmt.Errorf("cannot use --in-place (-i) with stdin")
	}

	var r io.Reader
	if o.transcript == "" || o.transcript == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(o.transcript)
		if err != nil {
			return fmt.Errorf("open transcript: %w", err)
		}
		defer f.Close()
		r = f
	}

	entries, err := decide.ParseJSONLTranscript(r)
	if err != nil {
		return fmt.Errorf("parse transcript: %w", err)
	}

	compactOpts := decide.CompactOptions{
		Model:          o.model,
		PinRecent:      o.pinRecent,
		Threshold:      o.threshold,
		TruncateLength: o.truncateLength,
		MaxStateBytes:  o.maxStateBytes,
	}

	compacted, report, err := decide.CompactTranscript(ctx, backend, entries, compactOpts)
	if err != nil {
		return fmt.Errorf("compact transcript: %w", err)
	}

	var dest io.Writer = out
	var closeDest func() error

	if o.output != "" {
		f, err := os.Create(o.output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		dest = f
		closeDest = f.Close
	}
	if closeDest != nil {
		defer closeDest()
	}

	if strings.ToLower(o.format) == "summary" {
		fmt.Fprintln(dest, "Compaction Summary")
		fmt.Fprintln(dest, "==================")
		fmt.Fprintf(dest, "Original entries:   %d\n", report.OriginalEntries)
		fmt.Fprintf(dest, "Compacted entries:  %d\n", report.CompactedEntries)
		fmt.Fprintf(dest, "Original bytes:     %d\n", report.OriginalBytes)
		fmt.Fprintf(dest, "Compacted bytes:    %d\n", report.CompactedBytes)
		fmt.Fprintf(dest, "Reduction ratio:    %.1f%%\n", report.ReductionRatio*100)
		fmt.Fprintf(dest, "Kept verbatim:      %d\n", report.KeptVerbatim)
		fmt.Fprintf(dest, "Truncated results:  %d\n", report.TruncatedResults)
		fmt.Fprintf(dest, "Excised tools:      %d\n", report.ExcisedTools)
		return nil
	}

	if strings.ToLower(o.format) == "json" {
		enc := json.NewEncoder(dest)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	// Format is jsonl
	if o.inPlace && o.transcript != "" && o.transcript != "-" {
		var buf bytes.Buffer
		if err := decide.WriteJSONLTranscript(&buf, compacted); err != nil {
			return fmt.Errorf("encode transcript: %w", err)
		}
		return os.WriteFile(o.transcript, buf.Bytes(), 0644)
	}

	if err := decide.WriteJSONLTranscript(dest, compacted); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	return nil
}
