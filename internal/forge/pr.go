package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/forgetoken"
)

// PullRequest is a normalized pull request returned by Forgejo or GitHub.
type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Draft     bool   `json:"draft"`
	Merged    bool   `json:"merged"`
	Mergeable *bool  `json:"mergeable,omitempty"`
	HTMLURL   string `json:"html_url"`
	Author    string `json:"author"`
	HeadRef   string `json:"head_ref"`
	BaseRef   string `json:"base_ref"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Comments  int    `json:"comments,omitempty"`
}

// PRComment is one issue/PR comment.
type PRComment struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// PRFile is one changed file in a pull request.
type PRFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
}

// CreatePullInput holds parameters for creating a pull request.
type CreatePullInput struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft,omitempty"`
}

// EditPullInput holds optional fields to update on a pull request.
type EditPullInput struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
	Base  *string `json:"base,omitempty"`
	State *string `json:"state,omitempty"`
}

// MergePullInput holds parameters for merging a pull request.
type MergePullInput struct {
	Method       string // "merge", "squash", "rebase"
	DeleteBranch bool
}

// PRClient performs read and write pull-request operations against Forgejo or GitHub.
type PRClient struct {
	Client     *http.Client
	ReadToken  func(Forge) string
	WriteToken func(Forge) (token, source string)
}

// SystemWriteToken resolves the effective write token for a forge:
// stored write token (Forgejo) -> token_env -> gh auth token (GitHub).
func SystemWriteToken(getenv func(string) string) func(Forge) (string, string) {
	return func(fg Forge) (string, string) {
		if fg.Kind == "forgejo" {
			if tok := strings.TrimSpace(forgetoken.LookupWrite(fg.Host)); tok != "" {
				return tok, "stored write token"
			}
		}
		for _, name := range fg.TokenEnv {
			if tok := strings.TrimSpace(getenv(name)); tok != "" {
				return tok, "env " + name
			}
		}
		if fg.Kind == "github" {
			if tok, err := ghAuthToken(); err == nil {
				if tok = strings.TrimSpace(tok); tok != "" {
					return tok, "gh auth token"
				}
			}
		}
		return "", ""
	}
}

// CheckHostWrite refuses forge write operations inside a harnez worker session.
func CheckHostWrite(getenv func(string) string) error {
	if strings.TrimSpace(getenv("HARNEZ_AGENT_ROLE")) != "" || strings.TrimSpace(getenv("HARNEZ_SESSION_ID")) != "" {
		return fmt.Errorf("forge writes run only in a host session")
	}
	return nil
}

type rawPull struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Draft     bool   `json:"draft"`
	Merged    bool   `json:"merged"`
	MergedAt  string `json:"merged_at"`
	Mergeable *bool  `json:"mergeable"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Comments  int    `json:"comments"`
	User      struct {
		Login    string `json:"login"`
		Username string `json:"username"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (r rawPull) normalize() PullRequest {
	author := r.User.Login
	if author == "" {
		author = r.User.Username
	}
	return PullRequest{
		Number:    r.Number,
		Title:     r.Title,
		Body:      r.Body,
		State:     r.State,
		Draft:     r.Draft,
		Merged:    r.Merged || r.MergedAt != "",
		Mergeable: r.Mergeable,
		HTMLURL:   r.HTMLURL,
		Author:    author,
		HeadRef:   r.Head.Ref,
		BaseRef:   r.Base.Ref,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Comments:  r.Comments,
	}
}

func (c PRClient) readToken(fg Forge) string {
	if c.ReadToken == nil {
		return ""
	}
	return c.ReadToken(fg)
}

func (c PRClient) repoBase(fg Forge, owner, repo string) string {
	return strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

// ListPulls lists pull requests for owner/repo filtered by state ("open", "closed", "all").
func (c PRClient) ListPulls(fg Forge, owner, repo, state string, limit int) ([]PullRequest, error) {
	if state == "" {
		state = "open"
	}
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("state", state)
	q.Set(pageParam(fg), strconv.Itoa(limit))
	q.Set("page", "1")
	endpoint := c.repoBase(fg, owner, repo) + "/pulls?" + q.Encode()
	var raw []rawPull
	if err := GetJSON(c.Client, fg, endpoint, c.readToken(fg), "API request to "+fg.Host, "", &raw); err != nil {
		return nil, err
	}
	out := make([]PullRequest, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.normalize())
	}
	return out, nil
}

// GetPull fetches a single pull request by number.
func (c PRClient) GetPull(fg Forge, owner, repo string, number int) (PullRequest, error) {
	endpoint := fmt.Sprintf("%s/pulls/%d", c.repoBase(fg, owner, repo), number)
	var raw rawPull
	if err := GetJSON(c.Client, fg, endpoint, c.readToken(fg), "API request to "+fg.Host, "", &raw); err != nil {
		return PullRequest{}, err
	}
	return raw.normalize(), nil
}

// ListPullComments fetches comments on a pull request (via the issues comments endpoint).
func (c PRClient) ListPullComments(fg Forge, owner, repo string, number int) ([]PRComment, error) {
	endpoint := fmt.Sprintf("%s/issues/%d/comments", c.repoBase(fg, owner, repo), number)
	var raw []struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		HTMLURL   string `json:"html_url"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := GetJSON(c.Client, fg, endpoint, c.readToken(fg), "API request to "+fg.Host, "", &raw); err != nil {
		return nil, err
	}
	out := make([]PRComment, 0, len(raw))
	for _, r := range raw {
		author := r.User.Login
		if author == "" {
			author = r.User.Username
		}
		out = append(out, PRComment{
			ID:        r.ID,
			Author:    author,
			Body:      r.Body,
			HTMLURL:   r.HTMLURL,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// ListPullFiles fetches changed files in a pull request.
func (c PRClient) ListPullFiles(fg Forge, owner, repo string, number int) ([]PRFile, error) {
	endpoint := fmt.Sprintf("%s/pulls/%d/files", c.repoBase(fg, owner, repo), number)
	var files []PRFile
	if err := GetJSON(c.Client, fg, endpoint, c.readToken(fg), "API request to "+fg.Host, "", &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c PRClient) writeRequest(fg Forge, method, endpoint string, payload, result any) error {
	if c.Client == nil {
		return fmt.Errorf("forge write: no HTTP client")
	}
	var tok, src string
	if c.WriteToken != nil {
		tok, src = c.WriteToken(fg)
	}
	if tok == "" {
		return fmt.Errorf("no write token configured for %s (run harnez forge status or set %s)", fg.Host, strings.Join(fg.TokenEnv, "/"))
	}
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("forge write: invalid request")
	}
	if fg.Kind == "github" {
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
	} else {
		req.Header.Set("Authorization", "token "+tok)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("forge write request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := ""
		var msg struct {
			Message string `json:"message"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&msg) == nil && safeWriteDetail(msg.Message, tok, data) {
			detail = strings.Join(strings.Fields(msg.Message), " ")
			if runes := []rune(detail); len(runes) > 200 {
				detail = string(runes[:200])
			}
		}
		var baseErr error
		if detail == "" {
			baseErr = fmt.Errorf("forge API request failed: HTTP %d", resp.StatusCode)
		} else {
			baseErr = &APIError{Status: resp.StatusCode, Transient: resp.StatusCode >= 500, Detail: detail}
		}
		if resp.StatusCode == http.StatusForbidden && src != "" {
			return fmt.Errorf("%w (%s may lack repository write scope; run harnez forge status)", baseErr, src)
		}
		return baseErr
	}
	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("forge write: invalid JSON response")
		}
	}
	return nil
}

// CreatePull creates a new pull request.
func (c PRClient) CreatePull(fg Forge, owner, repo string, in CreatePullInput) (PullRequest, error) {
	if strings.TrimSpace(in.Title) == "" {
		return PullRequest{}, fmt.Errorf("pull request title must not be empty")
	}
	if strings.TrimSpace(in.Head) == "" || strings.TrimSpace(in.Base) == "" {
		return PullRequest{}, fmt.Errorf("pull request head and base branches are required")
	}
	var raw rawPull
	err := c.writeRequest(fg, http.MethodPost, c.repoBase(fg, owner, repo)+"/pulls", in, &raw)
	if err != nil {
		return PullRequest{}, err
	}
	return raw.normalize(), nil
}

// EditPull updates fields on an existing pull request.
func (c PRClient) EditPull(fg Forge, owner, repo string, number int, in EditPullInput) (PullRequest, error) {
	endpoint := fmt.Sprintf("%s/pulls/%d", c.repoBase(fg, owner, repo), number)
	var raw rawPull
	if err := c.writeRequest(fg, http.MethodPatch, endpoint, in, &raw); err != nil {
		return PullRequest{}, err
	}
	return raw.normalize(), nil
}

// CommentPull adds a comment to a pull request and returns its HTML URL.
func (c PRClient) CommentPull(fg Forge, owner, repo string, number int, body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("comment body must not be empty")
	}
	endpoint := fmt.Sprintf("%s/issues/%d/comments", c.repoBase(fg, owner, repo), number)
	var res struct {
		HTMLURL string `json:"html_url"`
	}
	err := c.writeRequest(fg, http.MethodPost, endpoint, struct {
		Body string `json:"body"`
	}{Body: body}, &res)
	return res.HTMLURL, err
}

// MergePull merges a pull request using the requested method ("merge", "squash", "rebase").
func (c PRClient) MergePull(fg Forge, owner, repo string, number int, in MergePullInput) error {
	method := strings.TrimSpace(in.Method)
	if method == "" {
		method = "merge"
	}
	switch method {
	case "merge", "squash", "rebase":
	default:
		return fmt.Errorf("unsupported merge method %q (want merge, squash, or rebase)", method)
	}
	endpoint := fmt.Sprintf("%s/pulls/%d/merge", c.repoBase(fg, owner, repo), number)
	if fg.Kind == "github" {
		payload := struct {
			MergeMethod string `json:"merge_method"`
		}{MergeMethod: method}
		return c.writeRequest(fg, http.MethodPut, endpoint, payload, nil)
	}
	payload := struct {
		Do                     string `json:"Do"`
		DeleteBranchAfterMerge bool   `json:"delete_branch_after_merge,omitempty"`
	}{
		Do:                     method,
		DeleteBranchAfterMerge: in.DeleteBranch,
	}
	return c.writeRequest(fg, http.MethodPost, endpoint, payload, nil)
}

// FormatPullList formats pull requests as a compact table.
func FormatPullList(pulls []PullRequest) string {
	if len(pulls) == 0 {
		return "no pull requests\n"
	}
	var b strings.Builder
	for _, p := range pulls {
		state := p.State
		if p.Merged {
			state = "merged"
		} else if p.Draft {
			state = "draft"
		}
		branch := ""
		if p.HeadRef != "" && p.BaseRef != "" {
			branch = fmt.Sprintf(" [%s -> %s]", p.HeadRef, p.BaseRef)
		}
		fmt.Fprintf(&b, "#%-5d %-7s %-16s %s%s\n", p.Number, state, p.Author, p.Title, branch)
	}
	return b.String()
}

// FormatPullDetail formats one pull request with optional comments and files.
func FormatPullDetail(pr PullRequest, comments []PRComment, files []PRFile) string {
	var b strings.Builder
	state := pr.State
	if pr.Merged {
		state = "merged"
	} else if pr.Draft {
		state = "draft"
	}
	fmt.Fprintf(&b, "#%d %s\n", pr.Number, pr.Title)
	fmt.Fprintf(&b, "state:  %s\n", state)
	if pr.Author != "" {
		fmt.Fprintf(&b, "author: %s\n", pr.Author)
	}
	if pr.HeadRef != "" || pr.BaseRef != "" {
		fmt.Fprintf(&b, "branch: %s -> %s\n", pr.HeadRef, pr.BaseRef)
	}
	if pr.Mergeable != nil {
		fmt.Fprintf(&b, "mergeable: %v\n", *pr.Mergeable)
	}
	if pr.HTMLURL != "" {
		fmt.Fprintf(&b, "url:    %s\n", pr.HTMLURL)
	}
	if strings.TrimSpace(pr.Body) != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(pr.Body, "\n"))
	}
	if len(files) > 0 {
		fmt.Fprintf(&b, "\nfiles (%d):\n", len(files))
		for _, f := range files {
			fmt.Fprintf(&b, "  %-8s %s (+%d -%d)\n", f.Status, f.Filename, f.Additions, f.Deletions)
		}
	}
	if len(comments) > 0 {
		fmt.Fprintf(&b, "\ncomments (%d):\n", len(comments))
		for _, c := range comments {
			fmt.Fprintf(&b, "  @%s (%s):\n", c.Author, c.CreatedAt)
			for _, line := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
	}
	return b.String()
}
