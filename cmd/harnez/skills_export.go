package main

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/claude"
)

type sitegenExport struct {
	FormatVersion int            `yaml:"format_version"`
	HarnezCommit  string         `yaml:"harnez_commit"`
	Skills        []sitegenSkill `yaml:"skills"`
}
type sitegenSkill struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Origin      string         `yaml:"origin"`
	Revision    string         `yaml:"source_revision"`
	License     string         `yaml:"license"`
	SourcePath  string         `yaml:"source_path"`
	Requires    []string       `yaml:"requires"`
	Targets     []string       `yaml:"targets"`
	Invocation  []string       `yaml:"invocation"`
	Content     sitegenContent `yaml:"content"`
}
type sitegenContent struct {
	Kind      string            `yaml:"kind"`
	Body      string            `yaml:"body"`
	Resources map[string]string `yaml:"resources,omitempty"`
}

func sitegenContentKind(kind string) (string, error) {
	switch kind {
	case "file-backed", "inline", "resource-backed":
		return kind, nil
	default:
		return "", fmt.Errorf("unknown sitegen skill content kind %q", kind)
	}
}

func encodeSitegenSkillExport(cfg *claude.Config, revision string) ([]byte, int, error) {
	if len(revision) != 40 {
		return nil, 0, fmt.Errorf("missing Harnez source revision")
	}
	out := sitegenExport{FormatVersion: 1, HarnezCommit: revision}
	for _, sk := range cfg.Skills {
		kind := "file-backed"
		if sk.File == "" {
			kind = "inline"
		}
		if len(sk.Resources) > 0 {
			kind = "resource-backed"
		}
		kind, err := sitegenContentKind(kind)
		if err != nil {
			return nil, 0, err
		}
		license := strings.TrimSpace(sk.License)
		path := sk.File
		body := sk.Content
		if path == "" {
			path = "config.yaml:skills." + sk.Name
		}
		if sk.File != "" {
			data, err := fs.ReadFile(cfg.FS, sk.File)
			if err != nil {
				return nil, 0, fmt.Errorf("skill %s source %s: %w", sk.Name, sk.File, err)
			}
			body = string(data)
		}
		resources := map[string]string{}
		for _, resource := range sk.Resources {
			data, err := fs.ReadFile(cfg.FS, resource.Source)
			if err != nil {
				return nil, 0, fmt.Errorf("skill %s resource %s: %w", sk.Name, resource.Source, err)
			}
			resources[resource.Target] = string(data)
		}
		if license == "" {
			return nil, 0, fmt.Errorf("skill %s has no license", sk.Name)
		}
		out.Skills = append(out.Skills, sitegenSkill{Name: sk.Name, Description: sk.Description, Origin: "first-party", Revision: revision, License: license, SourcePath: path, Requires: append([]string(nil), sk.Requires...), Targets: []string{"codex", "claude"}, Invocation: []string{"/" + sk.Name}, Content: sitegenContent{Kind: kind, Body: body, Resources: resources}})
	}
	for _, source := range cfg.BundledSkills {
		if source.URL == "" || source.Commit == "" {
			return nil, 0, fmt.Errorf("bundled skill source %s is missing provenance", source.Dir)
		}
		for _, name := range source.Skills {
			path := strings.TrimSuffix(source.Dir, "/") + "/" + name + "/SKILL.md"
			data, err := fs.ReadFile(cfg.FS, path)
			if err != nil {
				return nil, 0, fmt.Errorf("bundled skill %s: %w", name, err)
			}
			licensePath := strings.TrimSuffix(source.Dir, "/") + "/LICENSE"
			licenseData, err := fs.ReadFile(cfg.FS, licensePath)
			if err != nil {
				return nil, 0, fmt.Errorf("bundled skill %s has no license file: %w", name, err)
			}
			license := ""
			if strings.Contains(string(licenseData), "MIT License") {
				license = "MIT"
			}
			if license == "" {
				return nil, 0, fmt.Errorf("bundled skill %s has an unrecognized license", name)
			}
			body, description := string(data), name
			if strings.HasPrefix(body, "---\n") {
				if end := strings.Index(body[4:], "\n---"); end >= 0 {
					front := body[4 : 4+end]
					for _, line := range strings.Split(front, "\n") {
						if strings.HasPrefix(line, "description:") {
							description = strings.Trim(strings.TrimPrefix(line, "description:"), " \"'")
						}
					}
					body = strings.TrimSpace(body[4+end+4:])
				}
			}
			out.Skills = append(out.Skills, sitegenSkill{Name: name, Description: description, Origin: "bundled", Revision: source.Commit, License: license, SourcePath: path, Targets: []string{"codex", "claude"}, Invocation: []string{"/" + name}, Content: sitegenContent{Kind: "file-backed", Body: body}})
		}
	}
	sort.Slice(out.Skills, func(i, j int) bool { return out.Skills[i].Name < out.Skills[j].Name })
	data, err := yaml.Marshal(out)
	return data, len(out.Skills), err
}

func newSkillsCmd() *cobra.Command {
	root := &cobra.Command{Use: "skills", Short: "Inspect and export Harnez's built-in skill catalogue"}
	export := &cobra.Command{Use: "export", Short: "Export public skills for static site generation", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		format, _ := cmd.Flags().GetString("format")
		if format != "sitegen-v1" {
			return fmt.Errorf("unsupported skills export format %q", format)
		}
		cfg, err := claude.LoadConfigEmbedded()
		if err != nil {
			return err
		}
		revision, err := buildRevision()
		if err != nil {
			return err
		}
		data, count, err := encodeSitegenSkillExport(cfg, revision)
		if err != nil {
			return err
		}
		if _, err = cmd.OutOrStdout().Write(data); err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.ErrOrStderr(), "exported %d skills\n", count)
		return err
	}}
	export.Flags().String("format", "", "export format (sitegen-v1)")
	root.AddCommand(export)
	return root
}

// buildRevision requires Go's clean VCS metadata. Falling back to the checkout
// HEAD can pin a different commit than the installed binary contains.
func buildRevision() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("missing Harnez build metadata")
	}
	return sitegenRevisionFromSettings(info.Settings)
}

func sitegenRevisionFromSettings(settings []debug.BuildSetting) (string, error) {
	revision, modified := "", ""
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if modified != "false" {
		return "", fmt.Errorf("refusing skill export from modified or revision-less Harnez build (vcs.modified=%q)", modified)
	}
	if len(revision) != 40 {
		return "", fmt.Errorf("refusing skill export from revision-less Harnez build")
	}
	decoded, err := hex.DecodeString(revision)
	if err != nil || len(decoded) != 20 {
		return "", fmt.Errorf("invalid Harnez build revision %q", revision)
	}
	return revision, nil
}
