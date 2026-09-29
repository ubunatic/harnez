package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/skillreg"
)

// newSkillCmd manages external (third-party) skills (issue 631).
func newSkillCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install, search, and explore external agent skills from git repositories",
		Long: `Manage external skills: SKILL.md directories from third-party git repositories.

install clones the repository into ~/.harnez/skills/src (or $HARNEZ_SKILLS_HOME),
pins the commit, and copies only the skill directory into the agent skill
directories that 'harnez apply' uses. Plugin hooks, MCP servers, commands, and
agents are never installed; explore lists them. Nothing from the repository is
executed.

Skills are explicit-only by default: agents use one only when you name it.
Claude Code gets the skill as a /command that never triggers on its own,
Codex gets it with implicit invocation disabled, and other agents get no copy
(they load it via 'harnez skill show'). Pass --auto to install a plain copy
that every agent may trigger on its own.`,
	}
	cmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	open := func() (*skillreg.Registry, error) {
		cfg, _, err := claude.OpenConfig(configPath)
		if err != nil {
			return nil, err
		}
		root, err := skillreg.DefaultRoot()
		if err != nil {
			return nil, err
		}
		reg := &skillreg.Registry{Root: root}
		for _, sk := range cfg.Skills {
			reg.Reserved = append(reg.Reserved, sk.Name)
		}
		reg.Reserved = append(reg.Reserved, cfg.Decommissioned.Skills...)
		for _, t := range claude.SkillTargetsByAgent(cfg) {
			reg.Targets = append(reg.Targets, skillreg.Target{Agent: t.Agent, Dir: t.Dir})
		}
		return reg, nil
	}

	var path, name string
	var auto bool
	var as string
	install := &cobra.Command{
		Use:   "install <git-url>[@ref]",
		Short: "Install one skill from a git repository into all agent skill directories",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			url, ref := skillreg.ParseSource(args[0])
			e, err := reg.Install(skillreg.InstallOptions{URL: url, Ref: ref, Path: path, Name: name, Auto: auto, As: as})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "installed %s @ %.12s from %s (%s)\n", e.Name, e.Commit, e.URL, skillMode(e))
			for _, w := range e.Warnings {
				fmt.Fprintf(out, "  warning: %s\n", w)
			}
			for _, t := range reg.Targets {
				if _, err := os.Stat(filepath.Join(t.Dir, e.Name)); err == nil {
					fmt.Fprintf(out, "  %-7s %s\n", t.Agent, fsutil.ContractHome(filepath.Join(t.Dir, e.Name)))
				} else {
					fmt.Fprintf(out, "  %-7s no copy; use: harnez skill show %s\n", t.Agent, e.Name)
				}
			}
			return nil
		},
	}
	install.Flags().StringVar(&path, "path", "", "skill directory inside the repository")
	install.Flags().StringVar(&name, "skill", "", "skill name, when the repository has several")
	install.Flags().StringVar(&as, "as", "", "install under this name, e.g. mattp-grilling, when the upstream name is taken")
	install.Flags().BoolVar(&auto, "auto", false, "let agents trigger the skill on their own (default: only when named)")

	list := &cobra.Command{
		Use:   "list",
		Short: "List installed external skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			entries, err := reg.Load()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(out, "no external skills installed")
			}
			for _, e := range entries {
				origin := e.URL
				if e.Upstream != "" {
					origin += " (upstream name " + e.Upstream + ")"
				}
				fmt.Fprintf(out, "%-24s %.12s  %-8s %s\n", e.Name, e.Commit, skillMode(e), origin)
			}
			return nil
		},
	}

	update := &cobra.Command{
		Use:   "update [name...]",
		Short: "Reinstall external skills from their source (all when no name is given)",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				entries, err := reg.Load()
				if err != nil {
					return err
				}
				for _, e := range entries {
					args = append(args, e.Name)
				}
			}
			for _, n := range args {
				before, after, err := reg.Update(n)
				if err != nil {
					return err
				}
				state := "unchanged"
				if before.Commit != after.Commit {
					state = fmt.Sprintf("%.12s -> %.12s", before.Commit, after.Commit)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %s\n", n, state)
			}
			return nil
		},
	}

	remove := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an external skill from all agent skill directories",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			if err := reg.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}

	search := &cobra.Command{
		Use:   "search <terms...>",
		Short: "Search installed external skills and their sibling skills in cached repositories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			hits, err := reg.Search(strings.Join(args, " "))
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(hits) == 0 {
				fmt.Fprintln(out, "no matches")
			}
			for _, h := range hits {
				state := "available"
				if h.Installed {
					state = "installed"
				}
				fmt.Fprintf(out, "%-24s %-9s %s --path %s\n  %s\n", h.Skill.Name, state, h.URL, h.Skill.Path, oneLine(h.Skill.Description, 100))
			}
			return nil
		},
	}

	explore := &cobra.Command{
		Use:   "explore <name|git-url[@ref]>",
		Short: "Show a repository's skills, scripts, required keys, and plugin extras without running anything",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			dir, url, commit := "", "", ""
			if e, ok, err := reg.Get(args[0]); err != nil {
				return err
			} else if ok {
				dir, url, commit = reg.SrcDir(e.Name, e.Commit), e.URL, e.Commit
			} else {
				tmp, err := os.MkdirTemp("", "harnez-explore-")
				if err != nil {
					return err
				}
				defer os.RemoveAll(tmp)
				var ref string
				url, ref = skillreg.ParseSource(args[0])
				dir = filepath.Join(tmp, "repo")
				if commit, err = skillreg.Clone(url, ref, dir); err != nil {
					return err
				}
			}
			rep, err := skillreg.Explore(dir)
			if err != nil {
				return err
			}
			rep.URL, rep.Commit = url, commit
			printReport(cmd.OutOrStdout(), rep)
			return nil
		},
	}

	show := &cobra.Command{
		Use:   "show <name>",
		Short: "Print an installed external skill's SKILL.md (for agents without native skills)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := open()
			if err != nil {
				return err
			}
			e, ok, err := reg.Get(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("skill %q is not an installed external skill", args[0])
			}
			dir := filepath.Join(reg.SrcDir(e.Name, e.Commit), e.Path)
			data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "<!-- skill dir: %s -->\n%s", fsutil.ContractHome(dir), data)
			return nil
		},
	}

	cmd.AddCommand(install, list, update, remove, search, explore, show)
	return cmd
}

func skillMode(e skillreg.Entry) string {
	if e.Auto {
		return "auto"
	}
	return "explicit"
}

func printReport(out io.Writer, rep skillreg.Report) {
	fmt.Fprintf(out, "%s @ %.12s\n", rep.URL, rep.Commit)
	for _, s := range rep.Skills {
		fmt.Fprintf(out, "\nskill %s (%s), %d files\n  %s\n", s.Name, s.Path, s.Files, oneLine(s.Description, 160))
		if s.AllowedTools != "" {
			fmt.Fprintf(out, "  allowed-tools: %s\n", s.AllowedTools)
		}
		if len(s.Scripts) > 0 {
			fmt.Fprintf(out, "  scripts: %s\n", strings.Join(s.Scripts, ", "))
		}
		if s.Doctor != "" {
			fmt.Fprintf(out, "  dependency check (not run): %s\n", s.Doctor)
		}
	}
	if len(rep.EnvKeys) > 0 {
		fmt.Fprintf(out, "\nenv keys (.env.example): %s\n", strings.Join(rep.EnvKeys, ", "))
	}
	if len(rep.Plugin) > 0 {
		fmt.Fprintln(out, "\nplugin extras, never installed by harnez:")
		for _, p := range rep.Plugin {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
}

func oneLine(s string, width int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > width {
		return string(r[:width-1]) + "…"
	}
	return s
}
