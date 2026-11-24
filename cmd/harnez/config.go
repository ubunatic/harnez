package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/access"
	"ubunatic.com/harnez/internal/repocfg"
)

func newConfigCmd() *cobra.Command {
	var dir string
	var unset, list, showAccess bool
	cmd := &cobra.Command{
		Use:   "config [-d <repo>] (<key> [<value>] | --unset <key> | --list | --access)",
		Short: "Get, set or unset keys of .harnez/repo.yaml, like git config",
		Long: `config reads and changes the per-repo settings in <repo>/.harnez/repo.yaml
without hand-editing YAML.

  harnez config git.mode                  print the effective value
  harnez config git.mode team             set a key
  harnez config --unset git.mode          remove a key
  harnez config --list                    print every effective key=value
  harnez config --access                  print the directories this mode needs,
                                          and whether this process can reach them

Keys are dotted paths (git.mode, issues.tracker, docs.copies, ...). The key
list, value types and allowed values come from the repo.yaml schema. Values
are parsed by type: true/false, an integer, one of the allowed words, or free
text. A list key such as docs.copies takes comma-separated items and replaces
the whole list (docs.copies golang,bash; an empty value gives an empty list).
A negative number needs -- before it, as in: harnez config <key> -- -1.

A set or unset is checked with the same rules as loading repo.yaml (schema,
required keys, git.protected_main needs git.mode team or fork). When a check
fails nothing is written and the error names the key and the allowed values.
Comments, key order and unrelated keys survive the edit.

--list shows spec defaults for keys the file leaves out, marked (default).
After a set or unset of a key that changes rendered output (AGENTS.md, rules,
Makefile), config prints "run: harnez init -d <repo>"; it does not run init.
get and --list also work without a repo.yaml; set and unset need one.
--access takes no key and prints the access manifest for the current mode.`,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			switch {
			case showAccess && (list || unset || len(args) > 0):
				return errors.New("--access takes no key and cannot be combined with --list or --unset")
			case list && unset:
				return errors.New("--list and --unset cannot be combined")
			case list && len(args) > 0:
				return errors.New("--list takes no key")
			case unset && len(args) != 1:
				return errors.New("--unset takes exactly one key")
			case !list && !showAccess && (len(args) == 0 || len(args) > 2):
				return errors.New("usage: harnez config <key> [<value>] | --unset <key> | --list | --access")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showAccess {
				return runAccessCmd(cmd, dir)
			}
			return runConfigCmd(cmd, dir, args, unset, list)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing .harnez/repo.yaml")
	cmd.Flags().BoolVar(&unset, "unset", false, "remove the key")
	cmd.Flags().BoolVar(&list, "list", false, "print every effective key=value, spec defaults marked (default)")
	cmd.Flags().BoolVar(&showAccess, "access", false, "print the directories this mode needs and whether they are reachable")
	return cmd
}

// runAccessCmd prints the access manifest rows for the detected mode.
func runAccessCmd(cmd *cobra.Command, dir string) error {
	repo, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	spec, err := access.Load()
	if err != nil {
		return err
	}
	mode, reason := spec.Detect(repo)
	return access.Render(cmd.OutOrStdout(), mode, reason, spec.Rows(mode, repo))
}

func runConfigCmd(cmd *cobra.Command, dir string, args []string, unset, list bool) error {
	out := cmd.OutOrStdout()
	if list {
		entries, err := repocfg.List(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			fmt.Fprintln(out, e)
		}
		return nil
	}
	key, err := repocfg.Lookup(args[0])
	if err != nil {
		return err
	}
	var res repocfg.Result
	switch {
	case unset:
		res, err = repocfg.Unset(dir, key)
	case len(args) == 2:
		res, err = repocfg.Set(dir, key, args[1])
	default:
		e, err := repocfg.Get(dir, key)
		if err != nil {
			return err
		}
		if !e.Set {
			return fmt.Errorf("%s is not set", key.Path)
		}
		fmt.Fprintln(out, e.Value)
		return nil
	}
	if err != nil {
		return err
	}
	if res.NeedsInit {
		fmt.Fprintf(out, "run: harnez init -d %s\n", dir)
	}
	return nil
}
