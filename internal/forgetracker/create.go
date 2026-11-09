package forgetracker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CreateRequest is a forge ticket to file (issue 1195).
type CreateRequest struct {
	Title    string
	BodyFile string
	Labels   []string
}

// Command is the forge CLI call that files the ticket.
type Command struct {
	Tool string
	Args []string
}

// String renders the command for a dry run, quoting arguments with spaces.
func (c Command) String() string {
	parts := []string{c.Tool}
	for _, a := range c.Args {
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			a = strconv.Quote(a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// PlanCreate picks the first installed CLI from the forge's issue_cli list (spec/pr.yaml)
// and builds its create command. The command runs in the repo dir, so tea and fj find
// their login from the origin remote. lookPath is exec.LookPath in production.
func PlanCreate(t Target, req CreateRequest, lookPath func(string) (string, error)) (Command, error) {
	if strings.TrimSpace(req.Title) == "" {
		return Command{}, fmt.Errorf("issues new: a title is required to file on %s", t)
	}
	if req.BodyFile == "" {
		return Command{}, fmt.Errorf("issues new: -F/--body-file is required to file on %s", t)
	}
	clis := t.Forge.IssueCLI
	if len(clis) == 0 {
		return Command{}, fmt.Errorf("issues new: spec/pr.yaml lists no issue_cli for %s", t.Forge.Host)
	}
	slug := t.Owner + "/" + t.Repo
	for _, tool := range clis {
		if _, err := lookPath(tool); err != nil {
			continue
		}
		switch tool {
		case "gh":
			args := []string{"issue", "create", "--repo", t.Forge.Host + "/" + slug, "--title", req.Title, "--body-file", req.BodyFile}
			for _, l := range req.Labels {
				args = append(args, "--label", l)
			}
			return Command{Tool: tool, Args: args}, nil
		case "tea":
			args := []string{"issues", "create", "--repo", slug, "--title", req.Title, "--description-file", req.BodyFile}
			if len(req.Labels) > 0 {
				args = append(args, "--labels", strings.Join(req.Labels, ","))
			}
			return Command{Tool: tool, Args: args}, nil
		case "fj":
			if len(req.Labels) > 0 {
				// fj issue create has no label flag; try the next CLI.
				continue
			}
			return Command{Tool: tool, Args: []string{"--host", t.Forge.Host, "issue", "create", "--repo", slug, "--body-file", req.BodyFile, req.Title}}, nil
		default:
			return Command{}, fmt.Errorf("issues new: unsupported issue_cli %q for %s (supported: gh, tea, fj)", tool, t)
		}
	}
	msg := fmt.Sprintf("issues new: no forge CLI found for %s: install one of %s", t, strings.Join(clis, ", "))
	if len(req.Labels) > 0 {
		msg += " (fj cannot set labels)"
	}
	return Command{}, fmt.Errorf("%s", msg)
}

var (
	createdURLRe = regexp.MustCompile(`https?://\S+/issues/(\d+)`)
	createdNumRe = regexp.MustCompile(`#(\d+)`)
)

// ParseCreated reads the ticket number and URL from the CLI's output. When the output
// has only "#N", the URL is built from the target. The title is removed first, so a
// title such as "Follow-up to #182" cannot be read as the new ticket's number.
func ParseCreated(t Target, out, title string) (int, string, bool) {
	if title != "" {
		out = strings.ReplaceAll(out, title, "")
	}
	if m := createdURLRe.FindStringSubmatch(out); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, strings.TrimRight(m[0], ".,)"), true
	}
	if m := createdNumRe.FindStringSubmatch(out); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, fmt.Sprintf("https://%s/%s/%s/issues/%d", t.Forge.Host, t.Owner, t.Repo, n), true
	}
	return 0, "", false
}
