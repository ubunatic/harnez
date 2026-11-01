package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const julesTestKey = "secret-test-key"

// fakeJules is a scripted Jules API: each GET of the session returns the next
// state, and the last state repeats.
type fakeJules struct {
	t          *testing.T
	mu         sync.Mutex
	states     []julesSession
	gets       int
	created    map[string]any
	creates    int
	messages   []string
	approvals  int
	activities []map[string]any
	getStatus  []int
	sourceCode int
}

func (f *fakeJules) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if got := r.Header.Get("X-Goog-Api-Key"); got != julesTestKey {
		f.t.Errorf("%s %s sent key %q", r.Method, r.URL.Path, got)
	}
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sources/github/"):
		if f.sourceCode != 0 {
			w.WriteHeader(f.sourceCode)
			fmt.Fprint(w, `{"error":{"message":"not found"}}`)
			return
		}
		fmt.Fprint(w, `{"githubRepo":{"defaultBranch":{"displayName":"main"}}}`)
	case r.Method == http.MethodPost && r.URL.Path == "/sessions":
		f.creates++
		if err := json.NewDecoder(r.Body).Decode(&f.created); err != nil {
			f.t.Errorf("decode create body: %v", err)
		}
		fmt.Fprint(w, `{"id":"123","name":"sessions/123","state":"QUEUED"}`)
	case r.Method == http.MethodPost && r.URL.Path == "/sessions/123:sendMessage":
		var body struct {
			Prompt string `json:"prompt"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.messages = append(f.messages, body.Prompt)
		fmt.Fprint(w, `{}`)
	case r.Method == http.MethodPost && r.URL.Path == "/sessions/123:approvePlan":
		f.approvals++
		fmt.Fprint(w, `{}`)
	case r.Method == http.MethodGet && r.URL.Path == "/sessions/123/activities":
		_ = json.NewEncoder(w).Encode(map[string]any{"activities": f.activities})
	case r.Method == http.MethodGet && r.URL.Path == "/sessions/123":
		index := f.gets
		f.gets++
		if index < len(f.getStatus) && f.getStatus[index] != 0 {
			w.WriteHeader(f.getStatus[index])
			fmt.Fprintf(w, `{"error":{"message":"status %d"}}`, f.getStatus[index])
			return
		}
		if index >= len(f.states) {
			index = len(f.states) - 1
		}
		_ = json.NewEncoder(w).Encode(f.states[index])
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

type julesHarness struct {
	driver JulesDriver
	fake   *fakeJules
	sleeps *int
}

func newJulesHarness(t *testing.T, fake *fakeJules, remotes map[string]string, heads map[string]string) julesHarness {
	t.Helper()
	fake.t = t
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	config := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(config, []byte(`{"key":"`+julesTestKey+`","github_projects":[{"repo":"harnez"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sleeps := 0
	driver := JulesDriver{
		Dir:    t.TempDir(),
		Spec:   &JulesSpec{BaseURL: server.URL, ConfigPath: config, PollIntervalMS: 1, WaitTimeoutMS: 60000, RequestTimeoutMS: 5000, MaxPollErrors: 3, CompletedPRPolls: 2},
		Client: server.Client(),
		Sleep:  func(context.Context, time.Duration) error { sleeps++; return nil },
		Getenv: func(string) string { return "" },
		Git: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch {
			case len(args) == 1 && args[0] == "remote":
				names := make([]string, 0, len(remotes))
				for name := range remotes {
					names = append(names, name)
				}
				return []byte(strings.Join(names, "\n")), nil
			case len(args) == 3 && args[0] == "remote" && args[1] == "get-url":
				return []byte(remotes[args[2]] + "\n"), nil
			case len(args) == 3 && args[0] == "ls-remote":
				return []byte(heads[args[1]] + "\t" + args[2] + "\n"), nil
			}
			return nil, fmt.Errorf("unexpected git %v", args)
		},
	}
	return julesHarness{driver: driver, fake: fake, sleeps: &sleeps}
}

var julesTestRemotes = map[string]string{"origin": "ssh://git@codeberg.org/o/harnez.git", "github": "git@github.com:o/harnez.git"}
var julesEqualHeads = map[string]string{"origin": "aaa", "github": "aaa"}

func julesDone(url string) julesSession {
	session := julesSession{ID: "123", State: "COMPLETED", URL: "https://jules.test/session/123"}
	if url != "" {
		session.Outputs = []julesOutput{{}, {PullRequest: &julesPullRequest{URL: url, Title: "Fix it", HeadRef: "jules/fix", BaseRef: "main"}}}
	}
	return session
}

func TestJulesRunSubmitsAndReportsPullRequest(t *testing.T) {
	fake := &fakeJules{states: []julesSession{{State: "QUEUED"}, {State: "IN_PROGRESS"}, julesDone("https://github.com/o/harnez/pull/9")}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	observed := ""
	ctx := WithProviderSessionObserver(context.Background(), func(id string) { observed = id })
	res, err := h.driver.Run(ctx, RunOptions{Prompt: "# 42 — Fix the bug\n\nDetails."})
	if err != nil {
		t.Fatal(err)
	}
	if observed != "123" || res.SessionID != "123" {
		t.Errorf("session id observed=%q result=%q", observed, res.SessionID)
	}
	for _, want := range []string{"COMPLETED", "https://github.com/o/harnez/pull/9", "jules/fix", "Nothing was merged", "https://jules.test/session/123"} {
		if !strings.Contains(res.Response, want) {
			t.Errorf("response misses %q:\n%s", want, res.Response)
		}
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v", res.Warnings)
	}
	created := fake.created
	if created["automationMode"] != "AUTO_CREATE_PR" || created["title"] != "42 — Fix the bug" || !strings.Contains(created["prompt"].(string), "Details.") {
		t.Errorf("create body = %v", created)
	}
	context := created["sourceContext"].(map[string]any)
	if context["source"] != "sources/github/o/harnez" || context["githubRepoContext"].(map[string]any)["startingBranch"] != "main" {
		t.Errorf("source context = %v", context)
	}
	if _, set := created["requirePlanApproval"]; set {
		t.Error("requirePlanApproval must stay unset by default")
	}
	if *h.sleeps != 2 {
		t.Errorf("sleeps = %d, want 2", *h.sleeps)
	}
}

func TestJulesRunSelectsBranchAndWarnsOnDrift(t *testing.T) {
	fake := &fakeJules{states: []julesSession{julesDone("https://github.com/o/harnez/pull/9")}}
	h := newJulesHarness(t, fake, julesTestRemotes, map[string]string{"origin": "new", "github": "old"})
	h.driver.Getenv = func(name string) string {
		if name == julesBranchEnv {
			return "topic"
		}
		return ""
	}
	res, err := h.driver.Run(context.Background(), RunOptions{Prompt: "task"})
	if err != nil {
		t.Fatal(err)
	}
	branch := fake.created["sourceContext"].(map[string]any)["githubRepoContext"].(map[string]any)["startingBranch"]
	if branch != "topic" {
		t.Errorf("startingBranch = %v", branch)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "github/topic differs from origin/topic") {
		t.Errorf("warnings = %v", res.Warnings)
	}
}

func TestJulesRunRefusals(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		remotes map[string]string
		source  int
		want    string
	}{
		{"empty prompt", "  ", julesTestRemotes, 0, "needs a prompt"},
		{"no github remote", "task", map[string]string{"origin": "ssh://git@codeberg.org/o/harnez.git"}, 0, "no github.com remote"},
		{"project not allowed", "task", map[string]string{"github": "https://github.com/o/other.git"}, 0, `"other" is not in github_projects`},
		{"not connected", "task", julesTestRemotes, http.StatusNotFound, "not connected to Jules"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeJules{states: []julesSession{{}}, sourceCode: tc.source}
			h := newJulesHarness(t, fake, tc.remotes, julesEqualHeads)
			_, err := h.driver.Run(context.Background(), RunOptions{Prompt: tc.prompt})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if fake.creates != 0 {
				t.Errorf("a refused run created %d sessions", fake.creates)
			}
		})
	}
}

func TestJulesConfigRefusesReadableFileAndKeepsKeySecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"key":"`+julesTestKey+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadJulesConfig(path)
	if err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), julesTestKey) {
		t.Fatal("error leaks the key")
	}
	if err := os.WriteFile(path, []byte(`{"key":"`+julesTestKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadJulesConfig(path); err == nil || strings.Contains(err.Error(), julesTestKey) {
		t.Fatalf("broken config err = %v", err)
	}
}

func TestJulesFailedSessionIsAnErrorWithReason(t *testing.T) {
	fake := &fakeJules{
		states:     []julesSession{{State: "FAILED", URL: "https://jules.test/session/123"}},
		activities: []map[string]any{{"createTime": "1", "sessionFailed": map[string]any{"reason": "tests would not run"}}},
	}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	_, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
	if err == nil || !strings.Contains(err.Error(), "tests would not run") || !strings.Contains(err.Error(), "https://jules.test/session/123") {
		t.Fatalf("err = %v", err)
	}
}

func TestJulesActionRequiredReturnsWhatToAnswer(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		activities []map[string]any
		want       []string
	}{
		{"plan", "AWAITING_PLAN_APPROVAL", []map[string]any{{"createTime": "1", "planGenerated": map[string]any{"plan": map[string]any{"steps": []map[string]any{{"title": "Edit docs"}}}}}}, []string{"1. Edit docs", "approve"}},
		{"feedback", "AWAITING_USER_FEEDBACK", []map[string]any{{"createTime": "2", "agentMessaged": map[string]any{"agentMessage": "newer"}}, {"createTime": "1", "agentMessaged": map[string]any{"agentMessage": "older"}}}, []string{"Jules says: newer", `"<message>"`}},
		{"paused", "PAUSED", nil, []string{"PAUSED", "session page"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeJules{states: []julesSession{{State: tc.state}}, activities: tc.activities}
			h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
			res, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(res.Response, want) {
					t.Errorf("response misses %q:\n%s", want, res.Response)
				}
			}
		})
	}
}

func TestJulesWaitIsBoundedAndKeepsSessionTrackable(t *testing.T) {
	fake := &fakeJules{states: []julesSession{{State: "IN_PROGRESS", URL: "https://jules.test/session/123"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	polls := 0
	h.driver.Sleep = func(context.Context, time.Duration) error {
		polls++
		if polls == 3 {
			return context.DeadlineExceeded
		}
		return nil
	}
	res, err := h.driver.Resume(context.Background(), "123", "", Model{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"IN_PROGRESS", "keeps running", "wait", "https://jules.test/session/123"} {
		if !strings.Contains(res.Response, want) {
			t.Errorf("response misses %q:\n%s", want, res.Response)
		}
	}
}

func TestJulesWaitDeadlineEndsARealWait(t *testing.T) {
	fake := &fakeJules{states: []julesSession{{State: "IN_PROGRESS"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	h.driver.Sleep = nil
	h.driver.Spec.WaitTimeoutMS = 20
	h.driver.Spec.PollIntervalMS = 1000
	res, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Response, "keeps running") {
		t.Errorf("response = %s", res.Response)
	}
}

func TestJulesCancelledWaitReportsCancellation(t *testing.T) {
	fake := &fakeJules{states: []julesSession{{State: "IN_PROGRESS"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	ctx, cancel := context.WithCancel(context.Background())
	h.driver.Sleep = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
	if _, err := h.driver.Resume(ctx, "123", "wait", Model{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestJulesPollErrors(t *testing.T) {
	t.Run("transient errors recover", func(t *testing.T) {
		fake := &fakeJules{states: []julesSession{julesDone("https://github.com/o/harnez/pull/9")}, getStatus: []int{503, 429}}
		h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
		res, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
		if err != nil || !strings.Contains(res.Response, "pull/9") {
			t.Fatalf("res = %v err = %v", res, err)
		}
	})
	t.Run("repeated transient errors stop with a warning", func(t *testing.T) {
		fake := &fakeJules{states: []julesSession{{}}, getStatus: []int{503, 503, 503, 503}}
		h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
		res, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "3 failed requests") || !strings.Contains(res.Response, "keeps running") {
			t.Fatalf("res = %+v", res)
		}
	})
	t.Run("refused key is an error without the key", func(t *testing.T) {
		fake := &fakeJules{states: []julesSession{{}}, getStatus: []int{401}}
		h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
		_, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
		if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), julesTestKey) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestJulesCompletedWithoutPullRequest(t *testing.T) {
	fake := &fakeJules{states: []julesSession{julesDone(""), julesDone(""), julesDone("https://github.com/o/harnez/pull/9")}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	res, err := h.driver.Resume(context.Background(), "123", "wait", Model{})
	if err != nil || !strings.Contains(res.Response, "pull/9") {
		t.Fatalf("a late PR was missed: res = %+v err = %v", res, err)
	}
	fake = &fakeJules{states: []julesSession{julesDone("")}}
	h = newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	res, err = h.driver.Resume(context.Background(), "123", "wait", Model{})
	if err != nil || !strings.Contains(res.Response, "No pull request") {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if fake.gets != 3 {
		t.Errorf("gets = %d, want the first read plus 2 extra polls", fake.gets)
	}
}

func TestJulesResumeSendsMessageAndIgnoresStaleState(t *testing.T) {
	waiting := julesSession{State: "AWAITING_USER_FEEDBACK", UpdateTime: "t1"}
	done := julesDone("https://github.com/o/harnez/pull/9")
	done.UpdateTime = "t2"
	fake := &fakeJules{states: []julesSession{waiting, waiting, done}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	res, err := h.driver.Resume(context.Background(), "123", "use the second option", Model{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.messages) != 1 || fake.messages[0] != "use the second option" {
		t.Errorf("messages = %v", fake.messages)
	}
	if !strings.Contains(res.Response, "pull/9") {
		t.Errorf("returned the stale state:\n%s", res.Response)
	}
}

// The field names come from a real session response (ticket 1071).
func TestJulesDecodesRealPullRequestShape(t *testing.T) {
	var session julesSession
	data := `{"state":"COMPLETED","outputs":[{"changeSet":{}},{"pullRequest":{"url":"u","title":"t","headRef":"fix","baseRef":"main"}}]}`
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		t.Fatal(err)
	}
	if pr := session.pullRequest(); pr == nil || pr.HeadRef != "fix" || pr.BaseRef != "main" {
		t.Fatalf("pull request = %#v", pr)
	}
}

func TestJulesAPIErrorRedactsEchoedKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":{"message":"API key %s is not valid"}}`, julesTestKey)
	}))
	t.Cleanup(server.Close)
	client := julesClient{http: server.Client(), baseURL: server.URL, key: julesTestKey}
	err := client.call(context.Background(), http.MethodGet, "sessions/123", nil, nil)
	if err == nil || strings.Contains(err.Error(), julesTestKey) || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("err = %v", err)
	}
}

func TestJulesCommandNeverCreatesASession(t *testing.T) {
	fake := &fakeJules{}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	for _, prompt := range []string{"wait", "APPROVE", " approve\n"} {
		if !h.driver.IsControlPrompt(prompt) {
			t.Errorf("%q is not a command", prompt)
		}
		if _, err := h.driver.Run(context.Background(), RunOptions{Prompt: prompt}); err == nil || !strings.Contains(err.Error(), "no Jules session") {
			t.Errorf("Run(%q) err = %v", prompt, err)
		}
	}
	if h.driver.IsControlPrompt("please wait for the tests") {
		t.Error("a message counts as a command")
	}
}

func TestJulesResumeApprove(t *testing.T) {
	waiting := julesSession{State: "AWAITING_PLAN_APPROVAL", UpdateTime: "t1"}
	done := julesDone("https://github.com/o/harnez/pull/9")
	done.UpdateTime = "t2"
	fake := &fakeJules{states: []julesSession{waiting, done}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	if _, err := h.driver.Resume(context.Background(), "123", "approve", Model{}); err != nil {
		t.Fatal(err)
	}
	if fake.approvals != 1 || len(fake.messages) != 0 {
		t.Errorf("approvals = %d messages = %v", fake.approvals, fake.messages)
	}
	fake = &fakeJules{states: []julesSession{{State: "IN_PROGRESS"}}}
	h = newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	if _, err := h.driver.Resume(context.Background(), "123", "approve", Model{}); err == nil || !strings.Contains(err.Error(), "not waiting for plan approval") {
		t.Fatalf("err = %v", err)
	}
	if fake.approvals != 0 {
		t.Errorf("approved a session not waiting: %d", fake.approvals)
	}
}

func TestJulesActivitiesFollowPages(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			fmt.Fprint(w, `{"activities":[{"createTime":"2"}],"nextPageToken":"a b"}`)
			return
		}
		if r.URL.Query().Get("pageToken") != "a b" {
			t.Errorf("pageToken = %q", r.URL.Query().Get("pageToken"))
		}
		_, _ = io.WriteString(w, `{"activities":[{"createTime":"1"}]}`)
	}))
	defer server.Close()
	all, err := JulesDriver{}.activities(context.Background(), julesClient{http: server.Client(), baseURL: server.URL}, "sessions/1")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].CreateTime != "1" {
		t.Fatalf("activities = %+v", all)
	}
}

func TestParseGitHubRemote(t *testing.T) {
	tests := []struct {
		url, owner, repo string
		ok               bool
	}{
		{"git@github.com:ubunatic/harnez.git", "ubunatic", "harnez", true},
		{"https://github.com/ubunatic/harnez", "ubunatic", "harnez", true},
		{"https://token@github.com/ubunatic/harnez.git", "ubunatic", "harnez", true},
		{"ssh://git@github.com/ubunatic/harnez.git\n", "ubunatic", "harnez", true},
		{"ssh://git@codeberg.org/ubunatic/harnez.git", "", "", false},
		{"https://github.com.evil.test/ubunatic/harnez", "", "", false},
	}
	for _, tc := range tests {
		owner, repo, ok := parseGitHubRemote(tc.url)
		if owner != tc.owner || repo != tc.repo || ok != tc.ok {
			t.Errorf("parseGitHubRemote(%q) = %q %q %v", tc.url, owner, repo, ok)
		}
	}
}

func TestJulesModelResolvesToDriver(t *testing.T) {
	model, err := ResolveModel("jules")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := DriverFor(model, t.TempDir()).(JulesDriver); !ok || model.SupportsEffort() {
		t.Fatalf("model = %+v driver = %T", model, DriverFor(model, ""))
	}
	spec, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Jules.validate(); err != nil {
		t.Fatal(err)
	}
}
