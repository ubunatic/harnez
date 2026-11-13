package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// julesWaitPrompt keeps tracking a session without sending anything to it.
	julesWaitPrompt = "wait"
	// julesApprovePrompt approves the plan of a session waiting for approval.
	julesApprovePrompt = "approve"
	// julesBranchEnv selects the starting branch; unset uses the GitHub default.
	julesBranchEnv = "HARNEZ_JULES_BRANCH"
	// julesMaxResponseBytes bounds one API response; activities carry patches.
	julesMaxResponseBytes = 32 << 20
	// julesMaxActivityPages bounds the activity listing of one report.
	julesMaxActivityPages = 20
	julesTitleMaxRunes    = 80
)

// JulesSpec is the jules section of spec/agent.yaml.
type JulesSpec struct {
	BaseURL          string `yaml:"base_url"`
	ConfigPath       string `yaml:"config_path"`
	PollIntervalMS   int    `yaml:"poll_interval_ms"`
	WaitTimeoutMS    int    `yaml:"wait_timeout_ms"`
	RequestTimeoutMS int    `yaml:"request_timeout_ms"`
	MaxPollErrors    int    `yaml:"max_poll_errors"`
	CompletedPRPolls int    `yaml:"completed_pr_polls"`
	// SyncRemote is the canonical remote (Codeberg) whose SyncBranch the GitHub
	// copy is brought level with before a session starts (issue 1074).
	SyncRemote    string `yaml:"sync_remote"`
	SyncBranch    string `yaml:"sync_branch"`
	SyncTimeoutMS int    `yaml:"sync_timeout_ms"`
	// AutoReply, when set, is sent once per session the first time Jules waits
	// for feedback, so a run that asks back still finishes (issue 1113).
	AutoReply string `yaml:"auto_reply"`
}

func (s JulesSpec) validate() error {
	if strings.TrimSpace(s.BaseURL) == "" || strings.TrimSpace(s.ConfigPath) == "" {
		return errors.New("agent spec: jules.base_url and jules.config_path are required")
	}
	if s.PollIntervalMS <= 0 || s.WaitTimeoutMS <= 0 || s.RequestTimeoutMS <= 0 || s.MaxPollErrors <= 0 || s.CompletedPRPolls < 0 {
		return errors.New("agent spec: positive jules poll, wait and request bounds required")
	}
	if strings.TrimSpace(s.SyncRemote) == "" || strings.TrimSpace(s.SyncBranch) == "" || s.SyncTimeoutMS <= 0 {
		return errors.New("agent spec: jules.sync_remote, jules.sync_branch and a positive jules.sync_timeout_ms are required")
	}
	return nil
}

// JulesDriver runs Google Jules through its REST API. Jules works in the cloud
// on a connected GitHub repository, so a turn submits or tracks a remote session
// and waits in a bounded way; the session keeps running after the wait ends.
// It never merges anything and never pushes: the owner integrates the PR.
type JulesDriver struct {
	Dir string
	// Spec, Client, Sleep, Git and Getenv are test seams; zero values use
	// spec/agent.yaml and the real clock, network, git and environment.
	Spec   *JulesSpec
	Client *http.Client
	Sleep  func(context.Context, time.Duration) error
	Git    func(context.Context, string, ...string) ([]byte, error)
	Getenv func(string) string
}

// julesConfig is the owner's ~/.jules/config.json. Key is a secret: never log
// it, and never put it in an error.
type julesConfig struct {
	Key      string `json:"key"`
	Projects []struct {
		Repo string `json:"repo"`
	} `json:"github_projects"`
}

type julesSession struct {
	Name       string        `json:"name"`
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	State      string        `json:"state"`
	URL        string        `json:"url"`
	UpdateTime string        `json:"updateTime"`
	Outputs    []julesOutput `json:"outputs"`
}

type julesOutput struct {
	PullRequest *julesPullRequest `json:"pullRequest"`
}

type julesPullRequest struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	HeadRef string `json:"headRef"`
	BaseRef string `json:"baseRef"`
}

type julesActivity struct {
	CreateTime string `json:"createTime"`
	// Originator is "agent" or "user"; only agent activities are reactions.
	Originator    string `json:"originator"`
	AgentMessaged *struct {
		AgentMessage string `json:"agentMessage"`
	} `json:"agentMessaged"`
	UserMessaged *struct {
		UserMessage string `json:"userMessage"`
	} `json:"userMessaged"`
	SessionFailed *struct {
		Reason string `json:"reason"`
	} `json:"sessionFailed"`
	PlanGenerated *struct {
		Plan struct {
			Steps []struct {
				Title string `json:"title"`
			} `json:"steps"`
		} `json:"plan"`
	} `json:"planGenerated"`
}

// julesAPIError is a non-2xx API answer. Message is the API's own text.
type julesAPIError struct {
	Status  int
	Message string
}

func (e *julesAPIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("Jules API returned HTTP %d", e.Status)
	}
	msg := fmt.Sprintf("Jules API returned HTTP %d: %s", e.Status, e.Message)
	if e.Status == http.StatusBadRequest && strings.Contains(e.Message, "Precondition check failed") {
		msg += " (seen when several sessions start within a minute or the repository is not connected at jules.google.com; wait a few minutes and retry with a new --name)"
	}
	return msg
}

func (e *julesAPIError) transient() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

type julesClient struct {
	http    *http.Client
	baseURL string
	key     string
}

func (c julesClient) call(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+"/"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Goog-Api-Key", c.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, julesMaxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &failure)
		// Errors end up in stored session errors and logs, so an echoed key must not.
		return &julesAPIError{Status: resp.StatusCode, Message: strings.ReplaceAll(failure.Error.Message, c.key, "[redacted]")}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Jules API response: %w", err)
	}
	return nil
}

func (d JulesDriver) spec() (JulesSpec, error) {
	if d.Spec != nil {
		return *d.Spec, nil
	}
	spec, err := agentSpecOnce()
	if err != nil {
		return JulesSpec{}, err
	}
	return spec.Jules, nil
}

func (d JulesDriver) getenv(name string) string {
	if d.Getenv != nil {
		return d.Getenv(name)
	}
	return os.Getenv(name)
}

func (d JulesDriver) sleep(ctx context.Context, wait time.Duration) error {
	if d.Sleep != nil {
		return d.Sleep(ctx, wait)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (d JulesDriver) git(ctx context.Context, args ...string) ([]byte, error) {
	if d.Git != nil {
		return d.Git(ctx, d.Dir, args...)
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", d.Dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	// A hook or helper can keep the output pipe open after git itself is killed
	// at the deadline; WaitDelay stops Output from waiting for it (issue 1074).
	cmd.WaitDelay = julesGitWaitDelay
	return cmd.Output()
}

// julesGitWaitDelay bounds how long a git command may run on after its context
// ends. It is a safety margin, not a user setting, so it is not in the spec.
const julesGitWaitDelay = 3 * time.Second

// loadJulesConfig reads the key file and refuses one other users can read.
func loadJulesConfig(path string) (julesConfig, error) {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return julesConfig{}, fmt.Errorf("resolve Jules config path: %w", err)
		}
		path = filepath.Join(home, rest)
	}
	info, err := os.Stat(path)
	if err != nil {
		return julesConfig{}, fmt.Errorf("Jules config: %w; create it with an API key from https://jules.google.com/settings#api", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return julesConfig{}, fmt.Errorf("Jules config %s is readable by other users; run: chmod 600 %s", path, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return julesConfig{}, fmt.Errorf("Jules config: %w", err)
	}
	var config julesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return julesConfig{}, julesConfigError(path, err)
	}
	if strings.TrimSpace(config.Key) == "" {
		return julesConfig{}, fmt.Errorf("Jules config %s has no key", path)
	}
	return config, nil
}

// julesConfigError names what is wrong with a config that does not decode. A
// syntax error gives its byte offset; a type mismatch names the expected shape.
// The decoder's text is never echoed, so the key cannot leak through it.
func julesConfigError(path string, err error) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Errorf("Jules config %s is not valid JSON (byte %d)", path, syntax.Offset)
	}
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) {
		return fmt.Errorf("Jules config %s is not valid JSON", path)
	}
	switch {
	case mismatch.Field == "github_projects" && mismatch.Type.Kind() == reflect.Slice:
		return fmt.Errorf(`Jules config %s: github_projects must be a list of {"repo": "<name>"}`, path)
	case strings.HasPrefix(mismatch.Field, "github_projects"):
		return fmt.Errorf(`Jules config %s: github_projects entries must be {"repo": "<name>"}`, path)
	case mismatch.Field == "key":
		return fmt.Errorf("Jules config %s: key must be a string", path)
	}
	return fmt.Errorf(`Jules config %s must be a JSON object with "key" and "github_projects"`, path)
}

func (d JulesDriver) client() (julesClient, julesConfig, JulesSpec, error) {
	spec, err := d.spec()
	if err != nil {
		return julesClient{}, julesConfig{}, JulesSpec{}, err
	}
	config, err := loadJulesConfig(spec.ConfigPath)
	if err != nil {
		return julesClient{}, julesConfig{}, JulesSpec{}, err
	}
	httpClient := d.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(spec.RequestTimeoutMS) * time.Millisecond}
	}
	return julesClient{http: httpClient, baseURL: spec.BaseURL, key: config.Key}, config, spec, nil
}

var julesGitHubRemote = regexp.MustCompile(`^(?:https://(?:[^@/]+@)?github\.com/|ssh://git@github\.com/|git@github\.com:)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// parseGitHubRemote returns the owner and repository of a github.com remote URL.
func parseGitHubRemote(remoteURL string) (owner, repo string, ok bool) {
	match := julesGitHubRemote.FindStringSubmatch(strings.TrimSpace(remoteURL))
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

// githubRemote finds the GitHub copy of the repository: the remote named github
// when it points at github.com, else the first remote that does. A fan-out
// remote (several push URLs, such as a local both) is used only when no
// single-URL remote points at GitHub (issue 1074).
func (d JulesDriver) githubRemote(ctx context.Context) (name, owner, repo string, err error) {
	out, err := d.git(ctx, "remote")
	if err != nil {
		return "", "", "", fmt.Errorf("list git remotes in %s: %w", d.Dir, err)
	}
	remotes := strings.Fields(string(out))
	sort.SliceStable(remotes, func(i, j int) bool { return remotes[i] == "github" && remotes[j] != "github" })
	var fanOut string
	var fanOutOwner, fanOutRepo string
	for _, remote := range remotes {
		remoteURL, urlErr := d.git(ctx, "remote", "get-url", remote)
		if urlErr != nil {
			continue
		}
		if owner, repo, ok := parseGitHubRemote(string(remoteURL)); ok {
			if d.fansOut(ctx, remote) {
				if fanOut == "" {
					fanOut, fanOutOwner, fanOutRepo = remote, owner, repo
				}
				continue
			}
			return remote, owner, repo, nil
		}
	}
	if fanOut != "" {
		return fanOut, fanOutOwner, fanOutRepo, nil
	}
	return "", "", "", fmt.Errorf("no github.com remote in %s; Jules only works on connected GitHub repositories", d.Dir)
}

// fansOut reports whether a remote pushes to several URLs at once.
func (d JulesDriver) fansOut(ctx context.Context, remote string) bool {
	out, err := d.git(ctx, "remote", "get-url", "--push", "--all", remote)
	return err == nil && len(strings.Fields(string(out))) > 1
}

func checkJulesProject(config julesConfig, repo string) error {
	if len(config.Projects) == 0 {
		return nil
	}
	names := make([]string, 0, len(config.Projects))
	for _, project := range config.Projects {
		if project.Repo == repo {
			return nil
		}
		names = append(names, project.Repo)
	}
	return fmt.Errorf("repository %q is not in github_projects of the Jules config (allowed: %s)", repo, strings.Join(names, ", "))
}

// branchDriftWarning compares the starting branch on GitHub with the same branch
// on the other remotes, asking the remotes themselves rather than stale local refs.
func (d JulesDriver) branchDriftWarning(ctx context.Context, github, branch string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ref := "refs/heads/" + branch
	head := func(remote string) (string, error) { return d.remoteHead(ctx, remote, ref) }
	githubHead, err := head(github)
	if err != nil {
		return fmt.Sprintf("could not check whether %s/%s is current: %v", github, branch, err)
	}
	out, err := d.git(ctx, "remote")
	if err != nil {
		return fmt.Sprintf("could not check whether %s/%s is current: %v", github, branch, err)
	}
	for _, remote := range strings.Fields(string(out)) {
		if remote == github {
			continue
		}
		other, err := head(remote)
		if err != nil {
			return fmt.Sprintf("could not compare %s/%s with %s: %v", github, branch, remote, err)
		}
		if other != "" && other != githubHead {
			return fmt.Sprintf("%s/%s differs from %s/%s, and Jules starts from GitHub; its work may be based on old code", github, branch, remote, branch)
		}
	}
	return ""
}

// remoteHead returns the commit a remote holds for ref, or "" when it has none.
func (d JulesDriver) remoteHead(ctx context.Context, remote, ref string) (string, error) {
	out, err := d.git(ctx, "ls-remote", remote, ref)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

// syncGitHubBranch brings the GitHub copy of spec.SyncBranch level with the
// canonical remote before Jules starts from it (issue 1074). It only fast-forwards:
// the one push is the canonical commit to the GitHub branch, with no force, prune,
// mirror or other refspec, and only when GitHub is strictly behind. Every failure
// or divergence comes back as a warning that names the commit Jules will read,
// and none of them stops the session.
func (d JulesDriver) syncGitHubBranch(ctx context.Context, github string, spec JulesSpec) []string {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(spec.SyncTimeoutMS)*time.Millisecond)
	defer cancel()
	branch := spec.SyncBranch
	ref := "refs/heads/" + branch
	githubMain := github + "/" + branch
	githubHead, err := d.remoteHead(ctx, github, ref)
	if err != nil {
		return []string{fmt.Sprintf("could not read %s: %s; Jules will read %s", githubMain, gitFailure(err), githubMain)}
	}
	canonicalHead, err := d.remoteHead(ctx, spec.SyncRemote, ref)
	if err != nil {
		return []string{fmt.Sprintf("could not read %s/%s: %s, so %s was not synced; Jules will read %s", spec.SyncRemote, branch, gitFailure(err), githubMain, julesReads(githubMain, githubHead))}
	}
	if canonicalHead == "" {
		return []string{fmt.Sprintf("%s/%s has no commit to sync from; Jules will read %s", spec.SyncRemote, branch, julesReads(githubMain, githubHead))}
	}
	if canonicalHead == githubHead {
		return nil
	}
	if githubHead == "" {
		return []string{fmt.Sprintf("%s has no commit while %s/%s is at %s; nothing was pushed; Jules will read %s", githubMain, spec.SyncRemote, branch, canonicalHead, julesReads(githubMain, githubHead))}
	}
	for _, source := range []struct{ remote, sha string }{{spec.SyncRemote, canonicalHead}, {github, githubHead}} {
		if _, err := d.git(ctx, "cat-file", "-e", source.sha+"^{commit}"); err == nil {
			continue
		}
		if _, err := d.git(ctx, "fetch", "--no-tags", "--quiet", source.remote, ref); err != nil {
			return []string{fmt.Sprintf("could not fetch %s/%s: %s, so %s was not synced; Jules will read %s", source.remote, branch, gitFailure(err), githubMain, julesReads(githubMain, githubHead))}
		}
	}
	base, err := d.git(ctx, "merge-base", githubHead, canonicalHead)
	if err != nil || strings.TrimSpace(string(base)) != githubHead {
		how := "the two have diverged; fetch both, integrate the GitHub-only commits through a normal PR, then retry"
		if _, berr := d.git(ctx, "merge-base", "--is-ancestor", canonicalHead, githubHead); berr == nil {
			how = "GitHub is ahead with commits " + spec.SyncRemote + " lacks; bring them to " + spec.SyncRemote + " through a normal PR"
		}
		return []string{fmt.Sprintf("%s is at %s and %s/%s at %s: %s. Nothing was pushed; Jules will read %s", githubMain, githubHead, spec.SyncRemote, branch, canonicalHead, how, julesReads(githubMain, githubHead))}
	}
	if _, err := d.git(ctx, "push", "--no-force", "--no-follow-tags", "--no-recurse-submodules", github, canonicalHead+":"+ref); err != nil {
		failure := gitFailure(err)
		hint := ""
		if strings.Contains(failure, "denied") || strings.Contains(failure, "403") {
			hint = "; your git credentials may not push to GitHub, so ask the repository owner to sync it"
		}
		return []string{fmt.Sprintf("could not push %s/%s (%s) to %s: %s%s; Jules will read %s", spec.SyncRemote, branch, canonicalHead, githubMain, failure, hint, julesReads(githubMain, githubHead))}
	}
	return nil
}

// julesReads names the commit a branch holds, which is the one Jules reads.
func julesReads(branch, sha string) string {
	if sha == "" {
		return branch + ", which has no commit"
	}
	return branch + " at " + sha
}

// gitFailure returns the stderr of a failed git command, joined on one line,
// or the error itself when git printed nothing.
func gitFailure(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if text := strings.TrimSpace(string(exitErr.Stderr)); text != "" {
			return strings.ReplaceAll(text, "\n", "; ")
		}
	}
	return err.Error()
}

func julesTitle(prompt string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(prompt), "\n")
	line = strings.TrimSpace(strings.TrimLeft(line, "# "))
	if runes := []rune(line); len(runes) > julesTitleMaxRunes {
		return string(runes[:julesTitleMaxRunes])
	}
	return line
}

// Run submits the prompt as a new Jules session and waits for it in a bounded way.
func (d JulesDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	if err := ValidateContextMode("jules", o.ContextMode); err != nil {
		return nil, err
	}
	started := time.Now()
	prompt := strings.TrimSpace(o.Prompt)
	if prompt == "" {
		return nil, errors.New("Jules needs a prompt: pass the issue text, for example -f issues/<n>-<slug>.md")
	}
	// A resume without a provider session falls back to Run; a command must not
	// spend a task on a session whose whole instruction is "wait".
	if d.IsControlPrompt(prompt) {
		return nil, fmt.Errorf("no Jules session to apply %q to; start one with the issue text first", prompt)
	}
	client, config, spec, err := d.client()
	if err != nil {
		return nil, err
	}
	remote, owner, repo, err := d.githubRemote(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkJulesProject(config, repo); err != nil {
		return nil, err
	}
	sourceName := "sources/github/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	var source struct {
		GitHubRepo struct {
			DefaultBranch struct {
				DisplayName string `json:"displayName"`
			} `json:"defaultBranch"`
		} `json:"githubRepo"`
	}
	if err := client.call(ctx, http.MethodGet, sourceName, nil, &source); err != nil {
		var apiErr *julesAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, fmt.Errorf("%s/%s is not connected to Jules; connect it at https://jules.google.com", owner, repo)
		}
		return nil, fmt.Errorf("look up Jules source: %w", err)
	}
	branch := strings.TrimSpace(d.getenv(julesBranchEnv))
	if branch == "" {
		branch = source.GitHubRepo.DefaultBranch.DisplayName
	}
	if branch == "" {
		return nil, fmt.Errorf("Jules source %s/%s has no default branch; set %s", owner, repo, julesBranchEnv)
	}
	var warnings []string
	// Jules starts from GitHub, so GitHub's copy of the sync branch is brought
	// level with the canonical remote first. Warnings never stop the session.
	if branch == spec.SyncBranch {
		warnings = append(warnings, d.syncGitHubBranch(ctx, remote, spec)...)
	}
	if warning := d.branchDriftWarning(ctx, remote, branch, time.Duration(spec.RequestTimeoutMS)*time.Millisecond); warning != "" {
		warnings = append(warnings, warning)
	}
	request := map[string]any{
		"prompt":         prompt,
		"title":          julesTitle(prompt),
		"automationMode": "AUTO_CREATE_PR",
		"sourceContext": map[string]any{
			"source":            sourceName,
			"githubRepoContext": map[string]any{"startingBranch": branch},
		},
	}
	// Never retried: a repeated create would spend a second task from the quota.
	var session julesSession
	if err := client.call(ctx, http.MethodPost, "sessions", request, &session); err != nil {
		return nil, fmt.Errorf("create Jules session: %w", err)
	}
	if session.ID == "" {
		return nil, errors.New("create Jules session: the response has no session id")
	}
	observeProviderSession(ctx, session.ID)
	return d.track(ctx, client, spec, session.ID, julesReply{}, warnings, started)
}

// IsControlPrompt reports the resume commands wait and approve, and the empty
// prompt, which also means wait.
func (d JulesDriver) IsControlPrompt(prompt string) bool {
	command := strings.TrimSpace(prompt)
	return command == "" || strings.EqualFold(command, julesWaitPrompt) || strings.EqualFold(command, julesApprovePrompt)
}

// Resume keeps tracking a session on "wait", approves its plan on "approve", and
// sends any other prompt to it as a message.
func (d JulesDriver) Resume(ctx context.Context, id, prompt string, _ Model) (*TurnResult, error) {
	started := time.Now()
	client, _, spec, err := d.client()
	if err != nil {
		return nil, err
	}
	name := "sessions/" + url.PathEscape(id)
	command := strings.TrimSpace(prompt)
	if command == "" || strings.EqualFold(command, julesWaitPrompt) {
		return d.track(ctx, client, spec, id, julesReply{}, nil, started)
	}
	var session julesSession
	if err := client.call(ctx, http.MethodGet, name, nil, &session); err != nil {
		return nil, fmt.Errorf("read Jules session %s: %w", id, err)
	}
	approve := strings.EqualFold(command, julesApprovePrompt)
	if approve && session.State != "AWAITING_PLAN_APPROVAL" {
		return nil, fmt.Errorf("Jules session %s is %s, not waiting for plan approval", id, session.State)
	}
	// The baseline is read before the send, so the owner's own message cannot be
	// mistaken for a reply. Without it nothing is sent.
	before, err := d.activities(ctx, client, name)
	if err != nil {
		return nil, fmt.Errorf("read Jules activities of session %s before sending: %w", id, err)
	}
	reply := julesReply{sent: true, pre: session.State, after: julesNewest(before)}
	if approve {
		if err := client.call(ctx, http.MethodPost, name+":approvePlan", map[string]any{}, nil); err != nil {
			return nil, fmt.Errorf("approve Jules plan: %w", err)
		}
	} else if err := client.call(ctx, http.MethodPost, name+":sendMessage", map[string]any{"prompt": command}, nil); err != nil {
		return nil, fmt.Errorf("send message to Jules session %s: %w", id, err)
	}
	return d.track(ctx, client, spec, id, reply, nil, started)
}

// julesReply is what a call sent to a session, and so what counts as Jules
// answering it.
type julesReply struct {
	sent  bool      // this call sent a message or a plan approval
	pre   string    // the session state read just before the send
	after time.Time // the newest activity before the send; zero when there was none
}

// gates reports whether a state must be checked for a reply first. After a send
// every state is; a bare wait only holds back the states that ask the owner.
func (r julesReply) gates(state string) bool {
	return r.sent || julesWaitingState(state)
}

// baseline is the time a reaction must come after: the newest activity before the
// send, or for a bare wait the owner's newest message (zero when there is none).
func (r julesReply) baseline(acts []julesActivity) time.Time {
	if r.sent {
		return r.after
	}
	var at time.Time
	for _, a := range acts {
		if a.Originator == "user" && julesCreated(a).After(at) {
			at = julesCreated(a)
		}
	}
	return at
}

// reacted reports whether an agent activity came after the baseline.
func (r julesReply) reacted(acts []julesActivity) bool {
	base := r.baseline(acts)
	if !r.sent && base.IsZero() {
		// A bare wait with no owner message has no older question to skip: the state is the answer.
		return true
	}
	for _, a := range acts {
		if a.Originator == "agent" && julesAfter(a, base) {
			return true
		}
	}
	return false
}

// answered reports whether Jules has answered the owner's last call: the state
// moved from the one before the send, or an agent activity came after the baseline.
// It also returns any activities fetched during the check.
func (d JulesDriver) answered(ctx context.Context, client julesClient, name string, reply julesReply, state string) (bool, []julesActivity) {
	if reply.sent && state != reply.pre {
		return true, nil
	}
	if !reply.gates(state) {
		return false, nil
	}
	acts, err := d.activities(ctx, client, name)
	if err != nil {
		return false, nil
	}
	return reply.reacted(acts), acts
}

// julesCreated parses an activity's createTime. Jules writes microseconds or
// nanoseconds, so the strings are never compared; an unparsable time is zero.
func julesCreated(a julesActivity) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, a.CreateTime)
	return at
}

// julesAfter reports whether an activity was created after base; a zero base admits every activity.
func julesAfter(a julesActivity, base time.Time) bool {
	return base.IsZero() || julesCreated(a).After(base)
}

// julesNewest is the latest createTime among the activities, or zero.
func julesNewest(acts []julesActivity) time.Time {
	var at time.Time
	for _, a := range acts {
		if julesCreated(a).After(at) {
			at = julesCreated(a)
		}
	}
	return at
}

// julesWaitingState reports the states in which Jules asks the owner something.
func julesWaitingState(state string) bool {
	return state == "AWAITING_PLAN_APPROVAL" || state == "AWAITING_USER_FEEDBACK" || state == "PAUSED"
}

// julesStalledReplies returns the number of user replies since Jules's last message
// when the session is waiting for feedback and at least two replies received no reaction.
// It returns 0 when the session is not stalled.
func julesStalledReplies(state string, acts []julesActivity) int {
	if !julesWaitingState(state) {
		return 0
	}
	var lastAgent time.Time
	hasAgent := false
	for _, a := range acts {
		if a.Originator == "agent" {
			t := julesCreated(a)
			if !hasAgent || t.After(lastAgent) {
				lastAgent = t
				hasAgent = true
			}
		}
	}
	replies := 0
	for _, a := range acts {
		if a.Originator == "user" {
			t := julesCreated(a)
			if !hasAgent || t.After(lastAgent) {
				replies++
			}
		}
	}
	if replies >= 2 {
		return replies
	}
	return 0
}

// track polls the session until it stops needing no one, or the wait runs out.
// Only a Jules failure or an API refusal is an error: a session still running or
// waiting for the owner is a normal result that says how to continue.
func (d JulesDriver) track(ctx context.Context, client julesClient, spec JulesSpec, id string, reply julesReply, warnings []string, started time.Time) (*TurnResult, error) {
	name := "sessions/" + url.PathEscape(id)
	interval := time.Duration(spec.PollIntervalMS) * time.Millisecond
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(spec.WaitTimeoutMS)*time.Millisecond)
	defer cancel()
	autoSent := false
	result := func(body string, warns []string) *TurnResult {
		if autoSent {
			body += "\nAuto-reply: sent the one reply from spec jules.auto_reply in this wait."
		}
		return &TurnResult{SessionID: id, Response: body, Messages: []string{body}, Warnings: warns, DurationMS: time.Since(started).Milliseconds()}
	}
	var last julesSession
	var lastActivities []julesActivity
	answered := false
	// stillRunning ends the wait while the session goes on; StillRunning
	// lets the CLI exit 3, like harnez pr wait (issue 1111).
	stillRunning := func(warns []string) *TurnResult {
		acts := lastActivities
		if len(acts) == 0 && julesWaitingState(last.State) && ctx.Err() == nil {
			if fetched, err := d.activities(ctx, client, name); err == nil {
				acts = fetched
			}
		}
		stalled := julesStalledReplies(last.State, acts)
		r := result(julesStillRunning(id, last, spec, reply.sent && !answered, stalled), warns)
		r.StillRunning = true
		return r
	}
	failures, prPolls := 0, 0
	for {
		var session julesSession
		err := client.call(waitCtx, http.MethodGet, name, nil, &session)
		var apiErr *julesAPIError
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case waitCtx.Err() != nil:
			return stillRunning(warnings), nil
		case err == nil:
			failures = 0
			last = session
		case errors.As(err, &apiErr) && !apiErr.transient():
			return nil, fmt.Errorf("read Jules session %s: %w", id, err)
		default:
			failures++
			if failures >= spec.MaxPollErrors {
				warnings = append(warnings, fmt.Sprintf("stopped polling after %d failed requests: %v", failures, err))
				return stillRunning(warnings), nil
			}
		}
		if err == nil {
			var acts []julesActivity
			answered, acts = d.answered(waitCtx, client, name, reply, session.State)
			if acts != nil {
				lastActivities = acts
			}
		}
		if err == nil && (answered || !reply.gates(session.State)) {
			switch session.State {
			case "COMPLETED":
				if session.pullRequest() != nil || prPolls >= spec.CompletedPRPolls {
					last, listErr := d.lastAgentMessage(waitCtx, client, name)
					if listErr != nil {
						warnings = append(warnings, "could not read Jules activities: "+listErr.Error())
					}
					return result(julesCompleted(id, session, last), warnings), nil
				}
				prPolls++
			case "FAILED":
				reason, listErr := d.failureReason(waitCtx, client, name)
				if listErr != nil {
					reason = "reason unavailable: " + listErr.Error()
				}
				return nil, fmt.Errorf("Jules session %s failed: %s (%s)", id, reason, session.URL)
			case "AWAITING_PLAN_APPROVAL", "AWAITING_USER_FEEDBACK", "PAUSED":
				if sent, autoErr := d.autoReply(waitCtx, client, spec, name, session.State); autoErr != nil {
					warnings = append(warnings, "auto-reply failed (it may still have reached Jules; a later wait will not resend it): "+autoErr.Error())
				} else if sent.sent {
					warnings = append(warnings, "Jules asked back; sent the one auto-reply from spec jules.auto_reply")
					reply, answered, autoSent = sent, false, true
					continue
				}
				detail, listErr := d.waitingDetail(waitCtx, client, name, session.State, reply)
				if listErr != nil {
					warnings = append(warnings, "could not read Jules activities: "+listErr.Error())
				}
				return result(julesNeedsOwner(id, session, detail), warnings), nil
			}
		}
		if err := d.sleep(waitCtx, interval); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return stillRunning(warnings), nil
		}
	}
}

func (s julesSession) pullRequest() *julesPullRequest {
	for _, output := range s.Outputs {
		if output.PullRequest != nil && output.PullRequest.URL != "" {
			return output.PullRequest
		}
	}
	return nil
}

// julesCompleted reports a finished session with Jules's last message, which can
// hold findings, most of all when the run opened no PR.
func julesCompleted(id string, session julesSession, last string) string {
	lines := []string{fmt.Sprintf("Jules session %s: COMPLETED", id), "Session: " + session.URL}
	if last != "" {
		lines = append(lines, "Jules says: "+last)
	}
	pr := session.pullRequest()
	if pr == nil {
		lines = append(lines, "No pull request was created. Check the session page for its changes.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Pull request: "+pr.URL)
	if pr.Title != "" {
		lines = append(lines, "Title: "+pr.Title)
	}
	if pr.HeadRef != "" {
		lines = append(lines, fmt.Sprintf("Branch: %s (into %s)", pr.HeadRef, pr.BaseRef))
	}
	lines = append(lines, "Nothing was merged, and Codeberg is unchanged. Review the pull request, then pull and integrate it yourself.")
	return strings.Join(lines, "\n")
}

func julesNeedsOwner(id string, session julesSession, detail string) string {
	lines := []string{fmt.Sprintf("Jules session %s: %s", id, session.State), "Session: " + session.URL}
	if detail != "" {
		lines = append(lines, detail)
	}
	switch session.State {
	case "AWAITING_PLAN_APPROVAL":
		lines = append(lines, `Next: harnez agent resume --name <session> approve, or send changes with harnez agent resume --name <session> "<message>".`)
	case "AWAITING_USER_FEEDBACK":
		lines = append(lines, `Next: answer with harnez agent resume --name <session> "<message>".`)
	default:
		lines = append(lines, "Next: open the session page; the API does not describe this state. Track it again with harnez agent resume --name <session> wait.")
	}
	return strings.Join(lines, "\n")
}

// julesStillRunning says the wait ran out. When a session waiting for feedback has had
// replies without reaction, it reports the session stalled. Otherwise it notes whether
// a message was sent and offers another wait.
func julesStillRunning(id string, session julesSession, spec JulesSpec, sentNoReply bool, stalledReplies int) string {
	state := session.State
	if state == "" {
		state = "state unknown"
	}
	if stalledReplies > 0 {
		lines := []string{fmt.Sprintf("Jules session %s: %s; stalled: %d replies since Jules's last message, no reaction", id, state, stalledReplies)}
		if session.URL != "" {
			lines = append(lines, "Session: "+session.URL)
		}
		lines = append(lines, "Next: stop or restart smaller instead of another wait.")
		return strings.Join(lines, "\n")
	}
	status := "stopped waiting, and the session keeps running on Jules."
	if sentNoReply {
		status = "message sent, no reply yet; stopped waiting, and the session keeps running on Jules."
	}
	lines := []string{fmt.Sprintf("Jules session %s: %s; %s", id, state, status)}
	if session.URL != "" {
		lines = append(lines, "Session: "+session.URL)
	}
	lines = append(lines, fmt.Sprintf("Next: track it again, up to %s more, with harnez agent resume --name <session> wait.", time.Duration(spec.WaitTimeoutMS)*time.Millisecond))
	return strings.Join(lines, "\n")
}

// activities lists a session's activities oldest first, in a bounded number of pages.
func (d JulesDriver) activities(ctx context.Context, client julesClient, name string) ([]julesActivity, error) {
	var all []julesActivity
	token := ""
	for page := 0; page < julesMaxActivityPages; page++ {
		path := name + "/activities?pageSize=100"
		if token != "" {
			path += "&pageToken=" + url.QueryEscape(token)
		}
		var response struct {
			Activities    []julesActivity `json:"activities"`
			NextPageToken string          `json:"nextPageToken"`
		}
		if err := client.call(ctx, http.MethodGet, path, nil, &response); err != nil {
			return nil, err
		}
		all = append(all, response.Activities...)
		token = response.NextPageToken
		if token == "" {
			break
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return julesCreated(all[i]).Before(julesCreated(all[j])) })
	return all, nil
}

// autoReply sends spec.AutoReply when Jules waits for feedback and no earlier
// message in the session carried it, so it goes out at most once per session.
// The real API echoes a sent message as userMessaged.userMessage (checked live
// on 2026-11-03 against session 5832791262383075829).
func (d JulesDriver) autoReply(ctx context.Context, client julesClient, spec JulesSpec, name, state string) (julesReply, error) {
	text := strings.TrimSpace(spec.AutoReply)
	if text == "" || state != "AWAITING_USER_FEEDBACK" {
		return julesReply{}, nil
	}
	acts, err := d.activities(ctx, client, name)
	if err != nil {
		return julesReply{}, err
	}
	for _, a := range acts {
		if a.UserMessaged != nil && strings.TrimSpace(a.UserMessaged.UserMessage) == text {
			return julesReply{}, nil
		}
	}
	if err := client.call(ctx, http.MethodPost, name+":sendMessage", map[string]any{"prompt": text}, nil); err != nil {
		return julesReply{}, err
	}
	return julesReply{sent: true, pre: state, after: julesNewest(acts)}, nil
}

// lastAgentMessage is the newest message Jules wrote in the session, or "".
func (d JulesDriver) lastAgentMessage(ctx context.Context, client julesClient, name string) (string, error) {
	activities, err := d.activities(ctx, client, name)
	if err != nil {
		return "", err
	}
	for i := len(activities) - 1; i >= 0; i-- {
		if m := activities[i].AgentMessaged; m != nil && m.AgentMessage != "" {
			return m.AgentMessage, nil
		}
	}
	return "", nil
}

func (d JulesDriver) failureReason(ctx context.Context, client julesClient, name string) (string, error) {
	activities, err := d.activities(ctx, client, name)
	if err != nil {
		return "", err
	}
	for i := len(activities) - 1; i >= 0; i-- {
		if failed := activities[i].SessionFailed; failed != nil && failed.Reason != "" {
			return failed.Reason, nil
		}
	}
	return "no reason reported", nil
}

// waitingDetail is the latest plan or agent message the owner must respond to,
// created after the reply baseline, so an older question is never shown again.
func (d JulesDriver) waitingDetail(ctx context.Context, client julesClient, name, state string, reply julesReply) (string, error) {
	activities, err := d.activities(ctx, client, name)
	if err != nil {
		return "", err
	}
	base := reply.baseline(activities)
	for i := len(activities) - 1; i >= 0; i-- {
		activity := activities[i]
		if !julesAfter(activity, base) {
			continue
		}
		if state == "AWAITING_PLAN_APPROVAL" && activity.PlanGenerated != nil {
			steps := make([]string, 0, len(activity.PlanGenerated.Plan.Steps))
			for n, step := range activity.PlanGenerated.Plan.Steps {
				steps = append(steps, fmt.Sprintf("%d. %s", n+1, step.Title))
			}
			return "Plan:\n" + strings.Join(steps, "\n"), nil
		}
		if state != "AWAITING_PLAN_APPROVAL" && activity.AgentMessaged != nil && activity.AgentMessaged.AgentMessage != "" {
			return "Jules says: " + activity.AgentMessaged.AgentMessage, nil
		}
	}
	return "", nil
}

// Compact is refused: Jules manages its own context on its side.
func (d JulesDriver) Compact(context.Context, string, Model) (*TurnResult, error) {
	return nil, errors.New("Jules sessions cannot be compacted; Jules manages its own context")
}

// Stop only ends local tracking: the API has no way to cancel a running session.
func (d JulesDriver) Stop(_ context.Context, id string) error {
	fmt.Fprintf(os.Stderr, "Jules has no cancel call, so session %s keeps running; end it on https://jules.google.com if needed.\n", id)
	return nil
}

// Delete removes only the local record and keeps the session and its history on Jules.
func (d JulesDriver) Delete(context.Context, string) error { return nil }

type providerSessionObserverKey struct{}

// WithProviderSessionObserver records a provider session id as soon as a batch
// driver has it, so a turn that dies while waiting can still be tracked.
func WithProviderSessionObserver(ctx context.Context, observer func(string)) context.Context {
	return context.WithValue(ctx, providerSessionObserverKey{}, observer)
}

func observeProviderSession(ctx context.Context, id string) {
	if observer, _ := ctx.Value(providerSessionObserverKey{}).(func(string)); observer != nil {
		observer(id)
	}
}
