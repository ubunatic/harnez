package forge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPRClient_ForgejoLifecycle(t *testing.T) {
	var gotAuth, gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodPut) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/app/pulls":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"number":   12,
				"title":    "Add pr verbs",
				"state":    "open",
				"html_url": "https://codeberg.org/acme/app/pulls/12",
				"user":     map[string]any{"login": "alice"},
				"head":     map[string]any{"ref": "feat/pr"},
				"base":     map[string]any{"ref": "main"},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/app/pulls/12":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":    12,
				"title":     "Add pr verbs",
				"body":      "Details here",
				"state":     "open",
				"mergeable": true,
				"html_url":  "https://codeberg.org/acme/app/pulls/12",
				"user":      map[string]any{"login": "alice"},
				"head":      map[string]any{"ref": "feat/pr"},
				"base":      map[string]any{"ref": "main"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/app/issues/12/comments":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": 1, "body": "LGTM", "created_at": "2026-11-23T10:00:00Z",
				"user": map[string]any{"login": "bob"},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/app/pulls/12/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"filename": "cmd/harnez/pr.go", "status": "modified", "additions": 10, "deletions": 2,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/app/pulls":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 13, "title": "New PR", "state": "open", "html_url": "https://codeberg.org/acme/app/pulls/13",
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/acme/app/pulls/12":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 12, "title": "Updated", "state": "closed",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/app/issues/12/comments":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"html_url": "https://codeberg.org/acme/app/pulls/12#issuecomment-9"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/app/pulls/12/merge":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL, TokenEnv: []string{"CODEBERG_TOKEN"}}
	cli := PRClient{
		Client:    srv.Client(),
		ReadToken: func(Forge) string { return "secret-read" },
		WriteToken: func(Forge) (string, string) {
			return "secret-write", "env CODEBERG_TOKEN"
		},
	}

	pulls, err := cli.ListPulls(fg, "acme", "app", "open", 15)
	if err != nil || len(pulls) != 1 || pulls[0].Number != 12 || pulls[0].Author != "alice" {
		t.Fatalf("ListPulls = %+v, err=%v", pulls, err)
	}
	if !strings.Contains(gotPath, "limit=15") {
		t.Errorf("Forgejo ListPulls should use limit param, got %s", gotPath)
	}

	pr, err := cli.GetPull(fg, "acme", "app", 12)
	if err != nil || pr.Title != "Add pr verbs" || pr.Mergeable == nil || !*pr.Mergeable {
		t.Fatalf("GetPull = %+v, err=%v", pr, err)
	}

	comments, err := cli.ListPullComments(fg, "acme", "app", 12)
	if err != nil || len(comments) != 1 || comments[0].Author != "bob" {
		t.Fatalf("ListPullComments = %+v, err=%v", comments, err)
	}

	files, err := cli.ListPullFiles(fg, "acme", "app", 12)
	if err != nil || len(files) != 1 || files[0].Filename != "cmd/harnez/pr.go" {
		t.Fatalf("ListPullFiles = %+v, err=%v", files, err)
	}

	detail := FormatPullDetail(pr, comments, files)
	if !strings.Contains(detail, "#12 Add pr verbs") || !strings.Contains(detail, "LGTM") || !strings.Contains(detail, "cmd/harnez/pr.go") {
		t.Errorf("unexpected FormatPullDetail:\n%s", detail)
	}

	created, err := cli.CreatePull(fg, "acme", "app", CreatePullInput{Title: "New PR", Head: "feat", Base: "main"})
	if err != nil || created.Number != 13 {
		t.Fatalf("CreatePull = %+v, err=%v", created, err)
	}
	if gotAuth != "token secret-write" {
		t.Errorf("Forgejo auth = %q, want token header", gotAuth)
	}

	closed := "closed"
	edited, err := cli.EditPull(fg, "acme", "app", 12, EditPullInput{State: &closed})
	if err != nil || edited.State != "closed" {
		t.Fatalf("EditPull = %+v, err=%v", edited, err)
	}

	url, err := cli.CommentPull(fg, "acme", "app", 12, "Ship it")
	if err != nil || !strings.Contains(url, "issuecomment-9") {
		t.Fatalf("CommentPull = %q, err=%v", url, err)
	}

	if err := cli.MergePull(fg, "acme", "app", 12, MergePullInput{Method: "squash", DeleteBranch: true}); err != nil {
		t.Fatalf("MergePull err = %v", err)
	}
	if gotMethod != http.MethodPost || gotBody["Do"] != "squash" || gotBody["delete_branch_after_merge"] != true {
		t.Errorf("Forgejo MergePull sent method=%s body=%+v", gotMethod, gotBody)
	}
}

func TestPRClient_GitHubMergeAndAuth(t *testing.T) {
	var gotAuth, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fg := Forge{Host: "github.com", Kind: "github", APIBase: srv.URL, TokenEnv: []string{"GITHUB_TOKEN"}}
	cli := PRClient{
		Client: srv.Client(),
		WriteToken: func(Forge) (string, string) {
			return "gh-secret", "env GITHUB_TOKEN"
		},
	}
	if err := cli.MergePull(fg, "acme", "app", 5, MergePullInput{Method: "rebase"}); err != nil {
		t.Fatalf("GitHub MergePull err = %v", err)
	}
	if gotMethod != http.MethodPut || gotAuth != "Bearer gh-secret" || gotBody["merge_method"] != "rebase" {
		t.Errorf("GitHub MergePull got method=%s auth=%q body=%+v", gotMethod, gotAuth, gotBody)
	}
}

func TestPRClient_WriteErrorsNeverLeakToken(t *testing.T) {
	const secret = "super-secret-pr-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "bad token " + secret})
	}))
	defer srv.Close()

	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL, TokenEnv: []string{"CODEBERG_TOKEN"}}
	cli := PRClient{
		Client: srv.Client(),
		WriteToken: func(Forge) (string, string) {
			return secret, "env CODEBERG_TOKEN"
		},
	}
	_, err := cli.CommentPull(fg, "acme", "app", 1, "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestCheckHostWrite(t *testing.T) {
	if err := CheckHostWrite(func(string) string { return "" }); err != nil {
		t.Errorf("host session should be allowed: %v", err)
	}
	if err := CheckHostWrite(func(k string) string {
		if k == "HARNEZ_AGENT_ROLE" {
			return "dev"
		}
		return ""
	}); err == nil {
		t.Error("worker session should be refused")
	}
}
