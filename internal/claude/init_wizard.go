package claude

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type initWizardSpec struct {
	Title   string `yaml:"title"`
	Invalid string `yaml:"invalid"`
	Prompts []struct {
		Key         string `yaml:"key"`
		Label       string `yaml:"label"`
		Explanation string `yaml:"explanation"`
	} `yaml:"prompts"`
}

// promptInitConfig is independent of terminals and files. It returns a copy,
// retaining defaults on EOF and leaving the input config untouched on failure.
func promptInitConfig(current *RepoConfig, detectedDocs []string, input io.Reader, output io.Writer) (*RepoConfig, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/init-wizard.yaml")
	if err != nil {
		return nil, err
	}
	var spec initWizardSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	next := *current
	next.Docs.Copies = append([]string{}, current.Docs.Copies...)
	next.Issues.MergeDriver = next.Issues.Tracker == "harnez"
	candidates := mergeDocs(next.Docs.Copies, detectedDocs)
	if _, err := fmt.Fprintln(output, spec.Title); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(input)
	for _, prompt := range spec.Prompts {
		if prompt.Key == "merge_driver" && next.Issues.Tracker != "harnez" {
			next.Issues.MergeDriver = false
			continue
		}
		var enabled bool
		switch prompt.Key {
		case "tracker":
			enabled = next.Issues.Tracker == "harnez"
		case "studies":
			enabled = next.Docs.Studies
		case "merge_driver":
			enabled = next.Issues.MergeDriver
		case "copies":
			enabled = len(next.Docs.Copies) > 0
		case "scaffold":
			enabled = next.Make.Scaffold
		case "quota":
			enabled = next.Rules.Quota
		}
		hint := "y/N"
		if enabled {
			hint = "Y/n"
		}
		for {
			if _, err := fmt.Fprintf(output, "%s — %s [%s]: ", prompt.Label, prompt.Explanation, hint); err != nil {
				return nil, err
			}
			answer, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return nil, err
			}
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "":
			case "y", "yes", "on":
				enabled = true
			case "n", "no", "off":
				enabled = false
			default:
				if _, writeErr := fmt.Fprintln(output, spec.Invalid); writeErr != nil {
					return nil, writeErr
				}
				if err != io.EOF {
					continue
				}
			}
			break
		}
		switch prompt.Key {
		case "tracker":
			next.Issues.Tracker = "none"
			if enabled {
				next.Issues.Tracker = "harnez"
			}
			next.Issues.MergeDriver = enabled
		case "studies":
			next.Docs.Studies = enabled
		case "merge_driver":
			next.Issues.MergeDriver = enabled
		case "copies":
			next.Docs.Copies = []string{}
			if enabled {
				selected := current.Docs.Copies
				if len(selected) == 0 {
					selected = candidates
				}
				next.Docs.Copies = append(next.Docs.Copies, selected...)
				if len(next.Docs.Copies) == 0 {
					if _, err := fmt.Fprintln(output, "No optional docs are detected or recorded; add one with --docs <name>."); err != nil {
						return nil, err
					}
				}
			}
		case "scaffold":
			next.Make.Scaffold = enabled
		case "quota":
			next.Rules.Quota = enabled
		}
	}
	return &next, nil
}

func writeInitWizardConfig(dir string, current *RepoConfig, detectedDocs []string, input io.Reader, output io.Writer) (*RepoConfig, error) {
	next, err := promptInitConfig(current, detectedDocs, input, output)
	if err != nil {
		return nil, err
	}
	if err := writeRepoConfig(dir, next); err != nil {
		return nil, err
	}
	return next, nil
}
