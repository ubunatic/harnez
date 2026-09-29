// decide implements `harnez decide`: typed decisions (noul, choice, score)
// from a decision model such as TypeSafe Jev. Backends come from
// spec/decide.yaml; see internal/decide and issue 633.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/decide"
)

type decideOpts struct {
	file      string
	backend   string
	model     string
	state     string
	stateFile string
	id        string
	noul      string
	choice    string
	score     string
	options   []string
	levels    []string
	pick      string
	threshold float64
	gate      bool // --threshold was given
	format    string
}

func newDecideCmd() *cobra.Command {
	var o decideOpts
	cmd := &cobra.Command{
		Use:   "decide [state text...]",
		Short: "Ask a decision model (Jev) typed questions: noul, choice, score",
		Long: `Ask a decision model typed questions about a state.

A request is a state plus named questions, from a spec file (-f, YAML or JSON,
"-" for stdin) or from one quick question (--noul, --choice or --score).
The state comes from --state (JSON or text), --state-file, the arguments,
or stdin, in that order; any of these replaces a state in the -f file, and
stdin is not read when the file has one.

Exit codes: 0 ok, 1 --threshold not met, 2 error.

Backends are defined in spec/decide.yaml; the default is Jev, which needs
$TYPESAFE_API_KEY.`,
		Example: `  harnez decide --noul "Does this need human confirmation?" \
    --option "true: destructive or rewrites remote history" --option "false: safe" \
    --state '{"command": "git push --force"}' --threshold 0.5

  harnez decide --choice "Classify the risk" --option "safe: read-only" \
    --option "danger: deletes data" --pick q "rm -rf build"

  harnez decide --score "How severe?" --level cosmetic --level degraded --level blocker \
    --state-file issue.md

  harnez decide -f review-gate.yaml --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.gate = cmd.Flags().Changed("threshold")
			err := runDecide(cmd, &o, args)
			var ec *exitCodeError
			if err != nil && !errors.As(err, &ec) {
				// Exit 2 on errors so a gate can tell "no" (1) from "broken".
				fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
				err = &exitCodeError{Code: 2}
			}
			return silenceIfExitCode(cmd, err)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&o.file, "file", "f", "", "request spec file (YAML or JSON, - for stdin)")
	f.StringVar(&o.backend, "backend", "", "backend from spec/decide.yaml (default: its default_backend)")
	f.StringVar(&o.model, "model", "", "model name sent to the backend")
	f.StringVar(&o.state, "state", "", "state as JSON, or plain text")
	f.StringVar(&o.stateFile, "state-file", "", "read the state as text from a file")
	f.StringVar(&o.id, "id", "q", "question id for --noul/--choice/--score")
	f.StringVar(&o.noul, "noul", "", "quick yes/no question; --option true: ..., false: ...")
	f.StringVar(&o.choice, "choice", "", "quick choice question; one --option name: description per choice")
	f.StringVar(&o.score, "score", "", "quick score question; one --level per rubric level, lowest first")
	f.StringArrayVar(&o.options, "option", nil, `criterion "name: description" (repeatable)`)
	f.StringArrayVar(&o.levels, "level", nil, "score level description (repeatable)")
	f.StringVar(&o.pick, "pick", "", "print only this question's answer (as JSON with --format json)")
	f.Float64Var(&o.threshold, "threshold", 0, "exit 1 unless the answer's level is >= this (noul/choice probability, score level)")
	f.StringVar(&o.format, "format", "table", "output: table, json, quiet")
	return cmd
}

func runDecide(cmd *cobra.Command, o *decideOpts, args []string) error {
	switch o.format {
	case "table", "json", "quiet":
	default:
		return fmt.Errorf("--format %q: want table, json or quiet", o.format)
	}
	req, err := decideRequest(o, args, cmd.InOrStdin())
	if err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	pick := o.pick
	if pick == "" && o.gate {
		if len(req.Questions) != 1 {
			return fmt.Errorf("--threshold with several questions needs --pick")
		}
		pick = req.QuestionIDs()[0]
	}
	if pick != "" {
		if _, ok := req.Questions[pick]; !ok {
			return fmt.Errorf("--pick %q: no such question", pick)
		}
	}
	spec, err := decide.LoadSpec()
	if err != nil {
		return err
	}
	backend, err := spec.Open(o.backend, nil)
	if err != nil {
		return err
	}
	resp, err := backend.Decide(context.Background(), req)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	switch {
	case o.format == "quiet":
	case o.format == "json":
		var v any = resp
		if o.pick != "" {
			v = resp.Answers[o.pick]
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return err
		}
	case o.pick != "":
		fmt.Fprintln(out, resp.Answers[o.pick].Value())
	default:
		printDecideTable(out, req, resp)
	}

	if o.gate {
		lv, ok := resp.Answers[pick].Level()
		if !ok {
			return fmt.Errorf("question %q: answer has no level to compare", pick)
		}
		if lv < o.threshold {
			return &exitCodeError{Code: 1}
		}
	}
	return nil
}

func printDecideTable(w io.Writer, req *decide.Request, resp *decide.Response) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "QUESTION\tTYPE\tANSWER\tLEVEL")
	for _, id := range req.QuestionIDs() {
		a := resp.Answers[id]
		level := ""
		if lv, ok := a.Level(); ok {
			level = fmt.Sprintf("%.2f", lv)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", id, a.Type, a.Value(), level)
	}
	tw.Flush()
}

// decideRequest builds the request from -f or from one quick question.
func decideRequest(o *decideOpts, args []string, stdin io.Reader) (*decide.Request, error) {
	quick := 0
	for _, q := range []string{o.noul, o.choice, o.score} {
		if q != "" {
			quick++
		}
	}
	if quick > 1 || (quick == 1 && o.file != "") {
		return nil, fmt.Errorf("use one of -f, --noul, --choice, --score")
	}
	usesStdin := o.file == "-"

	req := &decide.Request{}
	switch {
	case o.file != "":
		var data []byte
		var err error
		if usesStdin {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(o.file)
		}
		if err != nil {
			return nil, err
		}
		// YAML is a superset of JSON, so one decoder reads both.
		if err := yaml.Unmarshal(data, req); err != nil {
			return nil, fmt.Errorf("%s: %w", o.file, err)
		}
	case quick == 1:
		q, err := quickQuestion(o)
		if err != nil {
			return nil, err
		}
		req.Questions = map[string]decide.Question{o.id: q}
	default:
		return nil, fmt.Errorf("no question: use -f, --noul, --choice or --score")
	}
	if o.model != "" {
		req.Model = o.model
	}

	state, err := decideState(o, args, stdin, usesStdin || req.State != nil)
	if err != nil {
		return nil, err
	}
	if state != nil {
		req.State = state
	}
	if req.State == nil {
		return nil, fmt.Errorf("no state: use --state, --state-file, arguments or stdin")
	}
	return req, nil
}

func quickQuestion(o *decideOpts) (decide.Question, error) {
	if o.score != "" && len(o.options) > 0 {
		return decide.Question{}, fmt.Errorf("--score takes --level, not --option")
	}
	if o.score == "" && len(o.levels) > 0 {
		return decide.Question{}, fmt.Errorf("--level is only for --score")
	}
	criteria := func() (map[string]string, error) {
		m := map[string]string{}
		for _, opt := range o.options {
			name, desc, ok := strings.Cut(opt, ":")
			name = strings.TrimSpace(name)
			if !ok || name == "" {
				return nil, fmt.Errorf("--option %q: want \"name: description\"", opt)
			}
			m[name] = strings.TrimSpace(desc)
		}
		return m, nil
	}
	switch {
	case o.noul != "":
		m, err := criteria()
		if err != nil {
			return decide.Question{}, err
		}
		for k := range m {
			if k != "true" && k != "false" {
				return decide.Question{}, fmt.Errorf("--noul options are true and false, not %q", k)
			}
		}
		q := decide.Question{Type: decide.TypeNoul, Instructions: o.noul}
		if len(m) > 0 {
			q.Criteria = m
		}
		return q, nil
	case o.choice != "":
		m, err := criteria()
		if err != nil {
			return decide.Question{}, err
		}
		if len(m) < 2 {
			return decide.Question{}, fmt.Errorf("--choice needs at least two --option")
		}
		return decide.Question{Type: decide.TypeChoice, Instructions: o.choice, Criteria: m}, nil
	default:
		if len(o.levels) < 2 {
			return decide.Question{}, fmt.Errorf("--score needs at least two --level")
		}
		return decide.Question{Type: decide.TypeScore, Instructions: o.score, Criteria: o.levels}, nil
	}
}

// decideState returns the state from flags, args or stdin, or nil when none
// was given. skipStdin is set when stdin holds the -f file or the file
// already has a state, so a never-closing pipe cannot block.
func decideState(o *decideOpts, args []string, stdin io.Reader, skipStdin bool) (any, error) {
	switch {
	case o.state != "":
		var v any
		if json.Unmarshal([]byte(o.state), &v) == nil {
			return v, nil
		}
		return o.state, nil
	case o.stateFile != "":
		data, err := os.ReadFile(o.stateFile)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	case len(args) > 0:
		return strings.Join(args, " "), nil
	}
	if skipStdin || stdinIsTerminal(stdin) {
		return nil, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, err
	}
	if s := strings.TrimSpace(string(data)); s != "" {
		return s, nil
	}
	return nil, nil
}

func stdinIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
