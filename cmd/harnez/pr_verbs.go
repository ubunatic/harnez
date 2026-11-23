package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

func resolvePRRepo(spec prwait.Spec, remote, repoFlag string) (forge.Forge, string, string, error) {
	remoteName := remote
	if remoteName == "" {
		remoteName = spec.DefaultRemote
	}
	out, err := exec.Command("git", "remote", "get-url", remoteName).Output()
	if err != nil {
		return forge.Forge{}, "", "", fmt.Errorf("git remote %q: %w", remoteName, err)
	}
	remoteURL := strings.TrimSpace(string(out))
	host, owner, repo, ok := forge.ParseRemoteURL(remoteURL)
	if !ok {
		return forge.Forge{}, "", "", fmt.Errorf("remote URL %q is not an ssh or https forge URL", remoteURL)
	}
	if repoFlag != "" {
		o, r, ok := strings.Cut(strings.TrimSpace(repoFlag), "/")
		if !ok || o == "" || r == "" {
			return forge.Forge{}, "", "", fmt.Errorf("invalid --repo %q (want owner/repo)", repoFlag)
		}
		owner, repo = o, r
	}
	fg, ok := spec.ForgeFor(host)
	if !ok {
		return forge.Forge{}, "", "", fmt.Errorf("forge host %q is not configured in spec/pr.yaml", host)
	}
	return fg, owner, repo, nil
}

func resolvePRTargetArg(spec prwait.Spec, arg, remote, repoFlag string) (prwait.Target, error) {
	if strings.Contains(arg, "://") {
		return prwait.ResolveTarget(spec, arg, "")
	}
	fg, owner, repo, err := resolvePRRepo(spec, remote, repoFlag)
	if err != nil {
		return prwait.Target{}, err
	}
	t, err := prwait.ResolveTarget(spec, arg, fmt.Sprintf("https://%s/%s/%s.git", fg.Host, owner, repo))
	if err != nil {
		return prwait.Target{}, err
	}
	return t, nil
}

func newPRClient(spec prwait.Spec) forge.PRClient {
	ts := forge.SystemTokenSource(os.Getenv)
	return forge.PRClient{
		Client:     &http.Client{Timeout: spec.RequestTimeout},
		ReadToken:  ts.For,
		WriteToken: forge.SystemWriteToken(os.Getenv),
	}
}

func readBodyArgOrFile(inline, filePath string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return inline, nil
}

func currentGitBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("detect current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD; pass --head <branch>")
	}
	return branch, nil
}

func newPRListCmd() *cobra.Command {
	var state, remote, repoFlag string
	var limit int
	var asJSON bool

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List pull requests on the origin forge",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			fg, owner, repo, err := resolvePRRepo(spec, remote, repoFlag)
			if err != nil {
				return err
			}
			pulls, err := newPRClient(spec).ListPulls(fg, owner, repo, state, limit)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(pulls)
			}
			fmt.Fprint(cmd.OutOrStdout(), forge.FormatPullList(pulls))
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "open", "filter by state: open, closed, or all")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of pull requests to return")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output pull requests as JSON")
	return cmd
}

func newPRShowCmd() *cobra.Command {
	var remote, repoFlag string
	var showComments, showFiles, asJSON bool

	cmd := &cobra.Command{
		Use:          "show <number|url>",
		Short:        "Show details of a pull request",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			cli := newPRClient(spec)
			pr, err := cli.GetPull(target.Forge, target.Owner, target.Repo, target.Number)
			if err != nil {
				return err
			}
			var comments []forge.PRComment
			if showComments {
				comments, err = cli.ListPullComments(target.Forge, target.Owner, target.Repo, target.Number)
				if err != nil {
					return err
				}
			}
			var files []forge.PRFile
			if showFiles {
				files, err = cli.ListPullFiles(target.Forge, target.Owner, target.Repo, target.Number)
				if err != nil {
					return err
				}
			}
			if asJSON {
				payload := struct {
					PullRequest forge.PullRequest `json:"pull_request"`
					Comments    []forge.PRComment `json:"comments,omitempty"`
					Files       []forge.PRFile    `json:"files,omitempty"`
				}{PullRequest: pr, Comments: comments, Files: files}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(payload)
			}
			fmt.Fprint(cmd.OutOrStdout(), forge.FormatPullDetail(pr, comments, files))
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&showComments, "comments", false, "include pull request comments")
	cmd.Flags().BoolVar(&showFiles, "files", false, "include changed files")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output pull request details as JSON")
	return cmd
}

func newPRCommentCmd() *cobra.Command {
	var bodyFile, remote, repoFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:          "comment <number|url> [body]",
		Short:        "Add a comment to a pull request",
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			inline := ""
			if len(args) == 2 {
				inline = args[1]
			}
			body, err := readBodyArgOrFile(inline, bodyFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("comment body is required (pass [body] or -F <file>)")
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would comment on %s (%d bytes)\n", target.PageURL(), len(body))
				return nil
			}
			url, err := newPRClient(spec).CommentPull(target.Forge, target.Owner, target.Repo, target.Number, body)
			if err != nil {
				return err
			}
			if url != "" {
				fmt.Fprintln(cmd.OutOrStdout(), url)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "commented on #%d\n", target.Number)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "read comment body from file")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without posting the comment")
	return cmd
}

func newPRCreateCmd() *cobra.Command {
	var title, body, bodyFile, head, base, remote, repoFlag string
	var draft, dryRun bool

	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a pull request on the origin forge",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			fg, owner, repo, err := resolvePRRepo(spec, remote, repoFlag)
			if err != nil {
				return err
			}
			if head == "" {
				head, err = currentGitBranch()
				if err != nil {
					return err
				}
			}
			if base == "" {
				base = spec.DefaultBranch
			}
			bodyText, err := readBodyArgOrFile(body, bodyFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("--title is required")
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would create PR on %s/%s (%s -> %s): %s\n", owner, repo, head, base, title)
				return nil
			}
			pr, err := newPRClient(spec).CreatePull(fg, owner, repo, forge.CreatePullInput{
				Title: title,
				Body:  bodyText,
				Head:  head,
				Base:  base,
				Draft: draft,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d %s\n", pr.Number, pr.HTMLURL)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "pull request title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "pull request body")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "read pull request body from file")
	cmd.Flags().StringVar(&head, "head", "", "head branch (default: current git branch)")
	cmd.Flags().StringVar(&base, "base", "", "base branch (default: default_branch in spec/pr.yaml)")
	cmd.Flags().BoolVar(&draft, "draft", false, "create as a draft pull request")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without creating the pull request")
	return cmd
}

func newPREditCmd() *cobra.Command {
	var title, body, bodyFile, base, remote, repoFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:          "edit <number|url>",
		Short:        "Edit a pull request's title, body, or base branch",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			var in forge.EditPullInput
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if cmd.Flags().Changed("body") || bodyFile != "" {
				text, err := readBodyArgOrFile(body, bodyFile)
				if err != nil {
					return err
				}
				in.Body = &text
			}
			if cmd.Flags().Changed("base") {
				in.Base = &base
			}
			if in.Title == nil && in.Body == nil && in.Base == nil {
				return fmt.Errorf("specify at least one of --title, --body, -F/--body-file, or --base")
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would edit %s\n", target.PageURL())
				return nil
			}
			pr, err := newPRClient(spec).EditPull(target.Forge, target.Owner, target.Repo, target.Number, in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d updated (%s)\n", pr.Number, pr.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "new pull request title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "new pull request body")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "read new pull request body from file")
	cmd.Flags().StringVar(&base, "base", "", "new base branch")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without editing the pull request")
	return cmd
}

func newPRCloseCmd() *cobra.Command {
	var remote, repoFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:          "close <number|url> [comment]",
		Short:        "Close a pull request",
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			comment := ""
			if len(args) == 2 {
				comment = strings.TrimSpace(args[1])
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would close %s\n", target.PageURL())
				return nil
			}
			cli := newPRClient(spec)
			if comment != "" {
				if _, err := cli.CommentPull(target.Forge, target.Owner, target.Repo, target.Number, comment); err != nil {
					return err
				}
			}
			closed := "closed"
			if _, err := cli.EditPull(target.Forge, target.Owner, target.Repo, target.Number, forge.EditPullInput{State: &closed}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d closed\n", target.Number)
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without closing the pull request")
	return cmd
}

func newPRReopenCmd() *cobra.Command {
	var remote, repoFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:          "reopen <number|url>",
		Short:        "Reopen a closed pull request",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would reopen %s\n", target.PageURL())
				return nil
			}
			open := "open"
			if _, err := newPRClient(spec).EditPull(target.Forge, target.Owner, target.Repo, target.Number, forge.EditPullInput{State: &open}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d reopened\n", target.Number)
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without reopening the pull request")
	return cmd
}

func newPRMergeCmd() *cobra.Command {
	var method, remote, repoFlag string
	var deleteBranch, dryRun bool

	cmd := &cobra.Command{
		Use:          "merge <number|url>",
		Short:        "Merge a pull request",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := forge.CheckHostWrite(os.Getenv); err != nil {
				return err
			}
			spec, err := prwait.LoadSpec()
			if err != nil {
				return err
			}
			target, err := resolvePRTargetArg(spec, args[0], remote, repoFlag)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would merge %s (method=%s)\n", target.PageURL(), method)
				return nil
			}
			err = newPRClient(spec).MergePull(target.Forge, target.Owner, target.Repo, target.Number, forge.MergePullInput{
				Method:       method,
				DeleteBranch: deleteBranch,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%d merged (%s)\n", target.Number, method)
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "merge", "merge method: merge, squash, or rebase")
	cmd.Flags().BoolVar(&deleteBranch, "delete-branch", false, "delete head branch after merge (Forgejo)")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote whose URL names the forge and repository")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "override repository as owner/repo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without merging the pull request")
	return cmd
}
